# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# First-time setup (generates API key, TLS cert, denylist)
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

The `make build` target always rebuilds the frontend first (`web/dist/`) and copies it into `internal/dashboard/dist/`, which is embedded into the binary via `//go:embed`. Changing frontend files without re-running `make build` will not affect the binary.

## Architecture

Axon is a **remote MCP (Model Context Protocol) server** that exposes the remote machine's shell and filesystem as MCP tools, secured with an API key and (optionally) TLS.

### Request flow

```
AI client (Claude / Cursor)
  → POST /mcp (Bearer token auth)
  → internal/server — rate-limit, IP allowlist, auth middleware
  → internal/mcp — JSON-RPC 2.0 dispatch (handleToolsCall)
  → internal/tools — shell / file / search execution
  → events.Bus.Publish() → Hub → WebSocket broadcast → React dashboard
```

### Key packages

| Package | Role |
|---|---|
| `cmd/axon` | CLI entry point: `init`, `serve`, `status`, `keygen` |
| `internal/mcp` | JSON-RPC 2.0 handler; all tool dispatch lives in `handleToolsCall` |
| `internal/tools` | Tool implementations: shell, read/write/edit file, grep, glob, system_info, send_input, cancel_command |
| `internal/server` | HTTP(S) server, middleware, WebSocket `/ws`, REST `/api/cancel` and `/api/input` |
| `internal/hub` | WebSocket fan-out; subscribes to `events.Bus` and pushes to all dashboard clients |
| `internal/events` | Simple pub/sub bus (buffered channels, drops on full) |
| `internal/security` | Auth, denylist, rate limiter, TLS, IP allowlist |
| `internal/config` | YAML config loader/saver; paths resolved via `internal/paths` |
| `internal/tunnel` | Cloudflare tunnel integration (`trycloudflare` quick tunnel or named tunnel) |
| `web/` | React 19 + Tailwind 4 dashboard, bundled by Vite, tested with Vitest |

### Shell tool behaviour

- **`sudo` detection**: `isPrivilegedCmd` in `mcp.go` does a substring scan for word-boundary `sudo`. Matching commands are never executed — they are published as `privileged_commands` events to the dashboard for manual copy-paste. The tool returns `{"status":"requires_user_action"}`.
- **Interactive stall**: non-sudo commands that produce no output for `InputStallSec` (default 5 s) return `{"status":"input_required", "process_id":"..."}`. The caller must use `send_input` or `cancel_command` to continue.
- **Read-only mode**: if `cfg.ReadOnly` is true, shell is disabled entirely.
- **Denylist**: `configs/denylist.txt` is loaded at startup; commands matching any pattern are blocked.

### Dashboard / WebSocket

The React dashboard connects to `GET /ws` (WebSocket). Authentication happens via the `apiKey` hash fragment in the URL (`/#<key>`), which `hub.ServeWS` validates. The hub fans out every `events.Event` (tool calls, results, input_required, system stats) to all connected clients using a non-blocking send (full subscriber buffers drop silently).

### Dev vs production mode

| | Dev (`-dev`) | Production |
|---|---|---|
| Transport | Plain HTTP | TLS (self-signed cert) |
| Listen addr | `127.0.0.1` | configurable (default `0.0.0.0`) |
| Tunnel | optional (`-tunnel` / `-tunnel-name`) | manual |

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
