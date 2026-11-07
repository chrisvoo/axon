package security

import (
	"testing"
	"time"
)

func TestAuthGuardBlocksAfterFiveFailuresForAMinute(t *testing.T) {
	now := time.Now()
	g := NewAuthGuard()
	g.now = func() time.Time { return now }

	for i := 1; i <= 4; i++ {
		if g.Fail("1.2.3.4") || g.Blocked("1.2.3.4") {
			t.Fatalf("failure %d must not block", i)
		}
	}
	if !g.Fail("1.2.3.4") {
		t.Fatal("the fifth failure must start a block")
	}
	if !g.Blocked("1.2.3.4") {
		t.Fatal("the address must be blocked")
	}
	if g.Blocked("5.6.7.8") {
		t.Fatal("another address must stay free")
	}

	now = now.Add(authBlockFor + time.Second)
	if g.Blocked("1.2.3.4") {
		t.Fatal("the block must end after a minute")
	}
}

func TestAuthGuardForgetsFailuresAfterTheRightKey(t *testing.T) {
	g := NewAuthGuard()
	for i := 0; i < 4; i++ {
		g.Fail("1.2.3.4")
	}
	g.OK("1.2.3.4")
	for i := 0; i < 4; i++ {
		if g.Fail("1.2.3.4") {
			t.Fatal("the count must start again after the right key")
		}
	}
}
