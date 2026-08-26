package main

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// 1. Declare all styles globally.
// This separates the UI design from your printing logic.
var (
	cyanStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("36"))
	greenStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("32"))
	yellowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("33"))
	dimStyle    = lipgloss.NewStyle().Faint(true)
	boldStyle   = lipgloss.NewStyle().Bold(true)

	// bannerStyle replaces the manual "╔════" string drawing.
	bannerStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("36")).
			Padding(0, 1).
			Margin(1, 0, 1, 2) // Replicates the 2-space left margin and empty lines.

	// indentStyle replaces the splitLines function and manual "│" prefixing.
	indentStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true). // Left border only
			BorderForeground(lipgloss.Color("240")).                    // Dim grey border
			PaddingLeft(2).
			MarginLeft(2)
)

func printBanner(version string) {
	// Replicates the original layout: bold version, normal text for the rest.
	leftText := boldStyle.Render(fmt.Sprintf("Axon %-5s", version))
	rightText := "— Secure Remote MCP Agent"

	fmt.Println(bannerStyle.Render(leftText + "  " + rightText))
}

// hyperlink emits an OSC 8 terminal hyperlink (iTerm2, Kitty, WezTerm, etc.).
// Falls back to plain text on terminals that don't support it.
func hyperlink(url, text string) string {
	return "\033]8;;" + url + "\033\\" + text + "\033]8;;\033\\"
}

func printServerInfo(listenLine, dashboardURL, apiKey string, extra ...string) {
	dashWithKey := dashboardURL + "#" + apiKey

	fmt.Printf("  %s  %s\n", boldStyle.Render("Listening:"), cyanStyle.Render(listenLine))
	fmt.Printf("  %s   %s\n", boldStyle.Render("Dashboard:"), hyperlink(dashWithKey, cyanStyle.Render(dashWithKey)))
	fmt.Printf("  %s    %s\n", boldStyle.Render("API key:"), greenStyle.Render(apiKey))

	for _, line := range extra {
		fmt.Println(line)
	}
	fmt.Println()
}

func printSectionHeader(title string) {
	headerPrefix := boldStyle.Render("  ── " + title)
	lineTrailing := dimStyle.Render(" ────────────────────────────────────")

	fmt.Println(headerPrefix + lineTrailing)
	fmt.Println()
}

// printCursorSection prints Cursor-specific MCP setup instructions.
func printCursorSection(mcpJSON, deeplink string) {
	printSectionHeader("Cursor")

	fmt.Printf("  %s\n", dimStyle.Render("Add to .cursor/mcp.json:"))
	indentBlock(mcpJSON)
	fmt.Println()

	if deeplink != "" {
		warning := yellowStyle.Render("(contains API key — do not share)")
		fmt.Printf("  %s\n", dimStyle.Render("One-click install "+warning+":"))
		fmt.Printf("  %s\n", greenStyle.Render(deeplink))
		fmt.Println()
	}
}

// printClaudeSection prints Claude Code-specific MCP setup instructions.
// mcpURL is the final MCP endpoint; apiKey is the raw token.
func printClaudeSection(mcpURL, apiKey, settingsJSON string) {
	printSectionHeader("Claude Code")

	fmt.Printf("  %s\n", dimStyle.Render("Set these environment variables (add to ~/.zshrc or ~/.bashrc):"))

	urlLine := greenStyle.Render(`  export AXON_MCP_URL="`) + cyanStyle.Render(mcpURL) + greenStyle.Render(`"`)
	tokenLine := greenStyle.Render(`  export AXON_MCP_TOKEN="`) + cyanStyle.Render(apiKey) + greenStyle.Render(`"`)

	fmt.Println(urlLine)
	fmt.Println(tokenLine)
	fmt.Println()

	fmt.Printf("  %s\n", dimStyle.Render("Then add to .claude/settings.json in your project (key never hard-coded):"))
	indentBlock(settingsJSON)
	fmt.Println()

	fmt.Printf("  %s\n", dimStyle.Render("Or register via CLI (one-time):"))

	cliLine1 := greenStyle.Render(`  claude mcp add axon --transport http "`) + cyanStyle.Render(mcpURL) + greenStyle.Render(`" \`)
	cliLine2 := greenStyle.Render(`    -H "Authorization: Bearer `) + cyanStyle.Render(apiKey) + greenStyle.Render(`"`)

	fmt.Println(cliLine1)
	fmt.Println(cliLine2)
	fmt.Println()
}

func indentBlock(s string) {
	// Lipgloss naturally handles the line splitting and left-border prefixing here.
	fmt.Println(indentStyle.Render(s))
}
