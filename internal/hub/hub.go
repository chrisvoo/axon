package hub

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/chrisvoo/axon/internal/events"
	"github.com/chrisvoo/axon/internal/tools"
	"github.com/gorilla/websocket"
)

const (
	// Subprotocol is the one that the server selects. A client offers it next to
	// its key protocol.
	Subprotocol = "axon.v1"
	// KeyProtocolPrefix starts the offered protocol that carries the API key:
	// "axon.key.<key>". The key goes in the Sec-WebSocket-Protocol request header,
	// so it never shows in a URL, a proxy access log or the browser history. The
	// server never echoes it back.
	KeyProtocolPrefix = "axon.key."
	// CloseUnauthorized is the close code (private use range) for a missing or
	// wrong key. A browser cannot read the HTTP status of a failed handshake, so
	// the server completes it and then closes with this code.
	CloseUnauthorized = 4401
)

// Hub fans out bus events to WebSocket clients and pushes periodic system stats.
type Hub struct {
	bus      *events.Bus
	version  string
	mu       sync.Mutex
	clients  map[*websocket.Conn]struct{}
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
			Subprotocols:    []string{Subprotocol},
			// No CheckOrigin: the default accepts a request with no Origin header
			// (not a browser) and a browser request whose Origin host equals Host.
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

// keyFromProtocols returns the key from the offered "axon.key.<key>" protocol.
func keyFromProtocols(protocols []string) string {
	for _, p := range protocols {
		if key, ok := strings.CutPrefix(p, KeyProtocolPrefix); ok {
			return key
		}
	}
	return ""
}

// ServeWS upgrades the connection and streams events to the browser. The client
// offers the protocols "axon.v1" and "axon.key.<API key>". validKey decides
// whether the key is right. A missing or wrong key closes the connection with
// CloseUnauthorized before the server sends any event.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request, validKey func(key string) bool) {
	key := keyFromProtocols(websocket.Subprotocols(r))
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	if key == "" || !validKey(key) {
		_ = conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(CloseUnauthorized, "unauthorized"),
			time.Now().Add(time.Second),
		)
		_ = conn.Close()
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
