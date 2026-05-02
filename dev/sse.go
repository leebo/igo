package dev

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// sseKeepAliveInterval is how often we emit a comment line on an idle SSE
// connection. Many reverse proxies (nginx default 60s, cloudflare 100s) drop
// connections without traffic; 15s leaves a comfortable margin.
const sseKeepAliveInterval = 15 * time.Second

// EventType 列举 /_ai/dev/events SSE 流推送的事件类型。
type EventType string

const (
	EventBuildStart  EventType = "build:start"
	EventBuildOK     EventType = "build:ok"
	EventBuildFail   EventType = "build:fail"
	EventReloadDone  EventType = "reload:done"
	EventChildExited EventType = "child:exited"
	// EventSnapshot is pushed exactly once when a new SSE client connects
	// so it gets the current state without waiting for the next build.
	EventSnapshot EventType = "snapshot"
)

// Event 是单条 SSE 推送。
//
// Type 进入 SSE 的 `event:` 行,Payload 序列化后进入 `data:` 行
// (writeEvent 单独处理),所以两个字段都不参与整体 JSON 序列化,
// 因此没有 json struct tag。
type Event struct {
	Type    EventType
	Payload map[string]any
}

// ServeSSE 把 HTTP 请求挂上 store 的事件订阅,持续推送直到客户端断开。
func ServeSSE(store *Store, w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	// 立即推一条 snapshot,避免新订阅者要等到下次 build 才有任何输出
	snapshot := store.Snapshot()
	writeEvent(w, flusher, Event{
		Type:    EventSnapshot,
		Payload: map[string]any{"state": snapshot},
	})

	ch, cancel := store.Subscribe()
	defer cancel()

	keepAlive := time.NewTicker(sseKeepAliveInterval)
	defer keepAlive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			writeEvent(w, flusher, ev)
		case <-keepAlive.C:
			// SSE comment line: ignored by clients but keeps proxies happy.
			_, _ = fmt.Fprint(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}

func writeEvent(w http.ResponseWriter, flusher http.Flusher, ev Event) {
	data, err := json.Marshal(ev.Payload)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "event: %s\n", ev.Type)
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}
