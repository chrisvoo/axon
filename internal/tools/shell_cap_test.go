package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/chrisvoo/axon/internal/config"
)

func TestRunShellRefusesAboveTheCap(t *testing.T) {
	pm := NewProcManager()
	for i := 0; i < MaxRunningCommands; i++ {
		pm.Register("sleep", func() {})
	}
	res, err := RunShell(context.Background(), &config.Config{}, nil, false, "echo hi", pm)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "error" || !strings.Contains(res.ErrorMessage, "too many running commands") {
		t.Fatalf("expected the cap error, got %+v", res)
	}
}
