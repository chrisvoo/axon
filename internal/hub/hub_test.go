package hub

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chrisvoo/axon/internal/events"
	"github.com/gorilla/websocket"
)

// serve starts a hub behind a test server. The key check accepts only "right".
func serve(t *testing.T) (*httptest.Server, *[]string) {
	t.Helper()
	h := NewHub(events.NewBus(), "test")
	var checked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeWS(w, r, func(key string) bool {
			checked = append(checked, key)
			return key == "right"
		})
	}))
	t.Cleanup(srv.Close)
	return srv, &checked
}

func dial(t *testing.T, srv *httptest.Server, path string, protocols ...string) (*websocket.Conn, *http.Response) {
	t.Helper()
	d := websocket.Dialer{Subprotocols: protocols, HandshakeTimeout: 2 * time.Second}
	conn, resp, err := d.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+path, nil)
	if err != nil {
		t.Fatalf("handshake failed: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn, resp
}

func expectUnauthorized(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	if !websocket.IsCloseError(err, CloseUnauthorized) {
		t.Fatalf("expected close code %d, got message %q and error %v", CloseUnauthorized, msg, err)
	}
}

func TestServeWS_AcceptsTheKeyInTheSubprotocol(t *testing.T) {
	srv, checked := serve(t)

	conn, resp := dial(t, srv, "/ws", Subprotocol, KeyProtocolPrefix+"right")

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	var ev events.Event
	if err := json.Unmarshal(msg, &ev); err != nil || ev.Type != "connected" {
		t.Fatalf("expected the connected event, got %q (%v)", msg, err)
	}
	if got := conn.Subprotocol(); got != Subprotocol {
		t.Fatalf("the server must select %q, got %q", Subprotocol, got)
	}
	if len(*checked) != 1 || (*checked)[0] != "right" {
		t.Fatalf("the key check must get the key without its prefix, got %v", *checked)
	}
	if strings.Contains(strings.Join(resp.Header.Values("Sec-WebSocket-Protocol"), ","), "right") {
		t.Fatal("the server must not echo the key")
	}
}

func TestServeWS_ClosesAWrongKey(t *testing.T) {
	srv, _ := serve(t)
	conn, _ := dial(t, srv, "/ws", Subprotocol, KeyProtocolPrefix+"wrong")
	expectUnauthorized(t, conn)
}

func TestServeWS_ClosesAMissingKey(t *testing.T) {
	srv, _ := serve(t)
	conn, _ := dial(t, srv, "/ws", Subprotocol)
	expectUnauthorized(t, conn)
}

// The key used to travel in the URL. The URL shows in proxy logs, so it must not work.
func TestServeWS_IgnoresAKeyInTheURL(t *testing.T) {
	srv, checked := serve(t)
	conn, _ := dial(t, srv, "/ws?key=right", Subprotocol)
	expectUnauthorized(t, conn)
	if len(*checked) != 0 {
		t.Fatalf("the key check must not run without a key protocol, got %v", *checked)
	}
}

func TestKeyFromProtocols(t *testing.T) {
	cases := []struct {
		name      string
		protocols []string
		want      string
	}{
		{"both protocols", []string{"axon.v1", "axon.key.abc"}, "abc"},
		{"key first", []string{"axon.key.abc", "axon.v1"}, "abc"},
		{"no key", []string{"axon.v1"}, ""},
		{"none", nil, ""},
		{"empty key", []string{"axon.key."}, ""},
	}
	for _, tc := range cases {
		if got := keyFromProtocols(tc.protocols); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}
