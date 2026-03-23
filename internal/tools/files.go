package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chrisvoo/axon/internal/config"
)

// ReadFile reads a file with optional line range [startLine, endLine] (1-based, inclusive).
func ReadFile(path string, startLine, endLine int) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if startLine <= 0 && endLine <= 0 {
		return string(data), nil
	}
	lines := strings.Split(string(data), "\n")
	if startLine < 1 {
		startLine = 1
	}
	if endLine <= 0 || endLine > len(lines) {
		endLine = len(lines)
	}
	if startLine > endLine {
		return "", fmt.Errorf("invalid line range")
	}
	out := strings.Join(lines[startLine-1:endLine], "\n")
	return out, nil
}

// WriteFile writes content to path (creates parent dirs).
func WriteFile(cfg *config.Config, path, content string) error {
	if cfg.ReadOnly {
		return fmt.Errorf("read_only mode: write_file disabled")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// EditFile replaces oldString with newString exactly once.
func EditFile(cfg *config.Config, path, oldString, newString string) error {
	if cfg.ReadOnly {
		return fmt.Errorf("read_only mode: edit_file disabled")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	s := string(data)
	if !strings.Contains(s, oldString) {
		return fmt.Errorf("old_string not found")
	}
	if strings.Count(s, oldString) > 1 {
		return fmt.Errorf("old_string matches multiple times; make it unique")
	}
	s = strings.Replace(s, oldString, newString, 1)
	return os.WriteFile(path, []byte(s), 0o644)
}
