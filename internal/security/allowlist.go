package security

import (
	"net"
	"net/http"
	"strings"
)

// IPAllowed returns true if allowlist is empty or remote IP is in the list.
func IPAllowed(r *http.Request, allowlist []string) bool {
	if len(allowlist) == 0 {
		return true
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, entry := range allowlist {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if strings.Contains(entry, "/") {
			_, cidr, err := net.ParseCIDR(entry)
			if err == nil && cidr.Contains(ip) {
				return true
			}
			continue
		}
		if hostIP := net.ParseIP(entry); hostIP != nil && hostIP.Equal(ip) {
			return true
		}
	}
	return false
}
