package server

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/chrisvoo/axon/internal/audit"
	"github.com/chrisvoo/axon/internal/config"
	"github.com/chrisvoo/axon/internal/dashboard"
	"github.com/chrisvoo/axon/internal/events"
	"github.com/chrisvoo/axon/internal/hub"
	"github.com/chrisvoo/axon/internal/mcp"
	"github.com/chrisvoo/axon/internal/security"
	"github.com/chrisvoo/axon/internal/tools"
)

// Options for HTTPS server.
type Options struct {
	Version string
	Events  *events.Bus
	Hub     *hub.Hub
	// DevMode disables TLS and runs plain HTTP. For local development only.
	DevMode bool
}

// Run starts the HTTPS server until context is cancelled.
func Run(ctx context.Context, cfg *config.Config, deny *security.Denylist, log *audit.Logger, opts Options) error {
	pm := tools.NewProcManager()
	mcpSrv := &mcp.Server{
		Cfg:     cfg,
		Deny:    deny,
		Audit:   log,
		Procs:   pm,
		Version: opts.Version,
		Events:  opts.Events,
	}

	rl := security.NewRateLimiter(cfg.RateLimitRPS)

	webFS, err := fs.Sub(dashboard.FS, "dist")
	if err != nil {
		return err
	}
	staticServer := http.FileServer(http.FS(webFS))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	secure := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			remoteIP := r.RemoteAddr
			if !security.IPAllowed(r, cfg.IPAllowlist) {
				http.Error(w, "forbidden", http.StatusForbidden)
				mcpSrv.Audit.Log(audit.Entry{RemoteIP: remoteIP, Action: "deny_ip"})
				return
			}
			if !rl.Allow(r) {
				http.Error(w, "rate limited", http.StatusTooManyRequests)
				return
			}
			next(w, r)
		}
	}

	mux.HandleFunc("POST /mcp", secure(func(w http.ResponseWriter, r *http.Request) {
		remoteIP := r.RemoteAddr
		token := security.BearerToken(r)
		if token == "" || !security.ConstantTimeEqual(token, cfg.APIKey) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		mcpSrv.HandleJSONRPC(r.Context(), w, r, remoteIP)
	}))

	mux.HandleFunc("POST /api/cancel", secure(func(w http.ResponseWriter, r *http.Request) {
		token := security.BearerToken(r)
		if token == "" || !security.ConstantTimeEqual(token, cfg.APIKey) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		remoteIP := r.RemoteAddr
		var body struct {
			ProcessID string `json:"process_id"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		mcpSrv.Audit.Log(audit.Entry{RemoteIP: remoteIP, Action: "cancel_command", Detail: body.ProcessID + " (dashboard)"})
		b, err := tools.CancelCommand(body.ProcessID, pm)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if opts.Events != nil {
			opts.Events.Publish(events.Event{
				Type: "dashboard_cancel",
				Data: map[string]any{
					"process_id": body.ProcessID,
					"remote_ip":  remoteIP,
				},
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	}))

	mux.HandleFunc("POST /api/input", secure(func(w http.ResponseWriter, r *http.Request) {
		token := security.BearerToken(r)
		if token == "" || !security.ConstantTimeEqual(token, cfg.APIKey) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		remoteIP := r.RemoteAddr
		var body struct {
			ProcessID string `json:"process_id"`
			Data      string `json:"data"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		mcpSrv.Audit.Log(audit.Entry{RemoteIP: remoteIP, Action: "send_input", Detail: body.ProcessID + " (dashboard)"})
		b, err := tools.SendInput(body.ProcessID, body.Data, pm)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if opts.Events != nil {
			opts.Events.Publish(events.Event{
				Type: "dashboard_input",
				Data: map[string]any{
					"process_id": body.ProcessID,
					"remote_ip":  remoteIP,
				},
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	}))

	mux.HandleFunc("GET /ws", secure(func(w http.ResponseWriter, r *http.Request) {
		if opts.Hub == nil {
			http.Error(w, "dashboard unavailable", http.StatusServiceUnavailable)
			return
		}
		opts.Hub.ServeWS(w, r, cfg.APIKey)
	}))

	mux.Handle("GET /", staticServer)

	if opts.Hub != nil && opts.Events != nil {
		go opts.Hub.Run(ctx)
		go opts.Hub.RunSystemStats(ctx, 5*time.Second)
	}

	addr := fmt.Sprintf("%s:%d", cfg.ListenAddr, cfg.ListenPort)

	srv := &http.Server{
		Handler: loggingMiddleware(mux),
	}

	var ln net.Listener
	if opts.DevMode {
		rawLn, err := net.Listen("tcp", addr)
		if err != nil {
			return err
		}
		ln = rawLn
	} else {
		tlsCfg, err := security.LoadTLSConfig(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return err
		}
		rawLn, err := net.Listen("tcp", addr)
		if err != nil {
			return err
		}
		ln = tls.NewListener(rawLn, tlsCfg)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(ln)
	}()

	select {
	case <-ctx.Done():
		shCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shCtx)
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

// ResolveMCPDisplayHost returns the host string used in printed URLs when listen is 0.0.0.0 or empty.
func ResolveMCPDisplayHost(listenAddr string) string {
	if listenAddr == "" || listenAddr == "0.0.0.0" {
		return "127.0.0.1"
	}
	return listenAddr
}

// MCPHTTPSURL is the Axon MCP endpoint URL (https://host:port/mcp) for client configuration.
func MCPHTTPSURL(listenAddr string, port int) string {
	h := ResolveMCPDisplayHost(listenAddr)
	return fmt.Sprintf("https://%s:%d/mcp", h, port)
}

// MCPHTTPDevURL is the plain-HTTP endpoint URL used in dev mode (no TLS).
func MCPHTTPDevURL(listenAddr string, port int) string {
	h := ResolveMCPDisplayHost(listenAddr)
	return fmt.Sprintf("http://%s:%d/mcp", h, port)
}

// CursorMCPInstallDeeplink builds a Cursor-specific install link (cursor://…/mcp/install).
// serverKey is the MCP server id (e.g. "axon"). mcpURL must be the full https://…/mcp URL.
// The link embeds the API key in the query string — treat it as a secret.
func CursorMCPInstallDeeplink(serverKey, mcpURL, apiKey string) (string, error) {
	if serverKey == "" {
		serverKey = "axon"
	}
	payload := map[string]any{
		serverKey: map[string]any{
			"url": mcpURL,
			"headers": map[string]string{
				"Authorization": "Bearer " + apiKey,
			},
		},
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	enc := base64.StdEncoding.EncodeToString(b)
	return fmt.Sprintf(
		"cursor://anysphere.cursor-deeplink/mcp/install?name=%s&config=%s",
		url.QueryEscape(serverKey),
		url.QueryEscape(enc),
	), nil
}

// PrettyMCPConfigForURL builds a Cursor mcp.json snippet for an arbitrary MCP endpoint URL.
// Use this when the public URL is provided externally (e.g. via a Cloudflare tunnel).
func PrettyMCPConfigForURL(serverName, mcpURL, apiKey string) string {
	if serverName == "" {
		serverName = "axon"
	}
	m := map[string]any{
		"mcpServers": map[string]any{
			serverName: map[string]any{
				"url": mcpURL,
				"headers": map[string]string{
					"Authorization": "Bearer " + apiKey,
				},
			},
		},
	}
	b, _ := json.MarshalIndent(m, "", "  ")
	return string(b)
}

// PrettyMCPDevConfig returns a JSON snippet for Cursor mcp.json using plain HTTP (dev mode).
func PrettyMCPDevConfig(listenAddr string, port int, apiKey string) string {
	m := map[string]any{
		"mcpServers": map[string]any{
			"axon-dev": map[string]any{
				"url": MCPHTTPDevURL(listenAddr, port),
				"headers": map[string]string{
					"Authorization": "Bearer " + apiKey,
				},
			},
		},
	}
	b, _ := json.MarshalIndent(m, "", "  ")
	return string(b)
}

// PrettyMCPConfig returns a JSON snippet for Cursor mcp.json.
func PrettyMCPConfig(listenAddr string, port int, apiKey string) string {
	m := map[string]any{
		"mcpServers": map[string]any{
			"axon": map[string]any{
				"url": MCPHTTPSURL(listenAddr, port),
				"headers": map[string]string{
					"Authorization": "Bearer " + apiKey,
				},
			},
		},
	}
	b, _ := json.MarshalIndent(m, "", "  ")
	return string(b)
}
