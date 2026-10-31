# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Context documents

Read the document that matches your task before you change code. This file keeps only the commands, the rules and the code-level invariants.

| Document | Read it when you touch | Contents |
|---|---|---|
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | `internal/server`, `internal/mcp`, `internal/tools`, `internal/hub`, events | Actors, stack, request flow, shell call sequence, operator secrets, events, trust limits |
| [docs/SECURITY.md](docs/SECURITY.md) | keys, denylist, `sudo` check, secret store, TLS, audit log | Operating notes, review findings (high, medium, low), guarantees of the secret path |
| [README.md](README.md) | commands, flags, config keys, MCP tools | User-facing setup and reference |

Keep the documents true. A change that fixes a finding updates its entry in `docs/SECURITY.md`. A change to a command, flag, config key or tool updates `README.md` and the commands in this file.

## Quick start

```bash
make build                       # compile the dashboard, then the Go binary (./axon)
./axon init                      # first time only: API key, operator key, TLS cert, denylist in ~/.axon/
make dev                         # plain HTTP on 127.0.0.1, no TLS, for local development
./axon update                    # later: pull the source and rebuild what is stale
```

`axon init` shows the operator key once. The config keeps only a hash. `axon update` never restarts `axon serve`: restart it to run the new build.

## Commands

```bash
# First-time setup (generates API key, operator key, TLS cert, denylist in ~/.axon/).
./axon init

# Pull the latest source and rebuild what is stale
./axon update
./axon update -check            # report only, change nothing
./axon update -force            # discard local changes and commits
./axon update -repo PATH        # use this checkout

# Build (compiles web dashboard, then Go binary)
make build

# Dev server — plain HTTP, no TLS (use this during local development)
make dev
# or: go run ./cmd/axon serve -dev -no-browser

# Run all Go tests
go test ./...

# Run a single package's tests
go test ./internal/server/...

# Frontend (in web/)
pnpm test          # run once
pnpm test:watch    # watch mode
pnpm build         # output goes to web/dist/, then copied by `make build`
```

The `make build` target always rebuilds the frontend first (`web/dist/`) and copies it into `internal/dashboard/dist/`, which is embedded into the binary via `//go:embed`. Changing frontend files without re-running `make build` will not affect the binary. `internal/dashboard/dist/` is committed so that `go install` works.

## Versioning

The version is the `version` constant in `cmd/axon/main.go`. It is the only source. Every change that you make to the code must bump it in the same commit, by the size of the change:

- **Big change** (breaking: a removed or renamed command, flag, config key or API; a new key model): bump the major version (1.2.3 → 2.0.0).
- **Medium change** (a new feature, command or tool, with no break): bump the minor version and reset the patch (1.2.3 → 1.3.0).
- **Minor change** (a bug fix, a refactor, a small tweak): bump the patch version (1.2.3 → 1.2.4).

A change to documents or tests only does not bump the version. `axon update` shows the version of the binary that it builds.

## Architecture in brief

Axon is a **remote MCP (Model Context Protocol) server**. It exposes the machine's shell and files as MCP tools. Two keys secure it (the agent's API key and the operator's key), and optionally TLS. For the full picture and diagrams, see [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

```
AI client (Claude / Cursor)
  → POST /mcp (Bearer API key). The dashboard uses the operator key on /ws and /api/*.
  → internal/server — IP allowlist, rate limit, then the key check of the route (requireKey)
  → internal/mcp — JSON-RPC 2.0 dispatch (handleToolsCall)
  → internal/tools — shell / file / search execution
  → events.Bus.Publish() → Hub → WebSocket broadcast → React dashboard
```

| Package | Role |
|---|---|
| `cmd/axon` | CLI entry point: `init`, `serve`, `status`, `keygen`, `update`, `version`; terminal banner in `startup.go` |
| `internal/mcp` | JSON-RPC 2.0 handler; all tool dispatch lives in `handleToolsCall` |
| `internal/tools` | Tool implementations: shell, read/write/edit file, grep, glob, system_info, send_input, cancel_command |
| `internal/server` | HTTP(S) server, routes (`/mcp`, `/ws`, `/api/cancel`, `/api/input`, `/api/agent-config`), and the MCP client-config snippet builders used by `cmd/axon` |
| `internal/hub` | WebSocket fan-out; subscribes to `events.Bus` and pushes to all dashboard clients |
| `internal/events` | Simple pub/sub bus (buffered channels, drops on full) |
| `internal/security` | Bearer-token check, denylist, rate limiter, TLS, IP allowlist |
| `internal/config` | YAML config loader/saver; embeds the default config and denylist; key rules (`HashKey`, `OperatorKeyMatches`, `Validate`); paths from `internal/paths` |
| `internal/audit` | JSON-lines audit log of tool calls |
| `internal/update` | `axon update`: git sync, stale-build assessment, rebuild (single file, `Exec`/`Look` seams for tests) |
| `internal/tunnel` | Cloudflare tunnel integration (`trycloudflare` quick tunnel or named tunnel) |
| `internal/dashboard` | Embeds the built dashboard (`dist/`) |
| `internal/rootcheck` | Refuses to run as root on Unix |
| `web/` | React 19 + Tailwind 4 dashboard, bundled by Vite, tested with Vitest |

`serve -dev`, `-tunnel` and `-tunnel-name` all run plain HTTP on `127.0.0.1`. Without them, the server uses TLS (self-signed certificate by default) on `listen_addr` (default `0.0.0.0`).

## Code-level invariants

These rules are easy to break by accident. The reasons are in [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) and [docs/SECURITY.md](docs/SECURITY.md).

### Shell tool

- **`sudo` detection**: `isPrivilegedCmd` in `mcp.go` does a word-boundary scan for `sudo`. `maskInertText` first blanks comments, single-quoted text and double-quoted text without `$(` or a backtick, so `echo "needs sudo"` passes. It masks nothing when the command has a heredoc (`<<`) or an unquoted runner word (`sh`, `bash`, `eval`, `ssh`, `xargs`, `exec`, `env`: `runnerWords`), and a quoted word in command position (`"sudo" id`) stays visible. `$'...'` reads backslash escapes. A matching command never runs. It is published as a `privileged_commands` event, and the tool returns `{"status":"requires_user_action"}`. This is a guardrail, not a security boundary.
- **Interactive stall**: a non-sudo command with no output for `InputStallSec` (default 5 s) returns `{"status":"input_required", "process_id":"..."}`. The command outlives the HTTP request and ends at `shell_timeout`. `ProcManager.Finish` keeps its final result for 2 minutes, so `send_input` (empty data) still returns the output and exit code.
- **Read-only mode**: if `cfg.ReadOnly` is true, `shell`, `write_file` and `edit_file` are disabled.
- **Denylist**: `~/.axon/denylist.txt` is loaded at startup (`axon init` writes it from the embedded `internal/config/denylist.txt`). A command that contains a pattern as a substring is blocked. A missing file means an empty denylist.

### Keys and dashboard

- The API key opens `/mcp` only. The operator key opens `/ws` (as the `?key=` query, checked in `hub.ServeWS`), `/api/input`, `/api/cancel` and `/api/agent-config` (as a Bearer token, `requireKey` in `server.go`).
- The agent must never get the operator key: it can register secrets and answer prompts. The config stores only `OperatorKeyHash` (`config.HashKey`, SHA-256). `Config.OperatorKeyMatches` hashes the token, so the stored hash does not work as a key. `Config.Validate` requires two different keys.
- The agent can read `config.yaml`. Never store or print the operator key anywhere other than once in `axon init` and `axon keygen -operator` (`ensureKeys`, `newOperatorKey`).
- The dashboard reads the key from the URL hash (`App.tsx`), keeps it in `sessionStorage` and removes the hash. `ConnectScreen` takes a pasted key. `axon serve` opens the dashboard without a key in the address.
- `GET /api/agent-config` gives the API key to the dashboard for its setup snippet.
- The hub fans out every `events.Event` (tool calls, results, input_required, system stats) to all clients. `events.Bus` drops events when a subscriber buffer is full.

### Operator secrets

- The dashboard sends `POST /api/input` with `secret` (default true). `tools.SendOperatorInput` registers each typed line in the `ProcManager` secret store (`AddSecret`, with its JSON-escaped form) before it writes to the pty.
- `AddSecret` needs a live process: an empty, unknown or ended process registers nothing, and `/api/input` rejects an empty `process_id`. A secret lasts as long as its process, then 10 minutes (`releaseSecretsLocked`). The store is shared by all processes.
- Anything the agent or the dashboard can read must pass `ProcManager.Redact`: shell results (`RunShell`), `send_input` and finished results, and the final `text` of every tool call in `mcp.handleToolsCall` (also errors and event details). The audit logger redacts `Detail` through `SetRedactor`.
- Never return `TrackedProc.snapshot()` or `OutBuf` directly. Events and audit lines never contain the input text. Redaction is best effort: exact text only.

### Update flow (`axon update`)

`update.Run` never restarts `axon serve`, and it refuses to run on Windows (a running `.exe` cannot be replaced). It works in three steps:

1. **Sync**: `git fetch`, then `git rebase @{u}`. A conflict aborts the rebase and returns an error. `-force` does `git reset --hard @{u}`. Dirty paths under `internal/dashboard/dist/` are output of an earlier update: they are restored first. Any other dirty path blocks the update unless `-force`. `-check` skips all writes and assesses `@{u}`.
2. **Assess** (`classify`): the diff base is the commit the binary was built from (`debug.ReadBuildInfo` `vcs.revision`, from `update.BuiltRevision`), else the old HEAD. So a failed build is retried by the next run. A Go change (`*.go`, `go.mod`, `go.sum`) or a changed committed dist rebuilds the binary (dist is embedded). A `web/` change without a dist change rebuilds the dashboard. An unknown built commit rebuilds the binary. A docs-only diff builds nothing.
3. **Build**: the dashboard with `make web` (needs `make` and `pnpm`; else warn and keep the committed dist), then `go build -o <exe>.new ./cmd/axon` inside the repo (so VCS stamping works) and an atomic rename over the executable.

`internal/dashboard/dist/` stays committed: `go install ...@latest` and plain `go build` embed it without pnpm, and `//go:embed all:dist` does not compile without that directory. Tests (`internal/update/update_test.go`) use real `git` on temp repos and fake `go`/`make`.

## axon MCP shell session behaviour (when using this server via MCP)

Each `mcp__axon__shell` call runs in an **ephemeral shell session**. Background processes (`&`) die when the shell exits — `nohup` and `setsid` are not sufficient; the axon daemon reaps all children regardless.

**For truly detached long-running jobs, use `systemd-run --user`:**
```bash
systemd-run --user --no-block --unit=myjob bash /path/to/script.sh
systemctl --user status myjob.service
journalctl --user -u myjob.service -f
```
Note: pass `bash /path/to/script` rather than the script directly (`systemd-run` will fail with "Permission denied" otherwise).

**5 s silence timeout:** commands that produce no stdout for ~5 s return `input_required`. Avoid `sleep N` in commands you want to run synchronously — split them into separate shell calls, or use a keepalive echo loop:
```bash
nohup long-command > /tmp/job.log 2>&1 &
PID=$!
while kill -0 $PID 2>/dev/null; do echo "still running..."; sleep 4; done
cat /tmp/job.log
```
