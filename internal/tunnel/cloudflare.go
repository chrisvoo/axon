// Package tunnel manages a Cloudflare quick tunnel alongside the Axon server.
package tunnel

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"sync"
)

// trycloudflareRe matches the public URL printed by cloudflared.
var trycloudflareRe = regexp.MustCompile(`https://[a-zA-Z0-9-]+\.trycloudflare\.com`)

// ErrNotFound is returned when cloudflared is not in PATH.
type ErrNotFound struct{}

func (ErrNotFound) Error() string {
	return "cloudflared not found in PATH\n" +
		"Install it from https://github.com/cloudflare/cloudflared/releases\n" +
		"or via your package manager (e.g. brew install cloudflared)"
}

// StartTrycloudflare starts a cloudflared quick tunnel pointing at localURL
// (e.g. "http://localhost:8443") and calls onURL once with the public
// https://….trycloudflare.com URL when cloudflared reports it.
// It blocks until ctx is cancelled or cloudflared exits.
func StartTrycloudflare(ctx context.Context, localURL string, onURL func(publicURL string)) error {
	if _, err := exec.LookPath("cloudflared"); err != nil {
		return ErrNotFound{}
	}

	cmd := exec.CommandContext(ctx, "cloudflared", "tunnel", "--url", localURL)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("cloudflared stderr pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("cloudflared stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start cloudflared: %w", err)
	}

	var once sync.Once
	scan := func(r io.Reader) {
		sc := bufio.NewScanner(r)
		for sc.Scan() {
			if m := trycloudflareRe.FindString(sc.Text()); m != "" {
				once.Do(func() { onURL(m) })
			}
		}
	}
	go scan(stderr)
	go scan(stdout)

	if err := cmd.Wait(); err != nil {
		// Context cancellation causes a non-zero exit — that's expected.
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("cloudflared exited: %w", err)
	}
	return nil
}
