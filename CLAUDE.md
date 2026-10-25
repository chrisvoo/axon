# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# First-time setup (generates API key, operator key, TLS cert, denylist in ~/.axon/).
# It shows the operator key once. The config keeps only a hash.
./axon init

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

## Architecture

Axon is a **remote MCP (Model Context Protocol) server** that exposes the remote machine's shell and filesystem as MCP tools, secured with two keys (the agent's API key and the operator's key) and (optionally) TLS.

### Request flow

```
AI client (Claude / Cursor)
  → POST /mcp (Bearer API key). The dashboard uses the operator key on /ws and /api/*.
  → internal/server — IP allowlist, rate limit, then the key check of the route (requireKey)
  → internal/mcp — JSON-RPC 2.0 dispatch (handleToolsCall)
  → internal/tools — shell / file / search execution
  → events.Bus.Publish() → Hub → WebSocket broadcast → React dashboard
```

### Key packages

| Package | Role |
|---|---|
| `cmd/axon` | CLI entry point: `init`, `serve`, `status`, `keygen`; terminal banner in `startup.go` |
| `internal/mcp` | JSON-RPC 2.0 handler; all tool dispatch lives in `handleToolsCall` |
| `internal/tools` | Tool implementations: shell, read/write/edit file, grep, glob, system_info, send_input, cancel_command |
| `internal/server` | HTTP(S) server, routes (`/mcp`, `/ws`, `/api/cancel`, `/api/input`, `/api/agent-config`), and the MCP client-config snippet builders used by `cmd/axon` |
| `internal/hub` | WebSocket fan-out; subscribes to `events.Bus` and pushes to all dashboard clients |
| `internal/events` | Simple pub/sub bus (buffered channels, drops on full) |
| `internal/security` | Bearer-token check, denylist, rate limiter, TLS, IP allowlist |
| `internal/config` | YAML config loader/saver; embeds the default config and denylist; key rules (`HashKey`, `OperatorKeyMatches`, `Validate`); paths from `internal/paths` |
| `internal/audit` | JSON-lines audit log of tool calls |
| `internal/tunnel` | Cloudflare tunnel integration (`trycloudflare` quick tunnel or named tunnel) |
| `internal/dashboard` | Embeds the built dashboard (`dist/`) |
| `internal/rootcheck` | Refuses to run as root on Unix |
| `web/` | React 19 + Tailwind 4 dashboard, bundled by Vite, tested with Vitest |

### Shell tool behaviour

- **`sudo` detection**: `isPrivilegedCmd` in `mcp.go` does a word-boundary scan for `sudo`. Matching commands are never executed — they are published as `privileged_commands` events to the dashboard for manual copy-paste. The tool returns `{"status":"requires_user_action"}`. This is a guardrail, not a security boundary: `/usr/bin/sudo` or a script that calls `sudo` is not detected.
- **Interactive stall**: non-sudo commands that produce no output for `InputStallSec` (default 5 s) return `{"status":"input_required", "process_id":"..."}`. The caller must use `send_input` or `cancel_command` to continue. The command outlives the HTTP request and ends at `shell_timeout`. When it ends, `ProcManager.Finish` keeps its final result for 2 minutes, so `send_input` (empty data) still returns the output and exit code if the operator answered in the dashboard.
- **Operator secrets**: the dashboard sends `POST /api/input` with `secret` (default true). `tools.SendOperatorInput` registers each typed line in the `ProcManager` secret store (`AddSecret`, with its JSON-escaped form) before it writes to the pty. `AddSecret` needs a live process: an empty, unknown or ended process registers nothing. A secret lasts as long as its process, then 10 minutes (`releaseSecretsLocked`). The store is shared by all processes. Anything the agent or the dashboard can read must pass `ProcManager.Redact`: shell results (`RunShell`), `send_input` and finished results, and the final `text` of every tool call in `mcp.handleToolsCall` (also errors and event details). The audit logger redacts `Detail` through `SetRedactor`. Never return `TrackedProc.snapshot()` or `OutBuf` directly. Events and audit lines never contain the input text. Redaction is best effort: exact text only.
- **Read-only mode**: if `cfg.ReadOnly` is true, `shell`, `write_file` and `edit_file` are disabled.
- **Denylist**: `~/.axon/denylist.txt` is loaded at startup (`axon init` writes it from the embedded `internal/config/denylist.txt`). Commands that contain any pattern as a substring are blocked. A missing file means an empty denylist.

### Dashboard / WebSocket

The React dashboard connects to `GET /ws` (WebSocket). The user pastes the operator key in `ConnectScreen` (opening `/#<operator key>` also works). `App.tsx` reads the key from the URL hash (if present), stores it in `sessionStorage` and removes the hash. `axon serve` opens the dashboard without a key in the address. The socket then sends the key as the `?key=` query parameter, which `hub.ServeWS` validates against the operator key. `/api/input`, `/api/cancel` and `/api/agent-config` take the operator key as a Bearer token (`requireKey` in `server.go`). `/mcp` takes only the API key. The agent must never get the operator key: it can register secrets and answer prompts. The config stores only `OperatorKeyHash` (`config.HashKey`, SHA-256). `Config.OperatorKeyMatches` hashes the token, so the stored hash does not work as a key. The agent can read `config.yaml`, so never store or print the operator key anywhere else than once in `axon init` and `axon keygen -operator` (`ensureKeys`, `newOperatorKey`). `Config.Validate` requires two different keys. `AddSecret` needs a live process: an empty id registers nothing, and `/api/input` rejects an empty `process_id`. `GET /api/agent-config` gives the API key to the dashboard for its setup snippet. The hub fans out every `events.Event` (tool calls, results, input_required, system stats) to all connected clients. `events.Bus` drops events when a subscriber buffer is full.

### Dev vs production mode

`serve -dev`, `-tunnel` and `-tunnel-name` all run plain HTTP on `127.0.0.1`. Without them, the server uses TLS (self-signed certificate by default) on `listen_addr` (default `0.0.0.0`).

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
