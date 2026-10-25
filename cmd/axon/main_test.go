package main

import (
	"strings"
	"testing"
)

func TestGenKey(t *testing.T) {
	a, err := genKey(apiKeyPrefix)
	if err != nil {
		t.Fatal(err)
	}
	o, err := genKey(operatorKeyPrefix)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(a, "axon_k_") || !strings.HasPrefix(o, "axon_o_") {
		t.Fatalf("unexpected prefixes: %q, %q", a, o)
	}
	if len(a) != len("axon_k_")+64 || len(o) != len("axon_o_")+64 {
		t.Fatalf("a key must hold 256 bits (64 hex characters): %q, %q", a, o)
	}
	other, _ := genKey(apiKeyPrefix)
	if a == other {
		t.Fatal("two keys must differ")
	}
}
