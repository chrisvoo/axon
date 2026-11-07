package security

import (
	"net"
	"strings"
)

// IPAllowed returns true if allowlist is empty or the address is in the list.
// addr is a bare IP, as ClientIP returns it.
func IPAllowed(addr string, allowlist []string) bool {
	if len(allowlist) == 0 {
		return true
	}
	ip := net.ParseIP(addr)
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
