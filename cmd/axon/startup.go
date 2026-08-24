package main

import "fmt"

// ANSI helpers — terminal colors, no external dependency.
const (
	reset  = "\033[0m"
	bold   = "\033[1m"
	dim    = "\033[2m"
	cyan   = "\033[36m"
	green  = "\033[32m"
	yellow = "\033[33m"
	blue   = "\033[34m"
	red    = "\033[31m"
)

func b(s string) string  { return bold + s + reset }
func c(s string) string  { return cyan + s + reset }
func g(s string) string  { return green + s + reset }
func d(s string) string  { return dim + s + reset }

func printBanner(version string) {
	fmt.Println()
	fmt.Println(bold + cyan + "  ╔═══════════════════════════════════════╗" + reset)
	fmt.Printf( bold+cyan+"  ║"+reset+"  "+bold+"Axon %-5s"+reset+"  — Secure Remote MCP Agent  "+bold+cyan+"║"+reset+"\n", version)
	fmt.Println(bold + cyan + "  ╚═══════════════════════════════════════╝" + reset)
	fmt.Println()
}

// hyperlink emits an OSC 8 terminal hyperlink (iTerm2, Kitty, WezTerm, etc.).
// Falls back to plain text on terminals that don't support it.
func hyperlink(url, text string) string {
	return "\033]8;;" + url + "\033\\" + text + "\033]8;;\033\\"
}

func printServerInfo(listenLine, dashboardURL, apiKey string, extra ...string) {
	dashWithKey := dashboardURL + "#" + apiKey
	fmt.Printf("  %s  %s\n", b("Listening:"), c(listenLine))
	fmt.Printf("  %s   %s\n", b("Dashboard:"), hyperlink(dashWithKey, cyan+dashWithKey+reset))
	fmt.Printf("  %s    %s\n", b("API key:"), apiKey)
	for _, line := range extra {
		fmt.Println(line)
	}
	fmt.Println()
}

func printSectionHeader(title string) {
	fmt.Println(bold + "  ── " + title + " " + dim + "────────────────────────────────────" + reset)
	fmt.Println()
}

// printCursorSection prints Cursor-specific MCP setup instructions.
func printCursorSection(mcpJSON, deeplink string) {
	printSectionHeader("Cursor")
	fmt.Printf("  %s\n", d("Add to .cursor/mcp.json:"))
	indentBlock(mcpJSON)
	fmt.Println()
	if deeplink != "" {
		fmt.Printf("  %s\n", d("One-click install "+yellow+"(contains API key — do not share)"+reset+":"))
		fmt.Printf("  %s\n", g(deeplink))
		fmt.Println()
	}
}

// printClaudeSection prints Claude Code-specific MCP setup instructions.
// mcpURL is the final MCP endpoint; apiKey is the raw token.
func printClaudeSection(mcpURL, apiKey, settingsJSON string) {
	printSectionHeader("Claude Code")

	fmt.Printf("  %s\n", d("Set these environment variables (add to ~/.zshrc or ~/.bashrc):"))
	fmt.Printf("  %s\n", green+`  export AXON_MCP_URL="`+cyan+mcpURL+reset+green+`"`+reset)
	fmt.Printf("  %s\n", green+`  export AXON_MCP_TOKEN="`+cyan+apiKey+reset+green+`"`+reset)
	fmt.Println()

	fmt.Printf("  %s\n", d("Then add to .claude/settings.json in your project (key never hard-coded):"))
	indentBlock(settingsJSON)
	fmt.Println()

	fmt.Printf("  %s\n", d("Or register via CLI (one-time):"))
	fmt.Printf("  %s\n", green+`  claude mcp add axon --transport http "`+cyan+mcpURL+reset+green+`" \`+reset)
	fmt.Printf("  %s\n", green+`    -H "Authorization: Bearer `+cyan+apiKey+reset+green+`"`+reset)
	fmt.Println()
}

func indentBlock(s string) {
	fmt.Println(dim + "  ┌" + reset)
	for _, line := range splitLines(s) {
		fmt.Printf(dim+"  │"+reset+"  %s\n", line)
	}
	fmt.Println(dim + "  └" + reset)
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}
