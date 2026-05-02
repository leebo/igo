package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newWSTestServer 启动一个 httptest.Server 并返回 ws:// 形式的 URL
func newWSTestServer(t *testing.T) (*httptest.Server, *Hub, string) {
	t.Helper()
	hub := newHub()
	app := setupApp(hub)

	srv := httptest.NewServer(app.Router)
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	return srv, hub, wsURL
}

// dialWS 连一个 ws 客户端
func dialWS(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	return conn
}

// readOnceWithTimeout 在指定 deadline 内读一帧
func readOnceWithTimeout(t *testing.T, conn *websocket.Conn, timeout time.Duration) ([]byte, error) {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(timeout))
	_, msg, err := conn.ReadMessage()
	return msg, err
}

// =============================================================================
// 自己发自己收：客户端是 hub 的成员，broadcast 时也会回到自己
// =============================================================================

func TestWebSocket_EchoToSender(t *testing.T) {
	_, _, wsURL := newWSTestServer(t)
	conn := dialWS(t, wsURL)

	want := []byte("hello self")
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, want))

	got, err := readOnceWithTimeout(t, conn, 2*time.Second)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

// =============================================================================
// 广播：A 发，B 应当收到
// =============================================================================

func TestWebSocket_Broadcast(t *testing.T) {
	_, hub, wsURL := newWSTestServer(t)

	connA := dialWS(t, wsURL)
	connB := dialWS(t, wsURL)

	// 等待 hub 把两个 client 都注册
	require.NoError(t, waitForClients(hub, 2, 1*time.Second))

	want := []byte("hi from A")
	require.NoError(t, connA.WriteMessage(websocket.TextMessage, want))

	got, err := readOnceWithTimeout(t, connB, 2*time.Second)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

// =============================================================================
// 客户端断开后 hub 应该清理
// =============================================================================

func TestWebSocket_ClientLeaveCleansHub(t *testing.T) {
	_, hub, wsURL := newWSTestServer(t)
	conn := dialWS(t, wsURL)

	require.NoError(t, waitForClients(hub, 1, 1*time.Second))

	conn.Close()

	assert.NoError(t, waitForClients(hub, 0, 2*time.Second))
}

// =============================================================================
// /stats 端点
// =============================================================================

func TestStatsEndpoint(t *testing.T) {
	srv, hub, wsURL := newWSTestServer(t)

	// 初始 0
	assert.Equal(t, 0, getStats(t, srv))

	// 连一个，stats=1
	dialWS(t, wsURL)
	require.NoError(t, waitForClients(hub, 1, 1*time.Second))
	assert.Equal(t, 1, getStats(t, srv))

	// 再连一个
	dialWS(t, wsURL)
	require.NoError(t, waitForClients(hub, 2, 1*time.Second))
	assert.Equal(t, 2, getStats(t, srv))
}

// =============================================================================
// 辅助
// =============================================================================

// waitForClients 自旋等待 hub 的客户端数达到目标值
// websocket 的注册/清理发生在 goroutine 里，需要轮询确认状态
func waitForClients(hub *Hub, want int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		hub.mu.RLock()
		got := len(hub.clients)
		hub.mu.RUnlock()
		if got == want {
			return nil
		}
		time.Sleep(20 * time.Millisecond)
	}
	hub.mu.RLock()
	got := len(hub.clients)
	hub.mu.RUnlock()
	return &timeoutErr{want: want, got: got}
}

type timeoutErr struct{ want, got int }

func (e *timeoutErr) Error() string {
	return "timed out waiting for clients: want " +
		itoa(e.want) + ", got " + itoa(e.got)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

// getStats 读 /stats 端点的 clients 字段
func getStats(t *testing.T, srv *httptest.Server) int {
	t.Helper()
	resp, err := http.Get(srv.URL + "/stats")
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var w struct {
		Data struct {
			Clients int `json:"clients"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &w), string(body))
	return w.Data.Clients
}
