# Security

This document has two parts. The [security notes](#security-notes) tell you how to run Axon safely. The [security review](#security-review) lists the findings of a full review of the repository.

For how the parts of Axon work together, see [ARCHITECTURE.md](ARCHITECTURE.md).

## Security notes

- Treat the API key like a password. Rotate it with `axon keygen` if it leaks. It opens `/mcp` and the dashboard feed. There is no second key.
- The agent can read `config.yaml`, so it can read the key. It already holds the key. It can also overwrite the file, and the change applies at the next start. Run Axon as a dedicated user and keep the config out of the agent's reach.
- Run Axon as an unprivileged user that has no passwordless `sudo`. The denylist and the `sudo` check are guardrails, not a sandbox. A client can bypass them, for example by writing a script that calls `sudo` and running the script. The `sudo` check catches a path such as `/usr/bin/sudo`, but it does not read scripts.
- The file tools can reach every path that the Axon user can reach.
- Axon does not pass or hide secrets. A command that asks for a password stops at `input_required`, and the agent cancels it. Give commands their credentials before they run: SSH keys, a Git credential helper, or environment variables. A password in a command line or in `send_input` shows in the audit log and in the dashboard.
- The dashboard is read-only and uses the API key. It keeps the key in `sessionStorage` for the life of the tab. Close the tab when you finish. In tunnel mode, Cloudflare ends the TLS connection, so it can see the `Sec-WebSocket-Protocol` header that carries the key. The key is not in the URL.
- In tunnel mode, every request comes from `127.0.0.1`. `ip_allowlist` and the per-IP rate limit cannot tell clients apart.
- Prefer a VPN or an SSH tunnel if you cannot expose a port safely.
- Review and extend `~/.axon/denylist.txt` for your environment.

## Security review

Date: 2026-10-25. Scope: the whole repository at commit `7ecf0b1` (after the secret store, the two keys and the operator key hash). This includes the Go server, the React dashboard, the build and release files, and the install scripts. Version 2.0.0 (2026-10-31) removed the secret store, the operator key and the dashboard controls. Sections 1 to 3 and the priority list show the state after that change. [Section 3](#3-what-version-200-removed) records why.

### Summary

Axon gives an AI agent a shell on a remote machine. That is a powerful tool, and it has one main boundary: the API key. Anyone with the key has the full power of the Axon user.

The code is careful in many small places. The main risks come from the design, not from coding mistakes:

1. The agent runs as the Axon user. It can read the API key, erase the audit log and change the denylist.
2. The denylist and the `sudo` check stop honest mistakes only.
3. The server has no request limits and no timeouts. It listens on all interfaces by default.
4. The default TLS setup leads users to turn off certificate checks.

Version 2.0.0 made the design smaller. There is one key. The dashboard only watches. Axon no longer takes passwords from a person, so it has no secret store to defend. [Section 3](#3-what-version-200-removed) gives the reasons.

### How I checked

| Mark | Meaning |
|---|---|
| **Probe** | I ran it against a throwaway server and saw the result. |
| **Code** | I read the code. I did not run it. |
| **Not run** | I could not check it here. |

Not run: `govulncheck` (not installed), the Windows code at run time, and Linux-only behaviour such as `ptrace_scope`.

### 1. Threat model

| Item | Description |
|---|---|
| Assets | The host account, the API key, the audit trail |
| Actors | AI agent (semi-trusted), operator, remote user, network attacker, Cloudflare |
| Trust boundary | The API key. With it, the caller can do anything the Axon user can do. The same key opens the read-only dashboard feed. |
| Out of scope | A compromised host, a malicious Axon user, a malicious `cloudflared` binary |

### 2. Findings

Severity: **High** can lead to full compromise or loss of audit trust. **Medium** weakens a protection or allows denial of service. **Low** is hardening.

#### High

| # | Finding | Evidence | Fix |
|---|---|---|---|
| H1 | The agent can read the API key and the TLS key with `read_file`. | Probe: the `api_key` line was visible. | Run Axon as a dedicated user. Keep keys out of the agent's reach, for example a root-owned config read at start. |
| H2 | The agent can erase or edit its own audit log. | Probe: `write_file` on `audit.log` left 0 lines. | Send audit lines to a sink the Axon user cannot edit (journald, syslog, an append-only file owned by another user). |
| H3 | The agent can empty the denylist and change `config.yaml` (`read_only`, `ip_allowlist`). The change applies at the next start. | Probe: `write_file` on `denylist.txt` returned `ok`. | Same as H1. Make the config read-only for the Axon user. |
| H4 | The denylist and the `sudo` check are bypassable. | Probe: `echo mk''fs` ran. `/usr/bin/sudo` was a bug and is fixed. A script that calls `sudo` still passes. The check ignores quoted text and comments (`maskInertText`), so the shell can still run text that it hides: a runner that is not in `runnerWords`, `s"u"do`, `x=sudo; $x id`, an encoded word (`base64 -d | sh`), or an alias. `TestIsPrivilegedCmd_KnownLimits` lists these. Code: patterns are plain substrings. | Treat both as guardrails. Use OS controls: no sudo rights for the Axon user, containers or a sandbox. |
| H5 | The default TLS setup leads users to turn off verification. The certificate is valid only for `localhost` and `127.0.0.1`. The printed fingerprint is 64 bits. | Code: `tls.go` lines 43 and 101. | Add a flag for host names (SANs). Print the full SHA-256. Document how clients trust the certificate. |

#### Medium

| # | Finding | Evidence | Fix |
|---|---|---|---|
| M1 | No server timeouts. A client can hold connections open before it sends a key. The default listen address is `0.0.0.0`. | Code: `http.Server{Handler: ...}` only. | Set `ReadHeaderTimeout`, `ReadTimeout`, `IdleTimeout`. |
| M2 | No size limit on `POST /mcp`. | Probe: a 30 MB body returned HTTP 200. | Wrap the body with `http.MaxBytesReader`. |
| M3 | No limit on concurrent commands. The rate limit is 20 requests per second per IP, and a command can run 5 minutes. | Code. | Cap running processes. Return an error at the cap. |
| M4 | In tunnel mode every request comes from `127.0.0.1`. The IP allowlist does nothing and all clients share one rate limit. | Code. | Trust `Cf-Connecting-Ip` only when the peer is loopback and a tunnel is active. |
| M5 | Failed logins leave no trace. | Probe: 0 audit lines after a wrong key. | Audit failures. Add a short delay or a lockout per IP. |
| M6 | One static key serves the agent and the dashboard. This is by design since 2.0.0: the dashboard cannot change anything, and a second key did not stop the agent from doing anything it can do with the first. A key cannot rotate without a restart (`axon keygen` does not change a running server). | Tests: `/mcp` and `/ws` accept only the key. The removed routes (`/api/input`, `/api/cancel`, `/api/agent-config`) return no data. | Reload the key on `SIGHUP`. |
| M7 | The key appears in places other users and logs can see: the arguments of `open` or `xdg-open` (`ps`), and the terminal banner. **Fixed in part:** the WebSocket key moved from the `?key=` query to the `Sec-WebSocket-Protocol` header in 2.0.0, so it no longer shows in URLs, proxy access logs or browser history. `axon serve` opens the browser without a key. The printed Dashboard link holds the key in the `#` fragment, which a browser never sends to a server. Still open: the terminal banner prints the key and the link, and a proxy that ends TLS (Cloudflare) can see the header. | Tests: the key in the URL is refused. The server does not echo the key. The printed link keeps the key after the `#`. | Let `axon serve` skip the key in the banner on request. |
| M8 | The WebSocket accepts any `Origin`. | Probe: a foreign Origin got `101`. | Check `Origin` against the host. |
| M9 | The dashboard has no security headers (CSP, frame-ancestors, nosniff). The page holds the API key, which runs commands. A script injected into the page could use it. | Code: plain `http.FileServer`. | Add a strict CSP and `X-Frame-Options: DENY`. |
| M10 | `GET /` and `GET /health` have no guard and no rate limit. | Probe: both returned 200. | Apply the rate limit. Keep `/health` minimal. |
| M11 | `curl \| bash` install with no checksum or signature. The script writes to a fixed `/tmp` path. The Windows script uses a URL that GitHub does not serve (`releases/download/latest/...`). | Code: `install.sh`, `install.ps1`. | Use `mktemp`. Publish checksums and signatures. Fix the Windows URL. |
| M12 | The release workflow pins actions by tag, not by commit. It has `contents: write`. It runs no tests and no vulnerability scan. | Code: `release.yml`. | Pin by commit SHA. Run tests and `govulncheck` before the release step. |
| M13 | The built dashboard is committed as minified files. A reviewer cannot compare it with the source. | Code: `internal/dashboard/dist/`. | Build in CI and check that it matches the commit. |
| M14 | A missing denylist file means no denylist. `audit_log: ""` turns off auditing. Neither prints a warning. | Code: `denylist.go:23`, `logger.go:28`. | Fail to start, or print a clear warning. |

#### Low

| # | Finding | Fix |
|---|---|---|
| L1 | Audit lines hold the full command. A command with a password inside is logged in clear text. Axon has no secret store since 2.0.0, so nothing hides it. | Redact common patterns (`-p...`, `Bearer ...`, `password=`) before logging and before the dashboard event. |
| L2 | `read_file` loads whole files. `glob` has no limit. `grep` drops all matches of a file with a line over 64 KB. | Add size limits. Raise the scanner buffer. |
| L3 | The hub holds a lock while it writes to each client. One slow client blocks all. The WebSocket has no read limit. | Write per client with a queue. Call `SetReadLimit`. |
| L4 | No `Origin` or `Host` check on `/mcp`. The MCP specification asks for it. | Check both. |
| L5 | `GET /mcp` returns 404, not 405. | Return 405. |
| L6 | The certificate is valid for 10 years. There is no rotation. | Use a shorter life and a `rotate` command. |
| L7 | A stray file is tracked: `.pnpm-store/v10/projects/...`. | Remove it and add the folder to `.gitignore`. |
| L8 | The audit write ignores errors. A failed write does not stop the action. | Decide whether auditing must fail closed. |
| L9 | Windows: `rootcheck` does not detect an administrator. | Check the token. |

#### Dependencies

| Area | Result |
|---|---|
| Dashboard, runtime | `pnpm audit --prod`: no known vulnerabilities. Only React and React DOM ship. |
| Dashboard, build | `pnpm audit`: 50 advisories (20 high) in `vite`, `vitest`, `undici`, `postcss`, `picomatch`, `nanoid`. They are build-time tools. Do not expose `pnpm dev` to a network. Update `vite`. |
| Go | Many indirect modules are from 2021 to 2022 (for example `go-ole`, `x/exp`, `plan9stats`). `govulncheck` is not installed, so I did not run it. Run it in CI. |

### 3. What version 2.0.0 removed

Versions 1.x let the operator type a password in the dashboard. A secret store hid that text from the agent. A second key, the operator key, guarded the dashboard routes. The dashboard also had a cancel button. Version 2.0.0 removed all of it.

| Removed | Reason |
|---|---|
| The secret store and `Redact` | It was best effort. It replaced exact text only. A hash, base64 or terminal codes passed through. A short secret such as `root` damaged ordinary output. A hostile agent could read the process with `ptrace` or `/proc`. The agent can finish most tasks without a password prompt, so the cost was not worth the protection. |
| `POST /api/input` | It existed to feed the secret store. The agent answers confirmation prompts with `send_input` and needs no person. |
| `POST /api/cancel` and the Cancel button | A person cannot react at the speed of the agent. The real stop is to close the connection or press `Ctrl+C` on `axon serve`. The agent keeps `cancel_command` to clean up its own stalled commands. |
| `GET /api/agent-config` | The dashboard already holds the key. |
| The operator key, its hash, `axon keygen -operator` | After the routes above were gone, the second key guarded only the read-only feed. It did not limit the agent. One key is easier to keep safe. |

What stays true:

- The agent holds the key, so it can read the dashboard feed. The feed shows only what the agent sent and received.
- A command that needs a password cannot run through Axon. This is the intended result. Use SSH keys, a credential helper or environment variables.
- The dashboard sends nothing to Axon after the handshake. There is no write route to attack.

What got worse, and how it is handled:

- A leak of the dashboard key now leaks the key that runs commands. The key moved out of the WebSocket URL and into the `Sec-WebSocket-Protocol` header for this reason (M7).
- The dashboard keeps the API key in `sessionStorage`. A script injected into the page could use it. Add security headers to reduce this risk (M9).

### 4. A bug I found while testing

| Bug | Result |
|---|---|
| `go test -race` failed because of a data race on `c.ProcessState`. | Fixed. The whole suite passes with `-race`. |

### 5. What is good

- API keys are 256 bits from `crypto/rand`. The comparison runs in constant time.
- Cross-site request forgery is not possible. The API uses a `Bearer` header, not cookies, and sets no CORS headers.
- Regular expressions use Go's RE2, so `grep` has no ReDoS risk.
- Config files are mode `0600` and the folder is `0700` (probe). Axon refuses to run as root on Unix.
- Dev and tunnel modes bind to `127.0.0.1`.
- The dashboard uses no `innerHTML`, `dangerouslySetInnerHTML`, `eval` or `localStorage`. React escapes all output. The demo code is not in the shipped bundle.
- A child process dies when its command times out. I tested `sh -c "sleep & sleep"` and no process stayed.

### 6. Priority list

1. Run Axon as a dedicated user without `sudo` rights. Protect the config and keys from that user (H1, H3).
2. Send the audit log to a sink the Axon user cannot edit (H2).
3. Add server timeouts, a body limit and a process cap (M1, M2, M3).
4. Fix TLS: host names in the certificate, a full fingerprint, a short guide for clients (H5).
5. Reload the key without a restart (M6). The WebSocket key is out of the URL (M7, done in 2.0.0).
6. Add security headers and an `Origin` check (M8, M9).
7. Harden the install and release steps (M11, M12, M13). Run `govulncheck` in CI.
