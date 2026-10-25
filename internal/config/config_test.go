package config

import (
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{"both keys", Config{APIKey: "a", OperatorKey: "o"}, ""},
		{"no API key", Config{OperatorKey: "o"}, "missing API key"},
		{"no operator key", Config{APIKey: "a"}, "missing operator key"},
		{"same key", Config{APIKey: "k", OperatorKey: "k"}, "must be different"},
	}
	for _, tc := range cases {
		err := tc.cfg.Validate()
		switch {
		case tc.wantErr == "" && err != nil:
			t.Errorf("%s: unexpected error %v", tc.name, err)
		case tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)):
			t.Errorf("%s: got %v, want %q", tc.name, err, tc.wantErr)
		}
	}
}

func TestSaveAndLoadKeepBothKeys(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("APPDATA", dir)

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.APIKey, cfg.OperatorKey = "axon_k_test", "axon_o_test"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.APIKey != "axon_k_test" || got.OperatorKey != "axon_o_test" {
		t.Fatalf("keys were not kept: %q, %q", got.APIKey, got.OperatorKey)
	}
}
