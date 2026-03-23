package hub

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/chrisvoo/axon/internal/events"
	"github.com/chrisvoo/axon/internal/security"
	"github.com/chrisvoo/axon/internal/tools"
	"github.com/gorilla/websocket"
)

// Hub fans out bus events to WebSocket clients and pushes periodic system stats.
type Hub struct {
	bus     *events.Bus
	version string
	mu      sync.Mutex
	clients map[*websocket.Conn]struct{}
	upgrader websocket.Upgrader
}

// NewHub creates a hub that broadcasts events from the given bus.
func NewHub(bus *events.Bus, version string) *Hub {
	return &Hub{
		bus:     bus,
		version: version,
		clients: make(map[*websocket.Conn]struct{}),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 4096,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

// Run consumes events from the bus until ctx is cancelled.
func (h *Hub) Run(ctx context.Context) {
	ch := h.bus.Subscribe()
	for {
		select {
		case <-ctx.Done():
			return
		case e := <-ch:
			h.broadcastEvent(e)
		}
	}
}

// RunSystemStats pushes system_stats every interval until ctx is cancelled.
func (h *Hub) RunSystemStats(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			b, err := tools.SystemInfo()
			if err != nil {
				continue
			}
			var data map[string]any
			if err := json.Unmarshal(b, &data); err != nil {
				continue
			}
			h.broadcastEvent(events.Event{Type: "system_stats", Data: data})
		}
	}
}

func (h *Hub) register(c *websocket.Conn) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) unregister(c *websocket.Conn) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
}

func (h *Hub) broadcastEvent(e events.Event) {
	payload, err := json.Marshal(e)
	if err != nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		_ = c.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if err := c.WriteMessage(websocket.TextMessage, payload); err != nil {
			_ = c.Close()
			delete(h.clients, c)
		}
	}
}

// ServeWS upgrades the connection and streams events to the browser.
// expectedKey is the configured API key; clients must pass ?key=<api_key>.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request, expectedKey string) {
	key := r.URL.Query().Get("key")
	if key == "" || !security.ConstantTimeEqual(key, expectedKey) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	h.register(conn)
	defer func() {
		h.unregister(conn)
		_ = conn.Close()
	}()

	connected := events.Event{
		Type: "connected",
		Data: map[string]any{
			"version": h.version,
			"time":    time.Now().UTC().Format(time.RFC3339Nano),
		},
	}
	if b, err := json.Marshal(connected); err == nil {
		_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		_ = conn.WriteMessage(websocket.TextMessage, b)
	}

	for {
		_ = conn.SetReadDeadline(time.Now().Add(120 * time.Second))
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}
