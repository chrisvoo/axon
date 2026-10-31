package tools

import (
	"testing"
	"time"
)

func TestProcManager_FinishedResultExpires(t *testing.T) {
	pm := NewProcManager()
	now := time.Unix(1000, 0)
	pm.now = func() time.Time { return now }
	code := 3
	pm.Finish("p1", ShellResult{Status: "completed", Stdout: "out", ExitCode: &code})

	res, ok := pm.Finished("p1")
	if !ok || res.Stdout != "out" || res.ExitCode == nil || *res.ExitCode != 3 {
		t.Fatalf("expected the stored result, got %+v ok=%v", res, ok)
	}

	now = now.Add(finishedTTL - time.Second)
	if _, ok := pm.Finished("p1"); !ok {
		t.Fatal("result must still be available just before the TTL")
	}

	now = now.Add(2 * time.Second)
	if _, ok := pm.Finished("p1"); ok {
		t.Fatal("result must expire after the TTL")
	}
}

func TestProcManager_FinishedUnknownID(t *testing.T) {
	if _, ok := NewProcManager().Finished("nope"); ok {
		t.Fatal("unknown id must not have a result")
	}
}

func TestProcManager_FinishDropsExpiredResults(t *testing.T) {
	pm := NewProcManager()
	now := time.Unix(1000, 0)
	pm.now = func() time.Time { return now }
	pm.Finish("old", ShellResult{Status: "completed"})

	now = now.Add(finishedTTL + time.Second)
	pm.Finish("new", ShellResult{Status: "completed"})

	if len(pm.finished) != 1 {
		t.Fatalf("expired results must be pruned, %d left", len(pm.finished))
	}
}
