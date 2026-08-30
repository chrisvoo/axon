import type { AxonEvent } from '../types'

export type Scenario = {
  label: string
  kind: 'normal' | 'input_required'
  build: () => AxonEvent[]
}

function uid() {
  return Math.random().toString(36).slice(2, 10)
}

// Exercises ANSI color codes, bold, and reset sequences
const ANSI_GIT_LOG = [
  '\x1b[33mcommit a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0\x1b[0m \x1b[1m(HEAD -> main, origin/main)\x1b[0m',
  '\x1b[34mAuthor:\x1b[0m Jane Smith <jane@example.com>',
  '\x1b[34mDate:\x1b[0m   Thu Aug 29 14:22:11 2026 +0000',
  '',
  '    feat: add distributed rate limiter with Redis backing',
  '',
  '    Implements a sliding-window token bucket via EVAL atomicity.',
  '    Fallback to in-process limiter when Redis is unreachable.',
  '',
  '\x1b[33mcommit 9f0e1d2c3b4a5968778899aabbccddeeff001122\x1b[0m',
  '\x1b[34mAuthor:\x1b[0m Bob Jones <bob@example.com>',
  '\x1b[34mDate:\x1b[0m   Wed Aug 28 09:14:55 2026 +0000',
  '',
  '    fix: prevent double-close of done channel in worker pool',
  '',
  '\x1b[33mcommit 7c8b9a0f1e2d3c4b5a6978899aabbccddeeff\x1b[0m',
  '\x1b[34mAuthor:\x1b[0m Jane Smith <jane@example.com>',
  '\x1b[34mDate:\x1b[0m   Tue Aug 27 18:05:30 2026 +0000',
  '',
  '    refactor: extract retry logic into backoff package',
].join('\n')

// Long output to exercise line-truncation and "Show more" button
const LONG_SHARD_OUTPUT = Array.from({ length: 28 }, (_, i) => {
  const shard = String(i + 1).padStart(2, '0')
  const bar = '█'.repeat((i % 8) + 2)
  const latency = (15 + Math.sin(i) * 10).toFixed(1)
  const status = i % 7 === 0 ? '\x1b[33m[WARN]\x1b[0m' : '\x1b[32m[ OK ]\x1b[0m'
  return `${status} shard ${shard}/28 ${bar} latency=${latency}ms  processed=\x1b[1m${(i + 1) * 1024}\x1b[0m rows`
}).join('\n') + '\n\n\x1b[32mAll shards processed.\x1b[0m  total_rows=28672  elapsed=3.8s'

// JSON output to exercise the tryPrettyJson path in ansi.ts
const JSON_OUTPUT = JSON.stringify({
  status: 'ok',
  server: { version: '0.1.0', uptime_s: 3721, pid: 12345 },
  connections: { active: 4, peak: 12, rejected: 0 },
  tools: ['shell_exec', 'read_file', 'write_file', 'search'],
  flags: { tls: true, tunnel: false, dev_mode: false },
})

// Error output with ANSI red, exercises is_error path
const TEST_FAILURE_OUTPUT = [
  '--- \x1b[31mFAIL\x1b[0m\tgithub.com/example/axon/internal/security (0.38s)',
  '    \x1b[31m--- FAIL: TestAllowlistReload\x1b[0m (0.10s)',
  '        allowlist_test.go:92: unexpected entry "192.168.99.0/24" in parsed list',
  '        allowlist_test.go:93: want 3 entries, got 4',
  '    \x1b[31m--- FAIL: TestRateLimitBurst\x1b[0m (0.02s)',
  '        ratelimit_test.go:44: burst exceeded: got 6 allowed, want ≤5',
  '\x1b[31mFAIL\x1b[0m',
  'exit status 1',
].join('\n')

// sudo-style interactive prompt to exercise input_required + InputPrompt dialog
const SUDO_LAST_OUTPUT = '\x1b[1m[sudo]\x1b[0m password for ubuntu: '

export const SCENARIOS: Scenario[] = [
  {
    label: 'shell_exec — ANSI git log (color + bold)',
    kind: 'normal',
    build: () => {
      const callId = uid()
      return [
        {
          type: 'tool_called',
          data: { call_id: callId, tool: 'shell_exec', detail: 'git log --color=always -10', remote_ip: '127.0.0.1' },
        },
        {
          type: 'tool_result',
          data: {
            call_id: callId,
            tool: 'shell_exec',
            ok: true,
            duration_ms: 38,
            output_preview: ANSI_GIT_LOG,
            shell_status: 'exited',
            exit_code: 0,
          },
        },
      ]
    },
  },
  {
    label: 'shell_exec — long output (truncation + Show more)',
    kind: 'normal',
    build: () => {
      const callId = uid()
      return [
        {
          type: 'tool_called',
          data: { call_id: callId, tool: 'shell_exec', detail: 'process-shards --all --verbose', remote_ip: '10.0.0.7' },
        },
        {
          type: 'tool_result',
          data: {
            call_id: callId,
            tool: 'shell_exec',
            ok: true,
            duration_ms: 3821,
            output_preview: LONG_SHARD_OUTPUT,
            shell_status: 'exited',
            exit_code: 0,
          },
        },
      ]
    },
  },
  {
    label: 'read_file — JSON pretty-print path',
    kind: 'normal',
    build: () => {
      const callId = uid()
      return [
        {
          type: 'tool_called',
          data: { call_id: callId, tool: 'read_file', detail: '/var/axon/status.json', remote_ip: '172.16.0.3' },
        },
        {
          type: 'tool_result',
          data: {
            call_id: callId,
            tool: 'read_file',
            ok: true,
            duration_ms: 5,
            output_preview: JSON_OUTPUT,
          },
        },
      ]
    },
  },
  {
    label: 'shell_exec — error / non-zero exit',
    kind: 'normal',
    build: () => {
      const callId = uid()
      return [
        {
          type: 'tool_called',
          data: { call_id: callId, tool: 'shell_exec', detail: 'go test ./internal/security/...', remote_ip: '127.0.0.1' },
        },
        {
          type: 'tool_result',
          data: {
            call_id: callId,
            tool: 'shell_exec',
            ok: false,
            is_error: true,
            duration_ms: 1247,
            output_preview: TEST_FAILURE_OUTPUT,
            shell_status: 'exited',
            exit_code: 1,
          },
        },
      ]
    },
  },
  {
    label: 'shell_exec — input_required (sudo password)',
    kind: 'input_required',
    build: () => {
      const callId = uid()
      return [
        {
          type: 'tool_called',
          data: { call_id: callId, tool: 'shell_exec', detail: 'sudo apt-get upgrade -y', remote_ip: '127.0.0.1' },
        },
        {
          type: 'input_required',
          data: {
            call_id: callId,
            tool: 'shell_exec',
            process_id: `demo-${callId}`,
            last_output: SUDO_LAST_OUTPUT,
            hint: 'sudo is asking for the system password to proceed with the upgrade',
          },
        },
      ]
    },
  },
  {
    label: 'privileged_commands modal',
    kind: 'normal',
    build: () => [
      {
        type: 'privileged_commands',
        data: {
          commands: [
            'rm -rf /var/log/nginx/old-*.gz',
            'dd if=/dev/zero of=/dev/sdb bs=4M count=256',
            'chmod 777 /etc/sudoers',
          ],
        },
      },
    ],
  },
]
