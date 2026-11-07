package security

import (
	"net/http"
	"testing"
)

func req(remote, cf string) *http.Request {
	r, _ := http.NewRequest("GET", "/", nil)
	r.RemoteAddr = remote
	if cf != "" {
		r.Header.Set("CF-Connecting-IP", cf)
	}
	return r
}

func TestClientIP(t *testing.T) {
	cases := []struct {
		name   string
		r      *http.Request
		tunnel bool
		want   string
	}{
		{"no tunnel: the header is ignored", req("127.0.0.1:5000", "203.0.113.5"), false, "127.0.0.1"},
		{"tunnel, loopback peer: the header counts", req("127.0.0.1:5000", "203.0.113.5"), true, "203.0.113.5"},
		{"tunnel, IPv6 loopback peer", req("[::1]:5000", "2001:db8::1"), true, "2001:db8::1"},
		{"tunnel, outside peer: a client sets the header itself", req("198.51.100.9:5000", "203.0.113.5"), true, "198.51.100.9"},
		{"tunnel, no header: the peer", req("127.0.0.1:5000", ""), true, "127.0.0.1"},
		{"tunnel, bad header: the peer", req("127.0.0.1:5000", "not-an-ip"), true, "127.0.0.1"},
	}
	for _, tc := range cases {
		if got := ClientIP(tc.r, tc.tunnel); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}
