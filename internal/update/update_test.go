package update

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		name    string
		changed []string
		known   bool
		goBuild bool
		dash    bool
		reason  string
	}{
		{"docs only", []string{"README.md", "CLAUDE.md"}, true, false, false, "nothing relevant"},
		{"go source", []string{"internal/mcp/mcp.go"}, true, true, false, "Go sources"},
		{"go deps", []string{"go.mod", "go.sum"}, true, true, false, "Go dependencies"},
		{"web without dist", []string{"web/src/App.tsx"}, true, true, true, "rebuild with pnpm"},
		{"web with dist", []string{"web/src/App.tsx", distDir + "index.html"}, true, true, false, "committed dashboard build"},
		{"web deps with dist", []string{"web/pnpm-lock.yaml", distDir + "index.html"}, true, true, false, "dashboard dependencies"},
		{"web deps only", []string{"web/package.json"}, true, true, true, "dashboard dependencies"},
		{"web build output is ignored", []string{"web/dist/a.js", "web/node_modules/x"}, true, false, false, "nothing relevant"},
		{"unknown built commit", nil, false, true, false, "no known commit"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := classify(c.changed, c.known)
			if p.RebuildGo != c.goBuild || p.RebuildDashboard != c.dash {
				t.Fatalf("go=%v dashboard=%v, want go=%v dashboard=%v", p.RebuildGo, p.RebuildDashboard, c.goBuild, c.dash)
			}
			if !strings.Contains(strings.Join(p.Reasons, "|"), c.reason) {
				t.Fatalf("reasons %q lack %q", p.Reasons, c.reason)
			}
		})
	}
}

// fx is a bare origin, a dev clone that pushes commits, and a work clone that
// plays the host. git runs for real. go and make are faked and recorded.
type fx struct {
	t       *testing.T
	root    string
	work    string
	exe     string
	calls   []string
	missing map[string]bool
	out     bytes.Buffer
}

func sh(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	out, err := defaultExec(dir, name, args...)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(out)
}

func write(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for p, c := range files {
		full := filepath.Join(dir, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func newFx(t *testing.T) *fx {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("axon update does not support Windows")
	}
	for k, v := range map[string]string{"GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@t", "GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@t", "GIT_CONFIG_GLOBAL": os.DevNull} {
		t.Setenv(k, v)
	}
	root := t.TempDir()
	f := &fx{t: t, root: root, work: filepath.Join(root, "work"), exe: filepath.Join(root, "bin", "axon"), missing: map[string]bool{}}
	sh(t, root, "git", "init", "-q", "--bare", "-b", "main", "origin.git")
	sh(t, root, "git", "init", "-q", "-b", "main", "dev")
	f.push(map[string]string{
		"go.mod":               modLine + "\n",
		"cmd/axon/main.go":     "package main\n",
		"README.md":            "docs\n",
		"web/src/App.tsx":      "app\n",
		distDir + "index.html": "dist\n",
	}, "init")
	sh(t, filepath.Join(root, "dev"), "git", "remote", "add", "origin", filepath.Join(root, "origin.git"))
	sh(t, filepath.Join(root, "dev"), "git", "push", "-q", "-u", "origin", "main")
	sh(t, root, "git", "clone", "-q", "origin.git", "work")
	write(t, root, map[string]string{"bin/axon": "old"})
	return f
}

// push commits files in the dev clone and pushes them when origin is set.
func (f *fx) push(files map[string]string, msg string) {
	dev := filepath.Join(f.root, "dev")
	write(f.t, dev, files)
	sh(f.t, dev, "git", "add", "-A")
	sh(f.t, dev, "git", "commit", "-q", "-m", msg)
	if out, _ := defaultExec(dev, "git", "remote"); out != "" {
		sh(f.t, dev, "git", "push", "-q")
	}
}

func (f *fx) head() string { return sh(f.t, f.work, "git", "rev-parse", "HEAD") }

func (f *fx) opts() Options {
	return Options{
		Repo:     f.work,
		Exe:      f.exe,
		BuiltRev: f.head(),
		Out:      &f.out,
		Look: func(n string) (string, error) {
			if f.missing[n] {
				return "", errors.New("not found")
			}
			return n, nil
		},
		Exec: func(dir, name string, args ...string) (string, error) {
			switch name {
			case "go":
				f.calls = append(f.calls, "go build")
				return "", os.WriteFile(args[2], []byte("new"), 0o755) // -o target
			case "make":
				f.calls = append(f.calls, "make web")
				return "", nil
			}
			return defaultExec(dir, name, args...)
		},
	}
}

func (f *fx) exeContent() string {
	b, err := os.ReadFile(f.exe)
	if err != nil {
		f.t.Fatal(err)
	}
	return string(b)
}

func (f *fx) status() string { return sh(f.t, f.work, "git", "status", "--porcelain") }

func TestUpToDateBuildsNothing(t *testing.T) {
	f := newFx(t)
	if err := Run(f.opts()); err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 0 || f.exeContent() != "old" || !strings.Contains(f.out.String(), "Nothing to rebuild") {
		t.Fatalf("calls=%v exe=%q out=%s", f.calls, f.exeContent(), f.out.String())
	}
}

func TestGoChangeRebasesAndReplacesTheBinary(t *testing.T) {
	f := newFx(t)
	o := f.opts()
	f.push(map[string]string{"cmd/axon/main.go": "package main\n// v2\n"}, "go change")
	if err := Run(o); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.calls, []string{"go build"}) || f.exeContent() != "new" {
		t.Fatalf("calls=%v exe=%q", f.calls, f.exeContent())
	}
	if f.head() != sh(t, f.work, "git", "rev-parse", "origin/main") || !strings.Contains(f.out.String(), "Restart `axon serve`") {
		t.Fatalf("not synced or no restart reminder:\n%s", f.out.String())
	}
	if _, err := os.Stat(f.exe + ".new"); !os.IsNotExist(err) {
		t.Fatal("the temporary binary must be gone")
	}
}

func TestDocsOnlyPullBuildsNothing(t *testing.T) {
	f := newFx(t)
	o := f.opts()
	f.push(map[string]string{"README.md": "more docs\n"}, "docs")
	if err := Run(o); err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 0 || f.exeContent() != "old" {
		t.Fatalf("calls=%v exe=%q", f.calls, f.exeContent())
	}
}

func TestWebChangeRebuildsTheDashboardOnlyWithoutANewDist(t *testing.T) {
	f := newFx(t)
	o := f.opts()
	f.push(map[string]string{"web/src/App.tsx": "app v2\n"}, "web only")
	if err := Run(o); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.calls, []string{"make web", "go build"}) {
		t.Fatalf("calls=%v", f.calls)
	}

	f = newFx(t)
	o = f.opts()
	f.push(map[string]string{"web/src/App.tsx": "app v2\n", distDir + "index.html": "dist v2\n"}, "web and dist")
	if err := Run(o); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.calls, []string{"go build"}) {
		t.Fatalf("calls=%v", f.calls)
	}
}

func TestMissingPnpmKeepsTheCommittedDashboard(t *testing.T) {
	f := newFx(t)
	o := f.opts()
	f.missing["pnpm"] = true
	f.push(map[string]string{"web/src/App.tsx": "app v2\n"}, "web only")
	if err := Run(o); err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 0 || !strings.Contains(f.out.String(), "warning: pnpm not found") {
		t.Fatalf("calls=%v out=%s", f.calls, f.out.String())
	}

	// A Go change still builds when the dashboard cannot.
	f.push(map[string]string{"cmd/axon/main.go": "package main\n// v2\n"}, "go")
	if err := Run(f.opts()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.calls, []string{"go build"}) {
		t.Fatalf("calls=%v", f.calls)
	}
}

func TestUnknownBuiltRevisionRebuilds(t *testing.T) {
	f := newFx(t)
	o := f.opts()
	o.BuiltRev = ""
	if err := Run(o); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.calls, []string{"go build"}) {
		t.Fatalf("calls=%v", f.calls)
	}
}

func TestEarlierSkippedBuildIsRetried(t *testing.T) {
	f := newFx(t)
	o := f.opts() // built from the first commit
	f.push(map[string]string{"cmd/axon/main.go": "package main\n// v2\n"}, "go")
	if err := Run(Options{Repo: o.Repo, Exe: o.Exe, Out: o.Out, Look: o.Look, Exec: func(dir, name string, args ...string) (string, error) {
		if name == "go" {
			return "", errors.New("boom") // the build fails after the pull
		}
		return o.Exec(dir, name, args...)
	}}); err == nil {
		t.Fatal("the failed build must be reported")
	}
	if f.exeContent() != "old" {
		t.Fatal("a failed build must keep the old binary")
	}
	if err := Run(o); err != nil { // BuiltRev still names the first commit
		t.Fatal(err)
	}
	if f.exeContent() != "new" {
		t.Fatal("the next update must rebuild")
	}
}

func TestGeneratedDistIsRestoredButOtherChangesBlock(t *testing.T) {
	f := newFx(t)
	write(t, f.work, map[string]string{distDir + "index.html": "rebuilt\n", distDir + "assets/new.js": "x\n"})
	if err := Run(f.opts()); err != nil {
		t.Fatal(err)
	}
	if f.status() != "" {
		t.Fatalf("dist must be restored:\n%s", f.status())
	}

	write(t, f.work, map[string]string{"README.md": "local edit\n"})
	err := Run(f.opts())
	if err == nil || !strings.Contains(err.Error(), "README.md") {
		t.Fatalf("a local edit must block the update, got %v", err)
	}
	if err := func() error { o := f.opts(); o.Force = true; return Run(o) }(); err != nil {
		t.Fatal(err)
	}
	if f.status() != "" {
		t.Fatalf("-force must discard the edit:\n%s", f.status())
	}
}

func TestConflictingLocalCommitAbortsTheRebase(t *testing.T) {
	f := newFx(t)
	write(t, f.work, map[string]string{"README.md": "local\n"})
	sh(t, f.work, "git", "commit", "-qam", "local")
	local := f.head()
	f.push(map[string]string{"README.md": "upstream\n"}, "upstream")

	if err := Run(f.opts()); err == nil || !strings.Contains(err.Error(), "-force") {
		t.Fatalf("a conflict must fail and mention -force, got %v", err)
	}
	if f.status() != "" || f.head() != local {
		t.Fatalf("the rebase must be aborted and the tree clean:\n%s", f.status())
	}

	o := f.opts()
	o.Force = true
	if err := Run(o); err != nil {
		t.Fatal(err)
	}
	if f.head() != sh(t, f.work, "git", "rev-parse", "origin/main") {
		t.Fatal("-force must reset to upstream")
	}
}

func TestCheckChangesNothing(t *testing.T) {
	f := newFx(t)
	o := f.opts()
	before := f.head()
	f.push(map[string]string{"cmd/axon/main.go": "package main\n// v2\n"}, "go")
	o.Check = true
	if err := Run(o); err != nil {
		t.Fatal(err)
	}
	if f.head() != before || len(f.calls) != 0 || f.exeContent() != "old" {
		t.Fatalf("-check must not change anything: calls=%v", f.calls)
	}
	if !strings.Contains(f.out.String(), "1 behind") || !strings.Contains(f.out.String(), "Go sources changed") {
		t.Fatalf("the report must show the plan:\n%s", f.out.String())
	}
}

func TestFindRepo(t *testing.T) {
	f := newFx(t)
	sub := filepath.Join(f.work, "cmd", "axon")
	got, err := FindRepo(sub)
	if err != nil || got != f.work {
		t.Fatalf("got %q, %v", got, err)
	}
	if _, err := FindRepo(t.TempDir()); err == nil {
		t.Fatal("a directory outside a checkout must fail")
	}
}
