export type AnsiSpan = { text: string; fg?: string; bold?: boolean }

// One Dark-inspired palette, readable on dark backgrounds
const FG: Record<number, string> = {
  30: '#5c6370', 31: '#e06c75', 32: '#98c379', 33: '#e5c07b',
  34: '#61afef', 35: '#c678dd', 36: '#56b6c2', 37: '#abb2bf',
  90: '#636d83', 91: '#ff7b85', 92: '#b5e890', 93: '#ffd68a',
  94: '#81c3fd', 95: '#da8fff', 96: '#7be0eb', 97: '#ffffff',
}

function processLine(line: string): string {
  // \r means "go to start of line" — take the last segment written
  const parts = line.split('\r')
  return parts[parts.length - 1]
}

function tryPrettyJson(s: string): string {
  const t = s.trim()
  if (t.startsWith('{') || t.startsWith('[')) {
    try { return JSON.stringify(JSON.parse(t), null, 2) } catch { /* not JSON */ }
  }
  return s
}

export function parseAnsi(raw: string): AnsiSpan[] {
  const cleaned = tryPrettyJson(raw)
  const normalized = cleaned.split('\n').map(processLine).join('\n')

  const spans: AnsiSpan[] = []
  let fg: string | undefined
  let bold = false
  const re = /\x1b\[([0-9;]*)m|\x1b\[[0-9;]*[A-Za-z]/g
  let last = 0
  let m: RegExpExecArray | null

  while ((m = re.exec(normalized)) !== null) {
    if (m.index > last) spans.push({ text: normalized.slice(last, m.index), fg, bold })
    // Only process color codes (ending in 'm')
    if (m[0].endsWith('m')) {
      const codes = m[1] ? m[1].split(';').map(Number) : [0]
      for (const code of codes) {
        if (code === 0) { fg = undefined; bold = false }
        else if (code === 1) { bold = true }
        else if (code === 22) { bold = false }
        else if (FG[code]) { fg = FG[code] }
      }
    }
    last = m.index + m[0].length
  }

  if (last < normalized.length) spans.push({ text: normalized.slice(last), fg, bold })
  return spans.filter(s => s.text.length > 0)
}
