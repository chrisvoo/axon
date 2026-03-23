package tools

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Grep searches files under root for regex pattern (max files scanned).
func Grep(root, pattern string, maxFiles int) ([]map[string]any, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	if maxFiles <= 0 {
		maxFiles = 500
	}
	var results []map[string]any
	scanned := 0

	fi, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return grepFile(root, re)
	}

	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if scanned >= maxFiles {
			return filepath.SkipAll
		}
		scanned++
		lines, err := grepFile(path, re)
		if err != nil {
			return nil
		}
		results = append(results, lines...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

func grepFile(path string, re *regexp.Regexp) ([]map[string]any, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []map[string]any
	sc := bufio.NewScanner(f)
	lineNum := 0
	for sc.Scan() {
		lineNum++
		line := sc.Text()
		if re.MatchString(line) {
			out = append(out, map[string]any{
				"path": path,
				"line": lineNum,
				"text": line,
			})
		}
	}
	return out, sc.Err()
}

// Glob matches file paths under root using filepath.Match on base name.
func Glob(root, pattern string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		match, err := filepath.Match(pattern, filepath.Base(path))
		if err != nil {
			return err
		}
		if match {
			out = append(out, path)
		}
		return nil
	})
	return out, err
}

// GlobPattern runs filepath.Glob for full patterns (e.g. /tmp/*.txt).
func GlobPattern(pattern string) ([]string, error) {
	if strings.Contains(pattern, "**") {
		root := filepath.Dir(pattern)
		if root == "." {
			root = "."
		}
		base := filepath.Base(pattern)
		return Glob(root, base)
	}
	return filepath.Glob(pattern)
}
