package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/charmbracelet/lipgloss"
	"github.com/chrisvoo/axon/internal/audit"
	"github.com/chrisvoo/axon/internal/config"
	"github.com/chrisvoo/axon/internal/events"
	"github.com/chrisvoo/axon/internal/hub"
	"github.com/chrisvoo/axon/internal/rootcheck"
	"github.com/chrisvoo/axon/internal/security"
	"github.com/chrisvoo/axon/internal/server"
	"github.com/chrisvoo/axon/internal/tunnel"
	"github.com/chrisvoo/axon/internal/update"
)

const version = "3.0.0" // semantic version: see "Versioning" in CLAUDE.md

// Specific styles for main.go (reuses cyanStyle, dimStyle, boldStyle from startup.go)
var (
	alertStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("31")).Bold(true)
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "init":
		if err := cmdInit(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "serve":
		if err := cmdServe(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "status":
		if err := cmdStatus(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "update":
		if err := cmdUpdate(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "version", "-v", "--version":
		fmt.Println(version)
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `axon %s — secure remote MCP agent

Commands:
  init            Create config, TLS certificate, and default denylist
  serve           Start HTTPS MCP server (makes a new API key at every start; the key is never saved)
    -dev          Plain HTTP, no TLS (local development only)
    -tunnel       Start a Cloudflare quick tunnel and print the public mcp.json snippet
                  (implies -dev; requires cloudflared in PATH)
    -tunnel-name  Named Cloudflare tunnel to run (requires -tunnel-url)
    -tunnel-url   Public URL of the named tunnel, e.g. https://axon.example.com
                  (requires -tunnel-name; implies -dev)
    -addr         Override listen address
    -port         Override listen port
    -claude       Print only the Claude Code setup (default)
    -cursor       Print only the Cursor setup; with -claude, print both
    -no-browser   Do not open the dashboard in a browser
  status          Show configuration paths and certificate fingerprint
  update          Pull the latest source and rebuild what is stale (binary, dashboard)
    -check        Only report what would change
    -force        Discard local changes and local commits (default: stop on conflicts)
    -repo         Path of the source checkout (default: found from the binary or the current directory)
  version         Print version

`, version)
}

func cmdInit() error {
	if err := rootcheck.EnsureNotRoot(); err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if _, err := os.Stat(cfg.CertFile); os.IsNotExist(err) {
		if err := security.GenerateSelfSignedCert(cfg.CertFile, cfg.KeyFile); err != nil {
			return fmt.Errorf("tls: %w", err)
		}
	} else if err != nil {
		return err
	}
	if err := config.WriteDefaultDenylist(cfg.DenylistPath()); err != nil {
		return err
	}
	if err := cfg.Save(); err != nil {
		return err
	}
	fmt.Println("Initialized Axon configuration.")
	fmt.Println("There is no API key to save: axon serve makes a new one at every start.")
	fp, err := security.CertFingerprint(cfg.CertFile)
	if err == nil {
		fmt.Printf("TLS fingerprint: %s\n", fp)
	}
	return nil
}

func cmdServe() error {
	if err := rootcheck.EnsureNotRoot(); err != nil {
		return err
	}
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	addr := fs.String("addr", "", "override listen address (default from config)")
	port := fs.Int("port", 0, "override listen port")
	noBrowser := fs.Bool("no-browser", false, "do not open the dashboard in a browser")
	dev := fs.Bool("dev", false, "development mode: plain HTTP, no TLS (localhost only)")
	useTunnel := fs.Bool("tunnel", false, "start a Cloudflare quick tunnel and print the public mcp.json snippet (implies -dev)")
	tunnelName := fs.String("tunnel-name", "", "named Cloudflare tunnel to run (requires -tunnel-url)")
	tunnelURL := fs.String("tunnel-url", "", "public URL of the named tunnel, e.g. https://axon.example.com (requires -tunnel-name)")
	claude := fs.Bool("claude", false, "print only the Claude Code setup (default when no client flag is set)")
	cursor := fs.Bool("cursor", false, "print only the Cursor setup (combine with -claude for both)")
	_ = fs.Parse(os.Args[2:])
	clients := clientSet{claude: *claude || !*cursor, cursor: *cursor}

	if (*tunnelName == "") != (*tunnelURL == "") {
		return fmt.Errorf("-tunnel-name and -tunnel-url must be used together")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	// The key lives in memory only. It dies with this process, so a restart is
	// the key rotation.
	key, err := genKey()
	if err != nil {
		return err
	}
	if *addr != "" {
		cfg.ListenAddr = *addr
	}
	if *port != 0 {
		cfg.ListenPort = *port
	}

	deny, err := security.LoadDenylist(cfg.DenylistPath())
	if err != nil {
		return err
	}
	log, err := audit.New(cfg.AuditLog)
	if err != nil {
		return err
	}

	bus := events.NewBus()
	h := hub.NewHub(bus, version)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	useNamedTunnel := *tunnelName != ""

	if *dev || *useTunnel || useNamedTunnel {
		// All tunnel/dev modes bind on loopback; cloudflared (or the user) provides the public endpoint.
		cfg.ListenAddr = "127.0.0.1"
		listenLine := fmt.Sprintf("http://127.0.0.1:%d/mcp", cfg.ListenPort)
		dashboardURL := fmt.Sprintf("http://127.0.0.1:%d/", cfg.ListenPort)

		printBanner(version)
		fmt.Printf("  %s  DEV MODE — plain HTTP, no TLS. Do not expose this port externally.\n\n", alertStyle.Render("⚠"))
		printServerInfo(listenLine, dashboardURL, key)

		switch {
		case useNamedTunnel:
			mcpURL := *tunnelURL + "/mcp"
			fmt.Printf("  %s Named Cloudflare Tunnel: %s\n\n", boldStyle.Render("☁"), cyanStyle.Render(*tunnelURL))
			printClients(clients, mcpURL, key, server.PrettyMCPConfigForURL("axon", mcpURL, key), true)
			fmt.Printf("  Starting named tunnel %q…\n\n", *tunnelName)
			go func() {
				if err := tunnel.StartNamedTunnel(ctx, *tunnelName); err != nil && ctx.Err() == nil {
					fmt.Fprintf(os.Stderr, "\ncloudflared error: %v\n", err)
				}
			}()

		case *useTunnel:
			localURL := fmt.Sprintf("http://127.0.0.1:%d", cfg.ListenPort)
			fmt.Printf("  Starting Cloudflare quick tunnel… %s\n\n", dimStyle.Render("(this may take a few seconds)"))
			go func() {
				err := tunnel.StartTrycloudflare(ctx, localURL, func(publicURL string) {
					mcpURL := publicURL + "/mcp"
					fmt.Printf("\n  %s Cloudflare Tunnel ready: %s\n\n", boldStyle.Render("☁"), cyanStyle.Render(publicURL))
					printClients(clients, mcpURL, key, server.PrettyMCPConfigForURL("axon", mcpURL, key), true)
				})
				if err != nil && ctx.Err() == nil {
					fmt.Fprintf(os.Stderr, "\ncloudflared error: %v\n", err)
				}
			}()

		default:
			mcpURL := fmt.Sprintf("http://%s:%d/mcp", cfg.ListenAddr, cfg.ListenPort)
			printClients(clients, mcpURL, key, server.PrettyMCPDevConfig(cfg.ListenAddr, cfg.ListenPort, key), false)
		}

		if !*noBrowser {
			// No key in the address: it would show in the process list. Click the
			// printed Dashboard link (it has the key) or paste the API key.
			go openBrowser(dashboardURL)
		}

		return server.Run(ctx, cfg, deny, log, server.Options{
			Version:      version,
			APIKey:       key,
			Events:       bus,
			Hub:          h,
			DevMode:      true,
			BehindTunnel: *useTunnel || useNamedTunnel,
		})
	}

	fp, err := security.CertFingerprint(cfg.CertFile)
	if err != nil {
		return err
	}
	host := cfg.ListenAddr
	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	mcpURL := server.MCPHTTPSURL(cfg.ListenAddr, cfg.ListenPort)
	dashboardURL := fmt.Sprintf("https://%s:%d/", host, cfg.ListenPort)
	listenLine := fmt.Sprintf("https://%s:%d/mcp", cfg.ListenAddr, cfg.ListenPort)

	printBanner(version)
	printServerInfo(listenLine, dashboardURL, key,
		fmt.Sprintf("  %s %s", boldStyle.Render("TLS fingerprint:"), dimStyle.Render(fp)),
	)

	printClients(clients, mcpURL, key, server.PrettyMCPConfig(cfg.ListenAddr, cfg.ListenPort, key), true)

	if !*noBrowser {
		go openBrowser(dashboardURL)
	}

	return server.Run(ctx, cfg, deny, log, server.Options{
		Version: version,
		APIKey:  key,
		Events:  bus,
		Hub:     h,
	})
}

// clientSet selects which MCP client setups `serve` prints.
type clientSet struct{ claude, cursor bool }

// printClients prints the setup of the selected clients. deeplink adds Cursor's one-click link.
func printClients(c clientSet, mcpURL, apiKey, cursorJSON string, deeplink bool) {
	if c.cursor {
		dl := ""
		if deeplink {
			dl, _ = server.CursorMCPInstallDeeplink("axon", mcpURL, apiKey)
		}
		printCursorSection(cursorJSON, dl)
	}
	if c.claude {
		printClaudeSection(mcpURL, apiKey, server.ClaudeCodeMCPConfig(mcpURL, apiKey))
	}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if cmd != nil {
		_ = cmd.Start()
	}
}

func cmdStatus() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	p, err := config.ConfigPath()
	if err != nil {
		return err
	}
	fmt.Printf("config dir:     %s\n", cfg.Dir())
	fmt.Printf("config file:    %s\n", p)
	fmt.Printf("certificate:    %s\n", cfg.CertFile)
	fp, err := security.CertFingerprint(cfg.CertFile)
	if err != nil {
		fmt.Printf("fingerprint:    (unavailable: %v)\n", err)
	} else {
		fmt.Printf("fingerprint:    %s\n", fp)
	}
	return nil
}

func cmdUpdate() error {
	if err := rootcheck.EnsureNotRoot(); err != nil {
		return err
	}
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	check := fs.Bool("check", false, "only report what would change")
	force := fs.Bool("force", false, "discard local changes and local commits")
	repo := fs.String("repo", "", "path of the source checkout")
	if err := fs.Parse(os.Args[2:]); err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return err
	}
	if *repo == "" {
		if *repo, err = update.FindRepo(filepath.Dir(exe)); err != nil {
			cwd, _ := os.Getwd()
			if *repo, err = update.FindRepo(cwd); err != nil {
				return err
			}
		}
	}
	return update.Run(update.Options{
		Repo:     *repo,
		Exe:      exe,
		BuiltRev: update.BuiltRevision(),
		Check:    *check,
		Force:    *force,
		Out:      os.Stdout,
	})
}

// apiKeyPrefix starts the one key: the AI agent's and the dashboard's.
const apiKeyPrefix = "axon_k_"

// genKey makes a 256-bit key from crypto/rand.
func genKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return apiKeyPrefix + hex.EncodeToString(b), nil
}
