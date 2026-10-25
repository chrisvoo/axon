# Security review

Date: 2026-10-25. Scope: the whole repository at the working tree state after the secret-redaction change. This includes the Go server, the React dashboard, the build and release files, and the install scripts.

## Summary

Axon gives an AI agent a shell on a remote machine. That is a powerful tool, and it has one main boundary: the API key. Anyone with the key has the full power of the Axon user.

The code is careful in many small places. The main risks come from the design, not from coding mistakes:

1. The agent runs as the Axon user. It can read the API key, erase the audit log and change the denylist.
2. The denylist and the `sudo` check stop honest mistakes only.
3. The server has no request limits and no timeouts. It listens on all interfaces by default.
4. The default TLS setup leads users to turn off certificate checks.

The operator-secret path (a password typed in the dashboard) is protected on a best-effort basis. A shared secret store replaces the text in everything the agent can read, in the dashboard events and in the audit log. Tests cover this. The limits are listed in section 3. They matter: the protection stops accidental exposure, not a hostile agent.

## How I checked

| Mark | Meaning |
|---|---|
| **Probe** | I ran it against a throwaway server and saw the result. |
| **Code** | I read the code. I did not run it. |
| **Not run** | I could not check it here. |

Not run: `govulncheck` (not installed), the Windows code at run time, and Linux-only behaviour such as `ptrace_scope`.

## 1. Threat model

| Item | Description |
|---|---|
| Assets | The host account, the API key, secrets typed by the operator, the audit trail |
| Actors | AI agent (semi-trusted), operator, remote user, network attacker, Cloudflare |
| Trust boundary | The API key. After it, the caller can do anything the Axon user can do. |
| Out of scope | A compromised host, a malicious Axon user, a malicious `cloudflared` binary |

## 2. Findings

Severity: **High** can lead to full compromise or loss of audit trust. **Medium** weakens a protection or allows denial of service. **Low** is hardening.

### High

| # | Finding | Evidence | Fix |
|---|---|---|---|
| H1 | The agent can read the API key and the TLS key with `read_file`. | Probe: the `api_key` line was visible. | Run Axon as a dedicated user. Keep keys out of the agent's reach, for example a root-owned config read at start. |
| H2 | The agent can erase or edit its own audit log. | Probe: `write_file` on `audit.log` left 0 lines. | Send audit lines to a sink the Axon user cannot edit (journald, syslog, an append-only file owned by another user). |
| H3 | The agent can empty the denylist and change `config.yaml` (`read_only`, `ip_allowlist`). The change applies at the next start. | Probe: `write_file` on `denylist.txt` returned `ok`. | Same as H1. Make the config read-only for the Axon user. |
| H4 | The denylist and the `sudo` check are bypassable. | Probe: `echo mk''fs` ran. `/usr/bin/sudo` was a bug and is fixed. A script that calls `sudo` still passes. Code: patterns are plain substrings. | Treat both as guardrails. Use OS controls: no sudo rights for the Axon user, containers or a sandbox. |
| H5 | The default TLS setup leads users to turn off verification. The certificate is valid only for `localhost` and `127.0.0.1`. The printed fingerprint is 64 bits. | Code: `tls.go` lines 43 and 101. | Add a flag for host names (SANs). Print the full SHA-256. Document how clients trust the certificate. |

### Medium

| # | Finding | Evidence | Fix |
|---|---|---|---|
| M1 | No server timeouts. A client can hold connections open before it sends a key. The default listen address is `0.0.0.0`. | Code: `http.Server{Handler: ...}` only. | Set `ReadHeaderTimeout`, `ReadTimeout`, `IdleTimeout`. |
| M2 | No size limit on `POST /mcp`. | Probe: a 30 MB body returned HTTP 200. | Wrap the body with `http.MaxBytesReader`. |
| M3 | No limit on concurrent commands. The rate limit is 20 requests per second per IP, and a command can run 5 minutes. | Code. | Cap running processes. Return an error at the cap. |
| M4 | In tunnel mode every request comes from `127.0.0.1`. The IP allowlist does nothing and all clients share one rate limit. | Code. | Trust `Cf-Connecting-Ip` only when the peer is loopback and a tunnel is active. |
| M5 | Failed logins leave no trace. | Probe: 0 audit lines after a wrong key. | Audit failures. Add a short delay or a lockout per IP. |
| M6 | One static key served the agent and the dashboard. **Fixed in part:** there are now two keys. The operator key opens `/ws`, `/api/input`, `/api/cancel` and `/api/agent-config`. The API key opens `/mcp` only. Still open: a key cannot rotate without a restart (`axon keygen` does not change a running server). | Probe and tests: each route accepts only its own key. | Reload the keys on `SIGHUP`. |
| M7 | The key appears in places other users and logs can see: the `?key=` query of the WebSocket (proxy and Cloudflare logs), the arguments of `open` or `xdg-open` (`ps`), and the terminal banner. | Code: `hub.go:110`, `main.go:288`. | Send the key in the first WebSocket message. Open the browser without the key in the arguments. |
| M8 | The WebSocket accepts any `Origin`. | Probe: a foreign Origin got `101`. | Check `Origin` against the host. |
| M9 | The dashboard has no security headers (CSP, frame-ancestors, nosniff). The page holds the key and takes secrets. | Code: plain `http.FileServer`. | Add a strict CSP and `X-Frame-Options: DENY`. |
| M10 | `GET /` and `GET /health` have no guard and no rate limit. | Probe: both returned 200. | Apply the rate limit. Keep `/health` minimal. |
| M11 | `curl \| bash` install with no checksum or signature. The script writes to a fixed `/tmp` path. The Windows script uses a URL that GitHub does not serve (`releases/download/latest/...`). | Code: `install.sh`, `install.ps1`. | Use `mktemp`. Publish checksums and signatures. Fix the Windows URL. |
| M12 | The release workflow pins actions by tag, not by commit. It has `contents: write`. It runs no tests and no vulnerability scan. | Code: `release.yml`. | Pin by commit SHA. Run tests and `govulncheck` before the release step. |
| M13 | The built dashboard is committed as minified files. A reviewer cannot compare it with the source. | Code: `internal/dashboard/dist/`. | Build in CI and check that it matches the commit. |
| M14 | A missing denylist file means no denylist. `audit_log: ""` turns off auditing. Neither prints a warning. | Code: `denylist.go:23`, `logger.go:28`. | Fail to start, or print a clear warning. |

### Low

| # | Finding | Fix |
|---|---|---|
| L1 | Audit lines hold the full command. A command with a password inside is logged in clear text. | Redact common patterns (`-p...`, `Bearer ...`, `password=`) before logging and before the dashboard event. |
| L2 | `read_file` loads whole files. `glob` has no limit. `grep` drops all matches of a file with a line over 64 KB. | Add size limits. Raise the scanner buffer. |
| L3 | The hub holds a lock while it writes to each client. One slow client blocks all. The WebSocket has no read limit. | Write per client with a queue. Call `SetReadLimit`. |
| L4 | No `Origin` or `Host` check on `/mcp`. The MCP specification asks for it. | Check both. |
| L5 | `GET /mcp` returns 404, not 405. | Return 405. |
| L6 | The certificate is valid for 10 years. There is no rotation. | Use a shorter life and a `rotate` command. |
| L7 | A stray file is tracked: `.pnpm-store/v10/projects/...`. | Remove it and add the folder to `.gitignore`. |
| L8 | The audit write ignores errors. A failed write does not stop the action. | Decide whether auditing must fail closed. |
| L9 | Windows: `rootcheck` does not detect an administrator. | Check the token. |

### Dependencies

| Area | Result |
|---|---|
| Dashboard, runtime | `pnpm audit --prod`: no known vulnerabilities. Only React and React DOM ship. |
| Dashboard, build | `pnpm audit`: 50 advisories (20 high) in `vite`, `vitest`, `undici`, `postcss`, `picomatch`, `nanoid`. They are build-time tools. Do not expose `pnpm dev` to a network. Update `vite`. |
| Go | Many indirect modules are from 2021 to 2022 (for example `go-ole`, `x/exp`, `plan9stats`). `govulncheck` is not installed, so I did not run it. Run it in CI. |

## 3. The operator-secret path

What the code guarantees, with tests:

| Channel | Result |
|---|---|
| `shell` results, including later commands that print or read the text | `[redacted]` |
| `send_input` and the stored final result | `[redacted]` |
| `read_file`, `grep`, `glob` and error messages | `[redacted]` |
| Reply to the dashboard | `[redacted]` |
| Dashboard events (details and output previews) | `[redacted]`. The `dashboard_input` event has the process id, the IP and the `secret` flag, no text. |
| Audit log and server log | `[redacted]` in the detail. No input text. |
| Browser | The field is masked, `autocomplete="new-password"`. The text is cleared after sending. It is not stored. |

Two reviews found three weaknesses in the first versions. All are fixed and tested:

- **A store for each process** missed a program that writes the text to a file. Now one shared store covers all output.
- **Anyone with the API key could register secrets.** The agent holds the API key. It called the dashboard route with an unknown process id and the word `curl`. Axon stored the word, and the audit line of the agent's next command no longer showed `curl` (probe). Now two things stop this. Only the operator key reaches the dashboard routes. And only a live process registers a secret.
- **A secret could expire before its process ended.** The 10-minute limit ran from the moment of typing. A long command could outlive it. Now a secret lasts as long as its process, then 10 minutes.

How it works: one secret store on the process manager. It holds each typed line and its JSON-escaped form. A secret lasts as long as its process, then 10 minutes. The text is registered before it is written to the pty. The server treats a missing `secret` flag as `true`.

Why the store is shared: a review of the first version found that a store for each process missed a program that writes the text to a file. The agent could then read the file with `cat` or `read_file`. A test now covers this case.

Limits. The protection is best effort:

- **Exact text only.** A changed copy is not replaced: a hash, base64, or a terminal that redraws the line with control codes.
- **Over-redaction.** The store replaces the text in all output. A short secret such as `root` also replaces ordinary words. The operator chooses this with the `Secret` box.
- **Time limit.** The text is no longer replaced 10 minutes after its process ends.
- **Same-user access.** A hostile agent runs as the Axon user. It can try `ptrace`, `/proc` or other tools on the target process. On Linux, keep `kernel.yama.ptrace_scope` at 1 or higher and run Axon as a dedicated user. I did not test this on Linux.
- **Tunnel.** Cloudflare ends the TLS connection. The browser's request, with the password, passes through its network.
- **Plain responses.** With the `Secret` box off, the agent sees the text on purpose.
- **Prompts that show typing.** The echo is replaced, so the agent sees `[redacted]` where the text was.

## 4. Two bugs I found while testing

| Bug | Result |
|---|---|
| The dashboard never showed the input prompt. The `tool_result` event replaced the "waiting" status with "done". | Fixed. Test added. I confirmed it in a browser. |
| `go test -race` failed because of a data race on `c.ProcessState`. | Fixed. The whole suite passes with `-race`. |

## 5. What is good

- API keys are 256 bits from `crypto/rand`. The comparison runs in constant time.
- Cross-site request forgery is not possible. The API uses a `Bearer` header, not cookies, and sets no CORS headers.
- Regular expressions use Go's RE2, so `grep` has no ReDoS risk.
- Config files are mode `0600` and the folder is `0700` (probe). Axon refuses to run as root on Unix.
- Dev and tunnel modes bind to `127.0.0.1`.
- The dashboard uses no `innerHTML`, `dangerouslySetInnerHTML`, `eval` or `localStorage`. React escapes all output. The demo code is not in the shipped bundle.
- A child process dies when its command times out. I tested `sh -c "sleep & sleep"` and no process stayed.

## 6. Priority list

1. Run Axon as a dedicated user without `sudo` rights. Protect the config and keys from that user (H1, H3).
2. Send the audit log to a sink the Axon user cannot edit (H2).
3. Add server timeouts, a body limit and a process cap (M1, M2, M3).
4. Fix TLS: host names in the certificate, a full fingerprint, a short guide for clients (H5).
5. Reload keys without a restart. Stop putting the operator key in the WebSocket URL and in process arguments (M6, M7). The separate operator key is done.
6. Add security headers and an `Origin` check (M8, M9).
7. Harden the install and release steps (M11, M12, M13). Run `govulncheck` in CI.
