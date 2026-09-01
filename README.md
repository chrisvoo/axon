# Axon

Axon is a **standalone Go agent** that listens on **HTTPS** and exposes an [**MCP**](https://modelcontextprotocol.io) (Model Context Protocol) API so an AI coding agent can run shell commands and inspect files on a remote machine.

- [Axon](#axon)
  - [Features](#features)
  - [Install](#install)
    - [From source (module proxy — installs binary to `$GOPATH/bin`)](#from-source-module-proxy--installs-binary-to-gopathbin)
    - [From a local clone (development / remote machine)](#from-a-local-clone-development--remote-machine)
    - [Linux / macOS (script)](#linux--macos-script)
    - [Windows (PowerShell)](#windows-powershell)
  - [Quick start (remote machine)](#quick-start-remote-machine)
    - [MCP config](#mcp-config)
    - [Web dashboard](#web-dashboard)
  - [Cloudflare Tunnel (public access without port forwarding)](#cloudflare-tunnel-public-access-without-port-forwarding)
    - [Quick tunnel (no account needed — temporary URL)](#quick-tunnel-no-account-needed--temporary-url)
    - [Named tunnel (permanent URL — recommended for regular use)](#named-tunnel-permanent-url--recommended-for-regular-use)
      - [One-time setup (on the Linux machine)](#one-time-setup-on-the-linux-machine)
      - [Running with the named tunnel](#running-with-the-named-tunnel)
  - [Local development (client and server on the same machine)](#local-development-client-and-server-on-the-same-machine)
    - [Dashboard UI demo mode (no server needed)](#dashboard-ui-demo-mode-no-server-needed)
  - [Commands](#commands)
  - [Configuration](#configuration)
  - [MCP tools](#mcp-tools)
  - [Security notes](#security-notes)
  - [License](#license)


## Features

- Uses Cloudflare as tunnel for exposing the remote machine to the coding agent
- **API key** authentication (`Authorization: Bearer axon_k_...`)
- **Optional IP allowlist** and **per-IP rate limiting**
- **Command denylist** (default patterns for obviously dangerous commands)
- **Read-only mode** (disable shell / writes / edits)
- **Interactive commands**: detects stalled output and returns `input_required`; use **`send_input`** and **`cancel_command`**
- **Audit log** (JSON lines) for tool invocations
- Refuses to run as **root** on Unix. Shell commands containing `sudo` are never executed automatically — they are forwarded to the dashboard as a **copy-paste dialog** for the remote user to run manually

## Install

### From source (module proxy — installs binary to `$GOPATH/bin`)

```bash
go install github.com/chrisvoo/axon/cmd/axon@latest
```

### From a local clone (development / remote machine)

Requires Go 1.21+. No build step needed — `go run` compiles and executes in one shot:

```bash
git clone https://github.com/chrisvoo/axon.git
cd axon

go run ./cmd/axon init    # first-time setup
go run ./cmd/axon serve   # start the server
```

Or build a permanent binary first:

```bash
go build -o axon ./cmd/axon
./axon init
./axon serve
```

> **Linux firewall tip:** if you want Cursor on another machine to reach Axon, open the listen port first:
> ```bash
> sudo ufw allow 8443/tcp
> ```
> Alternatively, use `axon serve -tunnel` (see [Cloudflare Tunnel](#cloudflare-tunnel-public-access-without-port-forwarding)) to get a public URL without touching the firewall.

### Linux / macOS (script)

Requires `curl` *or* `wget`. Installs to `~/.local/bin/axon`:

```bash
curl -fsSL https://raw.githubusercontent.com/chrisvoo/axon/main/scripts/install.sh | bash
```

Set `AXON_REPO` / `AXON_VERSION` if you fork or pin a release.

### Windows (PowerShell)

```powershell
irm https://raw.githubusercontent.com/chrisvoo/axon/main/scripts/install.ps1 | iex
```

## Quick start (remote machine)

```bash
axon init    # TLS cert, API key, default denylist under ~/.axon/
axon serve   # listens on https://0.0.0.0:8443/mcp by default
```

On the machine where Axon runs, **`axon serve`** prints the **API key**, **TLS fingerprint**, and a ready-to-paste MCP snippet. You reuse the same values in the AI agent on your **operator** machine

### MCP config

Share the MCP configuration on the machine where your coding agent runs. Replace host, port, and token with **your** values from `axon serve` (or the dashboard **Copy snippet**).

```json
{
  "mcpServers": {
    "axon": {
      "url": "https://YOUR_REMOTE_HOST:8443/mcp",
      "headers": {
        "Authorization": "Bearer axon_k_..."
      }
    }
  }
}
```

Alternatively, in case you use Cursor, it supports registering an MCP server from a **`cursor://`** URL. The authoritative format, examples, and an **online helper to generate links** (Base64-encode the config for you) are here:

**[Cursor — MCP install links](https://cursor.com/docs/context/mcp/install-links)**

Shape of the link (from Cursor’s docs):

```text
cursor://anysphere.cursor-deeplink/mcp/install?name=axon&config=BASE64(JSON)
```

> [!WARNING] Security notes
> The `config` query parameter **embeds your API key** (inside Base64 JSON).
> **Anyone who has the full install URL can act as your MCP client.** Do not post it
> publicly, always use a secure channel for sharing it.

Remember to reload your AI agent

### Web dashboard

Axon serves a small **dashboard** at the **origin** of the agent, e.g. `https://YOUR_REMOTE_HOST:8443/` (not the `/mcp` path). When using the default self-signed certificate, you will need to accept the browser's certificate warning first. With a Cloudflare tunnel or `-dev` mode no certificate warning appears. Enter the **same API key** to open the WebSocket stream.

In this dashboard you can follow the inverse chronological order of the commands sent to your machine from the AI agent. If privileged commands need to be run, a dialog box will appear showing you the command to paste.

## Cloudflare Tunnel (public access without port forwarding)

If the remote machine is behind NAT or you don't want to open firewall ports, Cloudflare Tunnel lets you expose Axon over a public HTTPS URL with no port forwarding, no public IP, and no TLS certificate management.

**Requirements:** [`cloudflared`](https://github.com/cloudflare/cloudflared/releases) must be in `PATH` on the remote machine.

```bash
# install cloudflared (Linux example)
curl -L https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64 \
  -o /usr/local/bin/cloudflared && chmod +x /usr/local/bin/cloudflared
```

### Quick tunnel (no account needed — temporary URL)

The fastest way to get started. Axon spawns `cloudflared` automatically and prints a ready-to-paste snippet:

```bash
axon init    # one-time setup
axon serve -tunnel
```

After a few seconds Axon prints:

```
Cloudflare quick tunnel ready: https://random-words.trycloudflare.com

Add to .cursor/mcp.json:
{
  "mcpServers": {
    "axon": {
      "url": "https://random-words.trycloudflare.com/mcp",
      "headers": { "Authorization": "Bearer axon_k_..." }
    }
  }
}
```

Share it with the AI agent on the client machine.

> [!NOTE]
> The `trycloudflare.com` URL changes on every restart. You will need to update `mcp.
> json` and reload Cursor each time. For a permanent URL, use a named tunnel (below).

### Named tunnel (permanent URL — recommended for regular use)

A named tunnel gives you a **stable hostname** (e.g. `https://axon.yourdomain.com`) that survives restarts. Set it up once; never touch `mcp.json` again.

#### One-time setup (on the Linux machine)

**Prerequisites:** a domain (or subdomain) managed by Cloudflare DNS, and a free Cloudflare account.

```bash
# 1. Log in — opens browser, saves ~/.cloudflared/cert.pem
cloudflared tunnel login

# 2. Create the tunnel (remember the UUID it prints)
cloudflared tunnel create axon-mcp

# 3. Map a hostname to it (replace with your actual domain)
cloudflared tunnel route dns axon-mcp axon.yourdomain.com

# 4. Create ~/.cloudflared/config.yml
cat > ~/.cloudflared/config.yml <<'EOF'
tunnel: axon-mcp
credentials-file: /home/<you>/.cloudflared/<uuid>.json
ingress:
  - hostname: axon.yourdomain.com
    service: http://localhost:8443
  - service: http_status:404
EOF
```

Replace `<you>` with your Linux username and `<uuid>` with the UUID printed in step 2.

#### Running with the named tunnel

```bash
axon init   # one-time, if not already done
axon serve -tunnel-name axon-mcp -tunnel-url https://axon.yourdomain.com
```

Axon immediately prints the permanent mcp.json snippet and one-click deeplink, then starts `cloudflared tunnel run axon-mcp` in the background:

```
Named Cloudflare Tunnel: https://axon.yourdomain.com

Add to .cursor/mcp.json (permanent — update only when key changes):
{
  "mcpServers": {
    "axon": {
      "url": "https://axon.yourdomain.com/mcp",
      "headers": { "Authorization": "Bearer axon_k_..." }
    }
  }
}
```

Add this snippet to `~/.cursor/mcp.json` on your Mac **once**. Because the URL is permanent, you never need to update it — even after restarting Axon on the Linux machine.

## Local development (client and server on the same machine)

During development you can run Axon on your own machine over **plain HTTP** — no TLS setup, no certificate trust required — and point Cursor at `localhost` as if it were a remote box.

```bash
axon init   # only needed once; creates API key and denylist (cert is unused in dev mode)
axon serve -dev
# or: make dev
```

The `-dev` flag switches the server to plain HTTP and prints a ready-to-paste snippet:

```
⚠  DEV MODE — plain HTTP, no TLS. Do not expose this port externally.
Axon 0.1.0 listening on http://0.0.0.0:8443/mcp
Dashboard: http://127.0.0.1:8443/
API key:   axon_k_...

Add to .cursor/mcp.json (dev — do not commit the key):
{
  "mcpServers": {
    "axon-dev": {
      "url": "http://127.0.0.1:8443/mcp",
      "headers": {
        "Authorization": "Bearer axon_k_..."
      }
    }
  }
}
```

Paste the snippet into `.cursor/mcp.json` (or `~/.cursor/mcp.json` for global use), reload Cursor, and all Axon tools appear under the `axon-dev` server entry.

> **Security:** `-dev` is for local development only. The HTTP port has no transport encryption; never expose it outside `localhost`.

### Dashboard UI demo mode (no server needed)

The dashboard frontend can run entirely without a live Axon server or any network connection. This is useful for iterating on UI layout, parsing, and formatting without the overhead of initialising a server and triggering real tool calls.

**How to start it:**

```bash
cd web
VITE_DEMO_MODE=true pnpm dev
```

Open the URL Vite prints (usually `http://localhost:5173`). The dashboard loads directly into the activity view — no API-key prompt, no WebSocket connection attempt.

**What changes in demo mode:**

- The sidebar shows a **Demo mode** panel instead of the "Change API key" button.
- A dropdown lets you choose a scenario; clicking **Inject event** fires that scenario's events through the same reducer that handles real WebSocket messages, with a 350 ms delay between steps so the state transitions are visible.
- For `input_required` scenarios the `InputPrompt` dialog appears as normal, but submit and cancel resolve locally — no HTTP request is made.

**Available scenarios** (chosen to exercise specific rendering paths):

| Scenario | What it exercises |
|---|---|
| `shell_exec` — ANSI git log | ANSI color codes, bold, reset sequences in `ansi.ts` |
| `shell_exec` — long output | Line-count truncation and the **Show more** button in `ActivityItem` |
| `read_file` — JSON output | The `tryPrettyJson` path that reformats raw JSON before rendering |
| `shell_exec` — error / non-zero exit | `is_error` badge, red status, non-zero `exit_code` display |
| `shell_exec` — input\_required | `InputPrompt` dialog with hint text; submit/cancel are mocked (triggered by interactive programs, not sudo — sudo is intercepted before execution) |
| `privileged_commands` modal | `PrivilegedModal` overlay with multiple command lines |

**Architecture note:** demo mode is a pure frontend concern. `VITE_DEMO_MODE` is inlined at build time by Vite, so demo code is tree-shaken out of production bundles. The `DemoPanel` component (`web/src/demo/`) simply calls the same `onEvent` callback that real WebSocket messages use — no special paths exist in the event reducer or any other component.

## Commands

| Command                 | Description                                                        |
|-------------------------|--------------------------------------------------------------------|
| `axon init`             | Create config, API key, TLS cert, denylist file                    |
| `axon serve`            | Start HTTPS MCP server (`-addr`, `-port` flags)                    |
| `axon serve -dev`       | Plain HTTP, no TLS — local development only                        |
| `axon serve -tunnel`    | Cloudflare quick tunnel — temporary public HTTPS URL               |
| `axon serve -tunnel-name N -tunnel-url U` | Named Cloudflare tunnel — permanent public URL  |
| `axon status`           | Show paths and certificate fingerprint                             |
| `axon keygen`           | Rotate API key                                                     |

## Configuration

Merged defaults + `~/.axon/config.yaml` (see `configs/default.yaml` in the repo for all keys).

Notable options:

- `ip_allowlist` — empty means any IP; otherwise only listed IPs/CIDRs
- `rate_limit_rps` — set `0` to disable
- `read_only: true` — only safe tools (`read_file`, `grep`, `glob`, `system_info`)
- `cert_file` / `key_file` — use your own certificate

## MCP tools

| Tool             | Purpose |
|------------------|---------|
| `shell`          | Run shell command (bash/sh on Unix, PowerShell/cmd on Windows) |
| `read_file`      | Read file (optional 1-based line range) |
| `write_file`     | Write/overwrite file |
| `edit_file`      | Unique string replace in a file |
| `grep`           | Regex search under a directory (scan limit) |
| `glob`           | Match files by basename pattern under a root |
| `system_info`    | Host, CPU, memory, disk summary |
| `send_input`     | Send data to stdin of a process that returned `input_required` |
| `cancel_command` | Kill a tracked process |

> **Privileged commands (`sudo`):** when the agent calls `shell` with a command containing `sudo`, Axon never executes it. Instead it emits a `privileged_commands` dashboard event, and the tool returns `{"status":"requires_user_action"}`. The remote user sees a copy-paste dialog in the dashboard and runs the command themselves.

## Security notes

- Treat the API key like a password; rotate with `axon keygen` if leaked.
- Prefer VPN or SSH tunnel if you cannot expose a port safely.
- Review and extend `denylist.txt` for your environment.

## License

MIT
