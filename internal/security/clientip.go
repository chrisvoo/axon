package security

import (
	"net"
	"net/http"
	"strings"
)

// ClientIP returns the address of the caller as a bare IP, without the port.
//
// Behind a Cloudflare tunnel, every request reaches Axon from loopback, because
// cloudflared runs on the same machine. Cloudflare puts the real address in
// CF-Connecting-IP. The header counts only when the peer is loopback and a tunnel
// is active. In any other case the caller could set the header itself.
func ClientIP(r *http.Request, behindTunnel bool) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if behindTunnel {
		if peer := net.ParseIP(host); peer != nil && peer.IsLoopback() {
			if real := net.ParseIP(strings.TrimSpace(r.Header.Get("CF-Connecting-IP"))); real != nil {
				return real.String()
			}
		}
	}
	return host
}
