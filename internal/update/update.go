// Package update implements `axon update`: sync the source checkout with its
// upstream, work out what is stale, and rebuild only that.
package update

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
)

const (
	modLine = "module github.com/chrisvoo/axon"
	distDir = "internal/dashboard/dist/"
)

// Options configure Run. Exec and Look are seams for tests.
type Options struct {
	Repo     string // source checkout
	Exe      string // binary to replace
	BuiltRev string // commit the running binary was built from ("" if unknown)
	Check    bool   // report only
	Force    bool   // discard local changes and commits
	Exec     func(dir, name string, args ...string) (string, error)
	Look     func(name string) (string, error)
	Out      io.Writer
}

// Plan says what is stale and why.
type Plan struct {
	RebuildGo        bool
	RebuildDashboard bool
	goOther          bool // Go rebuild needed for a reason other than the dashboard
	Reasons          []string
}

// BuiltRevision returns the commit this binary was built from, as stamped by
// `go build` inside a git checkout. It is "" for `go run` or `go install pkg@v`.
func BuiltRevision() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	for _, s := range bi.Settings {
		if s.Key == "vcs.revision" {
			return s.Value
		}
	}
	return ""
}

// FindRepo walks up from start to the Axon source checkout.
func FindRepo(start string) (string, error) {
	for d := start; ; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, ".git")); err == nil {
			if b, err := os.ReadFile(filepath.Join(d, "go.mod")); err == nil && strings.Contains(string(b), modLine) {
				return d, nil
			}
		}
		if filepath.Dir(d) == d {
			return "", errors.New("not a source checkout: axon update needs a git clone of axon (reinstall with install.sh or `go install` instead)")
		}
	}
}

// classify turns the changed paths into a rebuild plan.
func classify(changed []string, builtKnown bool) Plan {
	var goSrc, goDeps, webSrc, webDeps, dist bool
	for _, f := range changed {
		switch {
		case f == "go.mod" || f == "go.sum":
			goSrc, goDeps = true, true
		case strings.HasSuffix(f, ".go"):
			goSrc = true
		case strings.HasPrefix(f, distDir):
			dist = true
		case f == "web/package.json" || f == "web/pnpm-lock.yaml":
			webSrc, webDeps = true, true
		case strings.HasPrefix(f, "web/") && !strings.HasPrefix(f, "web/dist/") && !strings.HasPrefix(f, "web/node_modules/"):
			webSrc = true
		}
	}
	p := Plan{
		RebuildDashboard: webSrc && !dist, // a dist that moved with the sources is already current
		goOther:          !builtKnown || goSrc || dist,
	}
	p.RebuildGo = p.goOther || p.RebuildDashboard
	add := func(ok bool, s string) {
		if ok {
			p.Reasons = append(p.Reasons, s)
		}
	}
	add(!builtKnown, "the running binary has no known commit")
	add(goDeps, "Go dependencies changed (go.mod or go.sum)")
	add(goSrc && !goDeps, "Go sources changed")
	add(dist, "the committed dashboard build changed")
	add(webDeps, "dashboard dependencies changed (package.json or pnpm-lock.yaml)")
	add(p.RebuildDashboard, "dashboard sources changed without a new committed build: rebuild with pnpm")
	add(len(p.Reasons) == 0, "nothing relevant changed")
	return p
}

func defaultExec(dir, name string, args ...string) (string, error) {
	c := exec.Command(name, args...)
	c.Dir = dir
	// A rebase makes commits, and a host may have no git identity. Never prompt.
	c.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0",
		"GIT_COMMITTER_NAME=axon update", "GIT_COMMITTER_EMAIL=axon@localhost")
	out, err := c.CombinedOutput()
	s := strings.TrimRight(string(out), "\n")
	if err != nil {
		return s, fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, s)
	}
	return s, nil
}

// Run syncs the checkout and rebuilds what is stale.
func Run(o Options) error {
	if runtime.GOOS == "windows" {
		return errors.New("axon update does not support Windows: a running .exe cannot be replaced")
	}
	if o.Exec == nil {
		o.Exec = defaultExec
	}
	if o.Look == nil {
		o.Look = exec.LookPath
	}
	if o.Out == nil {
		o.Out = io.Discard
	}
	git := func(args ...string) (string, error) { return o.Exec(o.Repo, "git", args...) }
	val := func(args ...string) (string, error) {
		s, err := git(args...)
		return strings.TrimSpace(s), err
	}
	say := func(f string, a ...any) { fmt.Fprintf(o.Out, f+"\n", a...) }

	// 1. Sync.
	if _, err := git("fetch", "origin"); err != nil {
		return err
	}
	upstream, err := val("rev-parse", "--abbrev-ref", "@{u}")
	if err != nil {
		return fmt.Errorf("the current branch has no upstream (detached HEAD?): %w", err)
	}
	counts, err := val("rev-list", "--left-right", "--count", "HEAD...@{u}")
	if err != nil {
		return err
	}
	var ahead, behind int
	if _, err := fmt.Sscan(counts, &ahead, &behind); err != nil {
		return fmt.Errorf("unexpected rev-list output %q: %w", counts, err)
	}
	oldHead, err := val("rev-parse", "HEAD")
	if err != nil {
		return err
	}
	say("Checkout %s, upstream %s: %d behind, %d ahead.", o.Repo, upstream, behind, ahead)

	target := "HEAD"
	if o.Check {
		target = "@{u}"
	} else {
		status, err := git("status", "--porcelain")
		if err != nil {
			return err
		}
		var dirty []string
		distDirty := false
		for l := range strings.SplitSeq(status, "\n") {
			if len(l) <= 3 {
				continue
			}
			if p := l[3:]; strings.HasPrefix(p, distDir) {
				distDirty = true // output of an earlier update
			} else {
				dirty = append(dirty, p)
			}
		}
		if distDirty {
			if _, err := git("checkout", "--", distDir); err != nil {
				return err
			}
			if _, err := git("clean", "-fdq", "--", distDir); err != nil {
				return err
			}
		}
		if len(dirty) > 0 && !o.Force {
			return fmt.Errorf("local changes block the update (use -force to discard them):\n  %s", strings.Join(dirty, "\n  "))
		}
		switch {
		case o.Force && (behind > 0 || ahead > 0 || len(dirty) > 0):
			if _, err := git("reset", "--hard", "@{u}"); err != nil {
				return err
			}
			say("Reset to %s (-force).", upstream)
		case behind > 0:
			if _, err := git("rebase", "@{u}"); err != nil {
				_, _ = git("rebase", "--abort")
				return fmt.Errorf("local commits conflict with %s; the rebase was aborted (use -force to drop them): %w", upstream, err)
			}
			say("Rebased onto %s.", upstream)
		}
	}

	// 2. Assess. Diff from the commit the binary was built from, so an earlier
	// failed or skipped build is retried.
	newHead, err := val("rev-parse", target)
	if err != nil {
		return err
	}
	builtKnown := o.BuiltRev != ""
	if builtKnown {
		if _, err := git("cat-file", "-e", o.BuiltRev+"^{commit}"); err != nil {
			builtKnown = false
		}
	}
	base := oldHead
	if builtKnown {
		base = o.BuiltRev
	}
	var changed []string
	if base != newHead {
		diff, err := git("diff", "--name-only", base, newHead)
		if err != nil {
			return err
		}
		changed = strings.Split(diff, "\n")
	}
	p := classify(changed, builtKnown)
	for _, r := range p.Reasons {
		say("  - %s", r)
	}
	if o.Check {
		say("-check: nothing was changed.")
		return nil
	}

	// 3. Build.
	if p.RebuildDashboard {
		for _, tool := range []string{"make", "pnpm"} {
			if _, err := o.Look(tool); err != nil {
				say("warning: %s not found; keeping the committed dashboard build.", tool)
				p.RebuildDashboard = false
				break
			}
		}
	}
	if p.RebuildDashboard {
		say("Rebuilding the dashboard (make web)...")
		if _, err := o.Exec(o.Repo, "make", "web"); err != nil {
			return err
		}
	}
	if !p.goOther && !p.RebuildDashboard {
		say("Nothing to rebuild.")
		return nil
	}
	say("Building axon...")
	tmp := o.Exe + ".new"
	if _, err := o.Exec(o.Repo, "go", "build", "-o", tmp, "./cmd/axon"); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, o.Exe); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	say("Replaced %s. Restart `axon serve` to run the new build.", o.Exe)
	return nil
}
