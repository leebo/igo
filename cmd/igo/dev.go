package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/leebo/igo/dev"
)

const (
	defaultWatcherPort = 18999
	defaultDebounceMS  = 300
	defaultBuildTimeoutSec = 60
)

// runDev 是 `igo dev [path]` 子命令入口。
//
// 行为:
//  1. 在当前(或指定)目录递归监听 *.go 变化(排除 vendor/、.git/、build 产物)
//  2. 防抖 300ms 后触发 `go build -o <tmpdir>/bin .`
//  3. build 失败:解析 stderr,写入 DevState,SSE 推送 build:fail
//  4. build 成功:SIGTERM 旧子进程,启动新子进程(注入 IGO_ENV=dev),SSE 推送
//  5. watcher 自身在 :18999 暴露 /_ai/dev、/_ai/dev/events、/healthz
//  6. SIGINT/SIGTERM 优雅退出:停 watcher → 停子进程 → 清理临时目录
func runDev(args []string) int {
	fs := flag.NewFlagSet("dev", flag.ContinueOnError)
	port := fs.Int("watcher-port", defaultWatcherPort, "watcher introspection port (/_ai/dev)")
	host := fs.String("watcher-host", "127.0.0.1", "host the child app should reach the watcher on (used for IGO_DEV_WATCHER env)")
	appAddr := fs.String("app-addr", ":0", "address passed to the child app via APP_ADDR env (`:port` or `host:port`)")
	dir := fs.String("dir", ".", "directory to watch (default: cwd)")
	debounceMS := fs.Int("debounce-ms", defaultDebounceMS, "debounce window for batched file saves, in milliseconds")
	buildTimeoutSec := fs.Int("build-timeout", defaultBuildTimeoutSec, "max seconds for a single `go build` before it is killed")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	root, err := filepath.Abs(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[igo dev] resolve dir: %v\n", err)
		return 1
	}

	// FIX #1: bind watcher port synchronously up front so a port conflict
	// (typical: stale watcher from a previous session) fails the command
	// instead of producing a half-working state.
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		fmt.Fprintf(os.Stderr, "[igo dev] bind watcher port :%d: %v\n", *port, err)
		fmt.Fprintln(os.Stderr, "[igo dev] hint: another `igo dev` process is likely still running")
		return 1
	}

	tmpDir, err := os.MkdirTemp("", "igo-dev-")
	if err != nil {
		_ = listener.Close()
		fmt.Fprintf(os.Stderr, "[igo dev] mktemp: %v\n", err)
		return 1
	}
	defer os.RemoveAll(tmpDir)
	binPath := filepath.Join(tmpDir, "bin")

	store := dev.NewStore(*port, []string{root})

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	go serveWatcherHTTP(ctx, store, listener)

	w := &watcher{
		root:         root,
		appAddr:      *appAddr,
		binPath:      binPath,
		watcherHost:  *host,
		store:        store,
		debounce:     time.Duration(*debounceMS) * time.Millisecond,
		buildTimeout: time.Duration(*buildTimeoutSec) * time.Second,
	}
	defer w.stopChild()

	w.runOnce(nil) // initial build + start child
	if err := w.watchLoop(ctx); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(os.Stderr, "[igo dev] watch loop: %v\n", err)
		return 1
	}
	return 0
}

// ---------- watcher ----------

type watcher struct {
	root         string
	appAddr      string
	binPath      string
	watcherHost  string        // host the child should put in IGO_DEV_WATCHER (FIX #7)
	debounce     time.Duration // debounce window for batched saves
	buildTimeout time.Duration // max duration for a single go build
	store        *dev.Store

	childMu sync.Mutex
	child   *exec.Cmd
	exitCh  chan struct{} // closed by the single Wait() goroutine when child exits
}

func (w *watcher) watchLoop(ctx context.Context) error {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("fsnotify: %w", err)
	}
	defer fsw.Close()

	if err := w.addAll(fsw); err != nil {
		return err
	}

	debounce := time.NewTimer(time.Hour)
	debounce.Stop()
	pending := map[string]struct{}{}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev, ok := <-fsw.Events:
			if !ok {
				return nil
			}
			if !shouldRebuild(ev) {
				continue
			}
			pending[ev.Name] = struct{}{}
			debounce.Reset(w.debounce)
			// new dirs created: pick them up automatically
			if ev.Op&fsnotify.Create != 0 {
				if info, err := os.Stat(ev.Name); err == nil && info.IsDir() && !skipDir(ev.Name) {
					_ = fsw.Add(ev.Name)
				}
			}
		case <-debounce.C:
			files := make([]string, 0, len(pending))
			for f := range pending {
				files = append(files, f)
			}
			sort.Strings(files)
			pending = map[string]struct{}{}
			w.runOnce(files)
		case err, ok := <-fsw.Errors:
			if !ok {
				return nil
			}
			fmt.Fprintf(os.Stderr, "[igo dev] watch error: %v\n", err)
		}
	}
}

func (w *watcher) addAll(fsw *fsnotify.Watcher) error {
	return filepath.Walk(w.root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			return nil
		}
		if skipDir(p) {
			return filepath.SkipDir
		}
		return fsw.Add(p)
	})
}

func skipDir(p string) bool {
	base := filepath.Base(p)
	return base == "vendor" || base == ".git" || base == "node_modules" || strings.HasPrefix(base, ".")
}

func shouldRebuild(ev fsnotify.Event) bool {
	if !strings.HasSuffix(ev.Name, ".go") {
		return false
	}
	if strings.HasSuffix(ev.Name, "_test.go") {
		return false
	}
	return ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename|fsnotify.Remove) != 0
}

// ---------- build + child ----------

func (w *watcher) runOnce(changedFiles []string) {
	w.store.MarkBuildStart(changedFiles)

	bctx, cancel := context.WithTimeout(context.Background(), w.buildTimeout)
	defer cancel()
	cmd := exec.CommandContext(bctx, "go", "build", "-o", w.binPath, ".")
	cmd.Dir = w.root
	cmd.Env = append(os.Environ(), "GOFLAGS=-buildvcs=false")
	out, err := cmd.CombinedOutput()
	if err != nil {
		errs := dev.ParseBuildErrors(string(out))
		if len(errs) == 0 {
			errs = []dev.StructuredError{{
				File:    "(unknown)",
				Type:    dev.ErrorTypeUnknown,
				Message: strings.TrimSpace(string(out)),
				Raw:     strings.TrimSpace(string(out)),
			}}
		}
		w.store.MarkBuildFail(errs)
		fmt.Fprintf(os.Stderr, "[igo dev] build failed (%d errors)\n", len(errs))
		return
	}
	w.store.MarkBuildOK(w.binPath)
	fmt.Println("[igo dev] build ok")
	w.restartChild()
}

func (w *watcher) restartChild() {
	w.stopChild()
	cmd := exec.Command(w.binPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(),
		"IGO_ENV=dev",
		fmt.Sprintf("IGO_DEV_WATCHER=http://%s:%d", w.watcherHost, w.store.Snapshot().WatcherPort),
		"APP_ADDR="+w.appAddr,
	)
	// FIX #2: put the child in its own process group so we can kill the
	// whole tree on shutdown. Without this, a SIGKILL on the watcher leaves
	// the child orphaned (reparented to init) and still bound to its port,
	// which makes the next `igo dev` fail with "address already in use".
	setNewProcessGroup(cmd)

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "[igo dev] start child: %v\n", err)
		return
	}
	exitCh := make(chan struct{})
	w.childMu.Lock()
	w.child = cmd
	w.exitCh = exitCh
	w.childMu.Unlock()
	w.store.MarkReload(cmd.Process.Pid, 0)
	// FIX #3: this is the *only* Wait() on the child; previous code had a
	// second Wait in stopChild which produced ECHILD races on whoever lost.
	// stopChild now waits on exitCh instead of calling Wait again.
	go func() {
		err := cmd.Wait()
		// FIX #20: surface child exit through SSE so AI clients see when the
		// app crashed mid-session instead of finding it gone via polling.
		w.childMu.Lock()
		stillTracked := (w.child == cmd) // false => stopChild() already cleared it
		w.childMu.Unlock()

		exit := dev.ChildExit{PID: cmd.Process.Pid}
		switch {
		case !stillTracked:
			exit.Reason = "watcher-restart"
		case err == nil:
			exit.Reason = "normal"
		default:
			if ee, ok := err.(*exec.ExitError); ok {
				exit.ExitCode = ee.ExitCode()
			} else {
				exit.ExitCode = -1
			}
			exit.Reason = "crash"
		}
		w.store.MarkChildExit(exit)
		close(exitCh)
	}()
}

func (w *watcher) stopChild() {
	w.childMu.Lock()
	c := w.child
	exitCh := w.exitCh
	w.child = nil
	w.exitCh = nil
	w.childMu.Unlock()
	if c == nil || c.Process == nil {
		return
	}
	// FIX #2: signal the whole process group, not just the lead pid, so
	// children spawned by the app (workers, sidecars) also get terminated.
	signalProcessGroup(c.Process.Pid, syscall.SIGTERM)
	select {
	case <-exitCh:
	case <-time.After(3 * time.Second):
		signalProcessGroup(c.Process.Pid, syscall.SIGKILL)
		<-exitCh // ensure the Wait goroutine has reaped the child
	}
}

// ---------- watcher HTTP ----------

// serveWatcherHTTP serves the introspection endpoints on a pre-bound listener
// (see FIX #1 in runDev). Using Serve(listener) instead of ListenAndServe
// guarantees that any port-conflict failure has already surfaced before
// runDev returns, so the rest of the watcher never runs in a half-broken
// state.
func serveWatcherHTTP(ctx context.Context, store *dev.Store, listener net.Listener) {
	mux := http.NewServeMux()
	mux.HandleFunc("/_ai/dev", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(store.Snapshot())
	})
	mux.HandleFunc("/_ai/dev/events", func(w http.ResponseWriter, r *http.Request) {
		dev.ServeSSE(store, w, r)
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	// FIX #8: child app phones home with the actual bound port after Listen.
	// Loopback-only (the watcher binds via the listener; reachability follows
	// from that). No body, just ?port=NNN; idempotent.
	mux.HandleFunc("/_internal/announce", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost && r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		p, err := strconv.Atoi(r.URL.Query().Get("port"))
		if err != nil || p <= 0 || p > 65535 {
			http.Error(w, "port query param required", http.StatusBadRequest)
			return
		}
		store.UpdateAppPort(p)
		w.WriteHeader(http.StatusNoContent)
	})
	srv := &http.Server{Handler: mux}
	go func() {
		<-ctx.Done()
		shutdownCtx, c := context.WithTimeout(context.Background(), 2*time.Second)
		defer c()
		_ = srv.Shutdown(shutdownCtx)
	}()
	fmt.Printf("[igo dev] watcher introspection: http://%s/_ai/dev\n", listener.Addr())
	if err := srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintf(os.Stderr, "[igo dev] watcher http: %v\n", err)
	}
}
