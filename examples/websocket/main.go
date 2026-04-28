// Package main 演示在 igo 中处理 WebSocket。
//
// AI 学习要点：
//   - igo 不内置 WebSocket，但 c.Writer/c.Request 完全暴露，可直接对接 gorilla/websocket
//   - 用 sync.RWMutex 保护并发的 client map（每个连接是独立 goroutine）
//   - 必须设置读超时 + Pong 处理器，否则连接会僵死
//   - 写入 ws.Conn 必须串行（不能并发 Write）→ 用单独的 goroutine 从 channel 读取并发送
//   - Origin 校验：默认 gorilla 的 CheckOrigin 会拒绝跨域，演示需放开（生产应严格限制）
//
// 测试：
//
//	go run ./examples/websocket
//	# 浏览器开多个 tab 访问 http://localhost:8080/，发送消息会广播到所有连接
package main

import (
	"log"
	"net/http"
	"sync"
	"time"

	igo "github.com/leebo/igo"
	"github.com/leebo/igo/core"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second    // 写超时
	pongWait       = 60 * time.Second    // 等待客户端 pong 的超时
	pingPeriod     = (pongWait * 9) / 10 // 服务端 ping 间隔，必须小于 pongWait
	maxMessageSize = 4096
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// 演示用：允许任意 Origin。生产应当只允许已知域名
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Hub 维护所有活跃 WebSocket 连接，负责广播消息
type Hub struct {
	mu      sync.RWMutex
	clients map[*Client]bool
}

func newHub() *Hub {
	return &Hub{clients: make(map[*Client]bool)}
}

func (h *Hub) add(c *Client) {
	h.mu.Lock()
	h.clients[c] = true
	h.mu.Unlock()
}

func (h *Hub) remove(c *Client) {
	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
	}
	h.mu.Unlock()
}

func (h *Hub) broadcast(msg []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		// 非阻塞发送：客户端处理过慢就丢消息（避免拖累整个 hub）
		select {
		case c.send <- msg:
		default:
			// 满载则在外层关掉这个连接
			go h.remove(c)
		}
	}
}

// Client 包装一个 ws.Conn 和它的发送队列
type Client struct {
	conn *websocket.Conn
	send chan []byte
}

// readPump 从客户端读取消息，转发给 hub 广播
// 每个 Client 一个 readPump goroutine
func (c *Client) readPump(hub *Hub) {
	defer func() {
		hub.remove(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("ws read: %v", err)
			}
			return
		}
		hub.broadcast(msg)
	}
}

// writePump 把 hub 推过来的消息写到客户端连接
// 同时定期发送 Ping 保持连接活跃
// 每个 Client 一个 writePump goroutine（保证写串行）
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// hub 关闭了这个 channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func wsHandler(hub *Hub) core.HandlerFunc {
	return func(c *core.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			// Upgrade 内部已经写了响应，这里只 log
			log.Printf("ws upgrade: %v", err)
			return
		}
		client := &Client{conn: conn, send: make(chan []byte, 32)}
		hub.add(client)

		// 启动两个 goroutine：一读一写，避免在同一连接上并发写
		go client.writePump()
		go client.readPump(hub)
	}
}

// 简单的 HTML 演示页，浏览器打开就能聊天
const indexHTML = `<!doctype html>
<html><body>
<h2>igo websocket demo</h2>
<div id="log" style="border:1px solid #ccc;height:200px;overflow:auto;padding:8px;"></div>
<input id="msg" placeholder="say something" style="width:60%"/>
<button onclick="send()">Send</button>
<script>
const log = document.getElementById("log");
const msg = document.getElementById("msg");
const ws = new WebSocket("ws://" + location.host + "/ws");
ws.onmessage = e => { log.innerHTML += e.data + "<br>"; log.scrollTop = log.scrollHeight; };
ws.onclose = () => log.innerHTML += "<i>disconnected</i><br>";
function send() { ws.send(msg.value); msg.value = ""; }
msg.addEventListener("keydown", e => { if (e.key === "Enter") send(); });
</script>
</body></html>`

// statsHandler 报告当前活跃 WebSocket 连接数
func statsHandler(hub *Hub) core.HandlerFunc {
	return func(c *core.Context) {
		hub.mu.RLock()
		n := len(hub.clients)
		hub.mu.RUnlock()
		c.Success(core.H{"clients": n})
	}
}

// indexHandler 返回简单的演示 HTML
func indexHandler(c *core.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Writer.Write([]byte(indexHTML))
}

// setupApp 把 hub 注入构造好的 App，便于测试
func setupApp(hub *Hub) *igo.App {
	app := igo.Simple()
	app.Get("/", indexHandler)
	app.Get("/ws", wsHandler(hub))
	app.Get("/stats", statsHandler(hub))
	return app
}

func main() {
	hub := newHub()
	app := setupApp(hub)
	app.PrintRoutes()
	app.Run(":8080")
}
