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
	"runtime"
	"syscall"

	"github.com/chrisvoo/axon/internal/audit"
	"github.com/chrisvoo/axon/internal/config"
	"github.com/chrisvoo/axon/internal/events"
	"github.com/chrisvoo/axon/internal/hub"
	"github.com/chrisvoo/axon/internal/rootcheck"
	"github.com/chrisvoo/axon/internal/security"
	"github.com/chrisvoo/axon/internal/server"
)

const version = "0.1.0"

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
	case "keygen":
		if err := cmdKeygen(); err != nil {
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
  init      Create config, TLS certificate, API key, and default denylist
  serve     Start HTTPS MCP server
  status    Show configuration paths and certificate fingerprint
  keygen    Rotate API key
  version   Print version

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
	if cfg.APIKey == "" {
		k, err := genAPIKey()
		if err != nil {
			return err
		}
		cfg.APIKey = k
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
	fmt.Printf("API key: %s\n", cfg.APIKey)
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
	_ = fs.Parse(os.Args[2:])

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.APIKey == "" {
		return fmt.Errorf("missing API key: run axon init")
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

	fp, err := security.CertFingerprint(cfg.CertFile)
	if err != nil {
		return err
	}
	host := cfg.ListenAddr
	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	dashboardURL := fmt.Sprintf("https://%s:%d/", host, cfg.ListenPort)
	fmt.Printf("Axon %s listening on https://%s:%d/mcp\n", version, cfg.ListenAddr, cfg.ListenPort)
	fmt.Printf("Dashboard:     %s\n", dashboardURL)
	fmt.Printf("TLS fingerprint (SHA-256 prefix): %s\n", fp)
	fmt.Printf("API key: %s\n\n", cfg.APIKey)
	fmt.Println("Add to Cursor .cursor/mcp.json:")
	fmt.Println(server.PrettyMCPConfig(cfg.ListenAddr, cfg.ListenPort, cfg.APIKey))
	deep, err := server.CursorMCPInstallDeeplink("axon", server.MCPHTTPSURL(cfg.ListenAddr, cfg.ListenPort), cfg.APIKey)
	if err == nil {
		fmt.Println("Cursor one-click install (deeplink — contains API key; do not share):")
		fmt.Println(deep)
		fmt.Println()
	}

	bus := events.NewBus()
	h := hub.NewHub(bus, version)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if !*noBrowser {
		go openBrowser(dashboardURL)
	}

	return server.Run(ctx, cfg, deny, log, server.Options{
		Version: version,
		Events:  bus,
		Hub:     h,
	})
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
	if cfg.APIKey != "" {
		fmt.Printf("api key set:    yes\n")
	} else {
		fmt.Printf("api key set:    no — run axon init\n")
	}
	return nil
}

func cmdKeygen() error {
	if err := rootcheck.EnsureNotRoot(); err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	k, err := genAPIKey()
	if err != nil {
		return err
	}
	cfg.APIKey = k
	if err := cfg.Save(); err != nil {
		return err
	}
	fmt.Printf("New API key: %s\n", k)
	return nil
}

func genAPIKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "axon_k_" + hex.EncodeToString(b), nil
}
