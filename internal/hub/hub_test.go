package hub

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chrisvoo/axon/internal/events"
)

func TestServeWS_RejectsAKeyThatFailsTheCheck(t *testing.T) {
	h := NewHub(events.NewBus(), "test")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/ws?key=wrong", nil)

	h.ServeWS(rec, req, func(key string) bool { return key == "right" })

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got HTTP %d, want 401", rec.Code)
	}
}

func TestServeWS_RejectsAMissingKey(t *testing.T) {
	h := NewHub(events.NewBus(), "test")
	rec := httptest.NewRecorder()

	h.ServeWS(rec, httptest.NewRequest("GET", "/ws", nil), func(string) bool { return true })

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got HTTP %d, want 401", rec.Code)
	}
}

func TestServeWS_AcceptsAKeyThatPassesTheCheck(t *testing.T) {
	h := NewHub(events.NewBus(), "test")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/ws?key=right", nil)

	h.ServeWS(rec, req, func(key string) bool { return key == "right" })

	if rec.Code == http.StatusUnauthorized {
		t.Fatal("a key that passes the check must not get 401")
	}
}
