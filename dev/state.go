package dev

import (
	"sync"
	"time"
)

// BuildPhase 表示当前 build 状态机所处阶段。
type BuildPhase string

const (
	BuildPhaseIdle     BuildPhase = "idle"
	BuildPhaseBuilding BuildPhase = "building"
	BuildPhaseOK       BuildPhase = "ok"
	BuildPhaseFail     BuildPhase = "failed"
)

// BuildStatus 描述最近一次 build 的结果。
type BuildStatus struct {
	Phase        BuildPhase `json:"phase"`
	StartedAt    time.Time  `json:"started_at,omitempty"`
	FinishedAt   time.Time  `json:"finished_at,omitempty"`
	DurationMS   int64      `json:"duration_ms,omitempty"`
	BinaryPath   string     `json:"binary_path,omitempty"`
	ChangedFiles []string   `json:"changed_files,omitempty"` // 触发本次 build 的文件集合 (去重排序)
}

// ChildExit 描述子 app 进程退出。
type ChildExit struct {
	PID      int       `json:"pid"`
	ExitCode int       `json:"exit_code"`
	ExitedAt time.Time `json:"exited_at"`
	Reason   string    `json:"reason,omitempty"` // "normal" / "signaled" / "watcher-restart"
}

// ReloadStatus 描述子进程(被重启的 app 实例)状态。
type ReloadStatus struct {
	PID          int       `json:"pid,omitempty"`
	StartedAt    time.Time `json:"started_at,omitempty"`
	ReloadCount  int       `json:"reload_count"`
	LastReloadAt time.Time `json:"last_reload_at,omitempty"`
}

// DevState 是 cmd/igo dev watcher 维护的运行时状态,通过 /_ai/dev 暴露。
//
// 所有访问通过 Store 串行化;不直接暴露字段。
type DevState struct {
	Mode          string            `json:"mode"`
	Build         BuildStatus       `json:"build"`
	Reload        ReloadStatus      `json:"reload"`
	CompileErrors []StructuredError `json:"compile_errors"`
	WatchedRoots  []string          `json:"watched_roots"`
	AppPort       int               `json:"app_port,omitempty"`
	WatcherPort   int               `json:"watcher_port"`
	StartedAt     time.Time         `json:"started_at"`
}

// Store 包装 DevState 提供并发安全访问 + 订阅事件的能力。
type Store struct {
	mu          sync.RWMutex
	state       DevState
	subscribers map[chan Event]struct{}
}

// NewStore 创建初始状态的 Store。
func NewStore(watcherPort int, watchedRoots []string) *Store {
	return &Store{
		state: DevState{
			Mode:         "dev",
			Build:        BuildStatus{Phase: BuildPhaseIdle},
			WatcherPort:  watcherPort,
			WatchedRoots: watchedRoots,
			StartedAt:    time.Now(),
		},
		subscribers: make(map[chan Event]struct{}),
	}
}

// Snapshot 返回当前状态的深拷贝。slice 字段独立,调用方修改不影响 store。
func (s *Store) Snapshot() DevState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := s.state
	out.CompileErrors = append([]StructuredError(nil), s.state.CompileErrors...)
	out.WatchedRoots = append([]string(nil), s.state.WatchedRoots...)
	out.Build.ChangedFiles = append([]string(nil), s.state.Build.ChangedFiles...)
	return out
}

// MarkBuildStart 进入 building 阶段并广播。
// changedFiles 已去重排序; 空 slice 表示初始 build 或未知触发源。
func (s *Store) MarkBuildStart(changedFiles []string) {
	files := append([]string(nil), changedFiles...)
	s.mu.Lock()
	s.state.Build = BuildStatus{
		Phase:        BuildPhaseBuilding,
		StartedAt:    time.Now(),
		ChangedFiles: files,
	}
	s.state.CompileErrors = nil
	s.mu.Unlock()
	s.broadcast(Event{Type: EventBuildStart, Payload: map[string]any{"files": files}})
}

// MarkBuildOK 进入 ok 阶段。
func (s *Store) MarkBuildOK(binaryPath string) {
	s.mu.Lock()
	now := time.Now()
	s.state.Build.Phase = BuildPhaseOK
	s.state.Build.FinishedAt = now
	s.state.Build.DurationMS = now.Sub(s.state.Build.StartedAt).Milliseconds()
	s.state.Build.BinaryPath = binaryPath
	s.state.CompileErrors = nil
	dur := s.state.Build.DurationMS
	s.mu.Unlock()
	s.broadcast(Event{Type: EventBuildOK, Payload: map[string]any{"duration_ms": dur, "binary_path": binaryPath}})
}

// MarkBuildFail 进入 failed 阶段并附结构化错误列表。
func (s *Store) MarkBuildFail(errs []StructuredError) {
	s.mu.Lock()
	now := time.Now()
	s.state.Build.Phase = BuildPhaseFail
	s.state.Build.FinishedAt = now
	s.state.Build.DurationMS = now.Sub(s.state.Build.StartedAt).Milliseconds()
	s.state.Build.BinaryPath = ""
	s.state.CompileErrors = errs
	s.mu.Unlock()
	s.broadcast(Event{Type: EventBuildFail, Payload: map[string]any{"errors": errs}})
}

// UpdateAppPort 设置当前子 app 实际监听的端口。
// 子 app 启动后通过 /_internal/announce 回调到 watcher,这里负责落表。
// 不广播事件 (避免和 reload:done 重复); 调用方在 reload 后才会调用。
func (s *Store) UpdateAppPort(port int) {
	s.mu.Lock()
	if port > 0 {
		s.state.AppPort = port
	}
	s.mu.Unlock()
}

// MarkChildExit 在子 app 进程被 wait reap 后调用,广播 child:exited 事件。
// reason 通常是 "normal" / "signaled" / "watcher-restart"。
func (s *Store) MarkChildExit(exit ChildExit) {
	if exit.ExitedAt.IsZero() {
		exit.ExitedAt = time.Now()
	}
	s.broadcast(Event{
		Type: EventChildExited,
		Payload: map[string]any{
			"pid":       exit.PID,
			"exit_code": exit.ExitCode,
			"reason":    exit.Reason,
		},
	})
}

// MarkReload 在子进程重启完成后调用。
func (s *Store) MarkReload(pid, port int) {
	s.mu.Lock()
	now := time.Now()
	s.state.Reload.PID = pid
	s.state.Reload.LastReloadAt = now
	s.state.Reload.ReloadCount++
	if s.state.Reload.StartedAt.IsZero() {
		s.state.Reload.StartedAt = now
	}
	if port > 0 {
		s.state.AppPort = port
	}
	count := s.state.Reload.ReloadCount
	s.mu.Unlock()
	s.broadcast(Event{Type: EventReloadDone, Payload: map[string]any{"pid": pid, "port": port, "reload_count": count}})
}

// Subscribe 注册事件订阅,返回 channel 与取消函数。
// channel 容量 16,慢消费者会丢失事件(SSE 客户端断开时调用 cancel 即可)。
func (s *Store) Subscribe() (chan Event, func()) {
	ch := make(chan Event, 16)
	s.mu.Lock()
	s.subscribers[ch] = struct{}{}
	s.mu.Unlock()
	cancel := func() {
		s.mu.Lock()
		delete(s.subscribers, ch)
		s.mu.Unlock()
		close(ch)
	}
	return ch, cancel
}

func (s *Store) broadcast(ev Event) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for ch := range s.subscribers {
		select {
		case ch <- ev:
		default:
			// drop on backpressure
		}
	}
}
