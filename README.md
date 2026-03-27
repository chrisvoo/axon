# Axon

Axon is a **standalone Go agent** that listens on **HTTPS** and exposes an [**MCP**](https://modelcontextprotocol.io) (Model Context Protocol) API so tools like **Cursor** can run shell commands and inspect files on a remote machine.

## Features

- **TLS** with auto-generated self-signed certificate (or bring your own cert paths in config)
- **API key** authentication (`Authorization: Bearer axon_k_...`)
- **Optional IP allowlist** and **per-IP rate limiting**
- **Command denylist** (default patterns for obviously dangerous commands)
- **Read-only mode** (disable shell / writes / edits)
- **Interactive commands**: detects stalled output and returns `input_required`; use **`send_input`** and **`cancel_command`**
- **Audit log** (JSON lines) for tool invocations
- Refuses to run as **root** on Unix

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
axon init    # TLS cert, API key, default denylist under ~/.axon/ (or %APPDATA%\axon on Windows)
axon serve   # listens on https://0.0.0.0:8443/mcp by default
```

On the machine where Axon runs, **`axon serve`** prints the **API key**, **TLS fingerprint**, and a ready-to-paste MCP snippet. You reuse the same values in Cursor on your **operator** machine (often your Mac).

### Where Cursor reads MCP config

Cursor loads MCP server definitions from a JSON file. There are two common locations; **only one file is used per context**, and the exact precedence can vary by Cursor version—if in doubt, use **global** config while testing.

| Scope | Typical path (macOS) | When to use it |
|--------|----------------------|----------------|
| **Global** | `~/.cursor/mcp.json` | Axon available from **any** project; simplest for a personal remote box. |
| **Per project** | `<your-repo>/.cursor/mcp.json` | Axon only for **one** codebase; good for teams or when keys differ per project. |

**Transparency:** This file is plain JSON on disk. Cursor does not “discover” Axon automatically—you (or your team) must add the `axon` entry. The **`Authorization`** header is how Axon knows the client is allowed to call tools; anyone who has the key can act as that MCP client.

### Optional: Cursor Settings UI

Some Cursor builds expose **Settings → MCP** (or similar) to add servers in the GUI. If your version supports **HTTP URL + custom headers**, you can enter:

- **URL:** `https://<host>:<port>/mcp` (must match what reaches Axon)
- **Header:** `Authorization: Bearer <same key as axon serve>`

If you do not see HTTP MCP with headers, use the **`mcp.json`** file method below—**that is fully supported** and is what Axon’s printed snippet targets.

### Minimal `mcp.json` shape (any OS)

Replace host, port, and token with **your** values from `axon serve` (or the dashboard **Copy snippet**).

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

- **`YOUR_REMOTE_HOST`**: hostname or IP **as seen from your Mac** (e.g. public IP, LAN IP, or DNS name). If Axon binds `0.0.0.0`, you still connect **to** the machine’s real address from the client.
- **`axon_k_...`**: the full API key from `axon init` / `axon serve` / `axon keygen`. It is a **secret**.

### Cursor one-click install (deeplink)

Cursor supports registering an MCP server from a **`cursor://`** URL. The authoritative format, examples, and an **online helper to generate links** (Base64-encode the config for you) are here:

**[Cursor — MCP install links](https://cursor.com/docs/context/mcp/install-links)**

Shape of the link (from Cursor’s docs):

```text
cursor://anysphere.cursor-deeplink/mcp/install?name=axon&config=BASE64(JSON)
```

The JSON you Base64-encode is **one object whose keys are server names** and whose values are the same transport config you would put under `mcpServers` in `mcp.json` (for Axon, `url` + `headers`):

```json
{
  "axon": {
    "url": "https://YOUR_REMOTE_HOST:8443/mcp",
    "headers": {
      "Authorization": "Bearer axon_k_..."
    }
  }
}
```

**For Axon you usually do not need the web generator:** `axon serve` prints a **ready-to-use** `cursor://…` line after the JSON snippet. The **dashboard** (see below) shows the same link with **Open in Cursor** and **Copy install link**, and links to Cursor’s docs if you want to verify or customize.

**Security — read this:** the `config` query parameter **embeds your API key** (inside Base64 JSON). **Anyone who has the full install URL can act as your MCP client.** Do not post it publicly, paste it into shared documents, or expose it in recordings. When you only want to share *instructions* without a secret, share the JSON *shape* and tell people to paste their own key, or use `mcp.json` on disk instead of a shareable URL.

### Cursor on macOS (step by step)

These steps are for the **Mac where Cursor runs** (operator), not necessarily the server running Axon.

1. **Reach Axon over HTTPS**  
   From the Mac, the URL `https://YOUR_REMOTE_HOST:8443/mcp` must be reachable (same network, VPN, port forward, or tunnel—whatever you chose). If you use an **IP allowlist** in Axon config, the **source IP Cursor uses** to reach the agent must be allowed (often your Mac’s public IP if Axon is on the internet).

2. **Accept or trust TLS (self-signed)**  
   Axon’s default cert is **self-signed**. Browsers and TLS clients may warn until you trust it once:
   - Open `https://YOUR_REMOTE_HOST:8443/` in **Safari** (or another browser), review the warning, and proceed if you **intentionally** trust this server (verify the **fingerprint** printed by `axon serve` / `axon status` matches what you expect).
   - For **Cursor’s MCP HTTPS client**, behavior depends on Cursor version: it may prompt, use system trust, or require a properly trusted certificate. If MCP fails with TLS errors, use a **real certificate** (Let’s Encrypt, internal CA, or `cert_file` / `key_file` in Axon config) or route through a tunnel you already trust.

3. **Create or edit the MCP file**  
   - For **global** use: create `~/.cursor/mcp.json` if it does not exist.  
   - For **one repo**: create `<project>/.cursor/mcp.json`.

4. **Add Axon to Cursor** — pick one:
   - **JSON:** Paste the `mcpServers` block from **`axon serve`** or **dashboard → MCP config → Copy JSON snippet** into `mcp.json`.
   - **Deeplink:** Click **Open in Cursor** (or paste the printed `cursor://…` URL from the terminal/dashboard). Confirm the install prompt in Cursor. Same TLS and **API-key-in-URL** caveats as above.

   **Important:** The dashboard builds the MCP URL from **the address in your browser’s address bar**. If you open the dashboard at `https://203.0.113.10:8443/`, the snippet and deeplink use that host/port—**which is what you usually want**. If you use an SSH tunnel or different hostname locally, you may need to **edit the `url`** (or regenerate via [Cursor’s install-link helper](https://cursor.com/docs/context/mcp/install-links)) so it matches what **Cursor on the Mac** should call.

5. **Reload Cursor** (if you used the JSON file and Cursor did not pick it up automatically)  
   Restart Cursor or reload the window so MCP settings are picked up (wording in the UI may vary by version).

6. **Verify**  
   In Cursor, confirm the **Axon** MCP server is listed and tools (e.g. `shell`, `read_file`) appear. Run a harmless command (e.g. `echo ok`) before anything destructive.

### Merging with other MCP servers

The top-level object must have **one** `"mcpServers"` key whose value is an object. Each server has its own name (key). **Do not paste a second whole file on top of the first**—merge keys:

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

To add more servers, keep **one** `"mcpServers"` object and add **more keys** beside `"axon"`—each key is a server name, and the value must match **that** vendor’s documented schema (do not copy-paste a second whole file over this one). Invalid JSON (trailing commas, two separate top-level `{ ... }{ ... }` blobs) will prevent Cursor from loading **any** MCP server from that file.

### Web dashboard (same TLS + API key)

Axon serves a small **dashboard** at the **origin** of the agent, e.g. `https://YOUR_REMOTE_HOST:8443/` (not the `/mcp` path). After you accept the certificate in the browser, enter the **same API key** to open the WebSocket stream.

The **MCP config (Cursor)** panel matches what **`axon serve`** prints: **copyable JSON** for `mcp.json`, a **`cursor://…` one-click install link** (with an explicit warning that it contains the key), **Open in Cursor**, **Copy install link**, and a link to **[Cursor’s MCP install links / generator](https://cursor.com/docs/context/mcp/install-links)** for transparency.

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

Paste it into `~/.cursor/mcp.json` on your Mac and reload Cursor.

> **Note:** The `trycloudflare.com` URL changes on every restart. You will need to update `mcp.json` and reload Cursor each time. For a permanent URL, use a named tunnel (below).

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

## Local development (simulating remote assistance)

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

## Security notes

- Treat the API key like a password; rotate with `axon keygen` if leaked.
- Prefer VPN or SSH tunnel if you cannot expose a port safely.
- Review and extend `denylist.txt` for your environment.

## License

MIT
