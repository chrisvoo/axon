//go:build windows

package rootcheck

// EnsureNotRoot is a no-op on Windows (elevated admin is common for dev).
func EnsureNotRoot() error {
	return nil
}
