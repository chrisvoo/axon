//go:build !windows

package rootcheck

import (
	"fmt"
	"os"
)

// EnsureNotRoot refuses to run as root on Unix.
func EnsureNotRoot() error {
	if os.Geteuid() == 0 {
		return fmt.Errorf("axon refuses to run as root; run as a normal user")
	}
	return nil
}
