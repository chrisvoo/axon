package mcp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/chrisvoo/axon/internal/audit"
	"github.com/chrisvoo/axon/internal/config"
	"github.com/chrisvoo/axon/internal/events"
	"github.com/chrisvoo/axon/internal/security"
	"github.com/chrisvoo/axon/internal/tools"
)

// Server handles MCP JSON-RPC over HTTP.
type Server struct {
	Cfg     *config.Config
	Deny    *security.Denylist
	Audit   *audit.Logger
	Procs   *tools.ProcManager
	Version string
	Events  *events.Bus
}

// jsonRPCRequest is a minimal JSON-RPC 2.0 request.
type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

// HandleJSONRPC processes one JSON-RPC request and writes the response.
func (s *Server) HandleJSONRPC(ctx context.Context, w http.ResponseWriter, r *http.Request, remoteIP string) {
	var req jsonRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeRPCError(w, nil, -32700, "Parse error")
		return
	}
	if req.JSONRPC != "2.0" {
		writeRPCError(w, req.ID, -32600, "Invalid Request")
		return
	}

	switch req.Method {
	case "initialize":
		s.handleInitialize(w, req)
	case "notifications/initialized":
		w.WriteHeader(http.StatusAccepted)
	case "tools/list":
		s.handleToolsList(w, req)
	case "tools/call":
		s.handleToolsCall(ctx, w, req, remoteIP)
	default:
		writeRPCError(w, req.ID, -32601, "Method not found")
	}
}

func (s *Server) handleInitialize(w http.ResponseWriter, req jsonRPCRequest) {
	res := map[string]any{
		"protocolVersion": "2025-03-26",
		"capabilities": map[string]any{
			"tools": map[string]any{},
		},
		"serverInfo": map[string]any{
			"name":    "axon",
			"version": s.Version,
		},
	}
	writeRPCResult(w, req.ID, res)
}

func (s *Server) handleToolsList(w http.ResponseWriter, req jsonRPCRequest) {
	toolList := []map[string]any{
		toolDef("shell", "Run a shell command on the remote machine. Commands that contain sudo are never executed automatically — they are forwarded to the remote user's dashboard for manual copy-paste and the tool returns {\"status\":\"requires_user_action\"}. When elevated privileges are needed, group all sudo lines into a single call. Non-sudo interactive commands that stall waiting for input return {\"status\":\"input_required\"} with a process_id; use send_input to continue or cancel_command to abort.", map[string]any{
			"command": map[string]any{"type": "string", "description": "Command string"},
		}),
		toolDef("read_file", "Read a text file", map[string]any{
			"path":       map[string]any{"type": "string"},
			"start_line": map[string]any{"type": "number", "description": "1-based start line (optional)"},
			"end_line":   map[string]any{"type": "number", "description": "1-based end line (optional)"},
		}),
		toolDef("write_file", "Write or overwrite a file", map[string]any{
			"path":    map[string]any{"type": "string"},
			"content": map[string]any{"type": "string"},
		}),
		toolDef("edit_file", "Replace unique old_string with new_string in a file", map[string]any{
			"path":       map[string]any{"type": "string"},
			"old_string": map[string]any{"type": "string"},
			"new_string": map[string]any{"type": "string"},
		}),
		toolDef("grep", "Search files with regex under a root path", map[string]any{
			"root":      map[string]any{"type": "string"},
			"pattern":   map[string]any{"type": "string"},
			"max_files": map[string]any{"type": "number"},
		}),
		toolDef("glob", "List files under root matching basename pattern", map[string]any{
			"root":    map[string]any{"type": "string"},
			"pattern": map[string]any{"type": "string"},
		}),
		toolDef("system_info", "Host, CPU, memory, disk summary", map[string]any{}),
		toolDef("send_input", "Send stdin to a process waiting for input. If the process already ended (for example the operator answered in the dashboard), returns its final result: status, output and exit code. Call with empty data to read it. The result stays available for 2 minutes.", map[string]any{
			"process_id": map[string]any{"type": "string"},
			"data":       map[string]any{"type": "string"},
		}),
		toolDef("cancel_command", "Cancel a running or waiting process", map[string]any{
			"process_id": map[string]any{"type": "string"},
		}),
	}
	writeRPCResult(w, req.ID, map[string]any{"tools": toolList})
}

func toolDef(name, desc string, schema map[string]any) map[string]any {
	return map[string]any{
		"name":        name,
		"description": desc,
		"inputSchema": map[string]any{
			"type":       "object",
			"properties": schema,
		},
	}
}

func newCallID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func (s *Server) publish(e events.Event) {
	if s.Events != nil {
		s.Events.Publish(e)
	}
}

func toolDetail(name string, args map[string]any) string {
	switch name {
	case "shell":
		c, _ := args["command"].(string)
		return c
	case "read_file", "write_file", "edit_file":
		p, _ := args["path"].(string)
		return p
	case "grep", "glob":
		r, _ := args["root"].(string)
		return r
	case "send_input", "cancel_command":
		id, _ := args["process_id"].(string)
		return id
	default:
		return name
	}
}

func previewText(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func (s *Server) handleToolsCall(ctx context.Context, w http.ResponseWriter, req jsonRPCRequest, remoteIP string) {
	var params struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeRPCError(w, req.ID, -32602, "Invalid params")
		return
	}
	if params.Arguments == nil {
		params.Arguments = map[string]any{}
	}

	callID := newCallID()
	started := time.Now()
	detail := s.Procs.Redact(toolDetail(params.Name, params.Arguments))
	s.publish(events.Event{
		Type: "tool_called",
		Data: map[string]any{
			"call_id":   callID,
			"tool":      params.Name,
			"remote_ip": remoteIP,
			"detail":    detail,
		},
	})

	var text string
	var err error
	var shellRes tools.ShellResult

	switch params.Name {
	case "shell":
		cmd, _ := params.Arguments["command"].(string)
		s.Audit.Log(audit.Entry{RemoteIP: remoteIP, Action: "shell", Detail: cmd})
		if isPrivilegedCmd(cmd) {
			cmds := splitPrivilegedCommands(cmd)
			s.publish(events.Event{
				Type: "privileged_commands",
				Data: map[string]any{
					"call_id":  callID,
					"commands": cmds,
				},
			})
			text = `{"status":"requires_user_action","message":"This command requires elevated privileges and cannot be run automatically. It has been sent to the dashboard for the remote user to execute manually."}`
		} else {
			shellRes, err = tools.RunShell(ctx, s.Cfg, s.Deny, s.Cfg.ReadOnly, cmd, s.Procs)
			if err == nil {
				var b []byte
				b, err = tools.ShellResultJSON(shellRes)
				text = string(b)
				if shellRes.Status == "input_required" {
					s.publish(events.Event{
						Type: "input_required",
						Data: map[string]any{
							"call_id":    callID,
							"process_id": shellRes.ProcessID,
							"command":    shellRes.Command,
							"last_output": previewText(
								shellRes.LastOutput,
								2000,
							),
							"hint": shellRes.Hint,
						},
					})
				}
			}
		}
	case "read_file":
		path, _ := params.Arguments["path"].(string)
		start, _ := toInt(params.Arguments["start_line"])
		end, _ := toInt(params.Arguments["end_line"])
		s.Audit.Log(audit.Entry{RemoteIP: remoteIP, Action: "read_file", Detail: path})
		text, err = tools.ReadFile(path, start, end)
	case "write_file":
		path, _ := params.Arguments["path"].(string)
		content, _ := params.Arguments["content"].(string)
		s.Audit.Log(audit.Entry{RemoteIP: remoteIP, Action: "write_file", Detail: path})
		err = tools.WriteFile(s.Cfg, path, content)
		if err == nil {
			text = "ok"
		}
	case "edit_file":
		path, _ := params.Arguments["path"].(string)
		oldS, _ := params.Arguments["old_string"].(string)
		newS, _ := params.Arguments["new_string"].(string)
		s.Audit.Log(audit.Entry{RemoteIP: remoteIP, Action: "edit_file", Detail: path})
		err = tools.EditFile(s.Cfg, path, oldS, newS)
		if err == nil {
			text = "ok"
		}
	case "grep":
		root, _ := params.Arguments["root"].(string)
		pattern, _ := params.Arguments["pattern"].(string)
		maxF, _ := toInt(params.Arguments["max_files"])
		s.Audit.Log(audit.Entry{RemoteIP: remoteIP, Action: "grep", Detail: root})
		var hits []map[string]any
		hits, err = tools.Grep(root, pattern, maxF)
		if err == nil {
			b, _ := json.Marshal(hits)
			text = string(b)
		}
	case "glob":
		root, _ := params.Arguments["root"].(string)
		pattern, _ := params.Arguments["pattern"].(string)
		s.Audit.Log(audit.Entry{RemoteIP: remoteIP, Action: "glob", Detail: root})
		var paths []string
		paths, err = tools.Glob(root, pattern)
		if err == nil {
			b, _ := json.Marshal(paths)
			text = string(b)
		}
	case "system_info":
		s.Audit.Log(audit.Entry{RemoteIP: remoteIP, Action: "system_info"})
		var b []byte
		b, err = tools.SystemInfo()
		text = string(b)
	case "send_input":
		id, _ := params.Arguments["process_id"].(string)
		data, _ := params.Arguments["data"].(string)
		s.Audit.Log(audit.Entry{RemoteIP: remoteIP, Action: "send_input", Detail: id})
		var b []byte
		b, err = tools.SendInput(id, data, s.Procs)
		text = string(b)
	case "cancel_command":
		id, _ := params.Arguments["process_id"].(string)
		s.Audit.Log(audit.Entry{RemoteIP: remoteIP, Action: "cancel_command", Detail: id})
		var b []byte
		b, err = tools.CancelCommand(id, s.Procs)
		text = string(b)
	default:
		writeRPCError(w, req.ID, -32601, "Unknown tool")
		return
	}

	// Everything the agent and the dashboard can read passes the secret store.
	text = s.Procs.Redact(text)
	durationMs := time.Since(started).Milliseconds()
	ok := err == nil
	if err != nil {
		errText := s.Procs.Redact(err.Error())
		s.publish(events.Event{
			Type: "tool_result",
			Data: map[string]any{
				"call_id":        callID,
				"tool":           params.Name,
				"remote_ip":      remoteIP,
				"duration_ms":    durationMs,
				"ok":             false,
				"is_error":       true,
				"output_preview": previewText(errText, 4000),
			},
		})
		writeRPCResult(w, req.ID, map[string]any{
			"content": []map[string]any{{"type": "text", "text": "error: " + errText}},
			"isError": true,
		})
		return
	}

	exitCode := 0
	if shellRes.ExitCode != nil {
		exitCode = *shellRes.ExitCode
	}
	resultData := map[string]any{
		"call_id":        callID,
		"tool":           params.Name,
		"remote_ip":      remoteIP,
		"duration_ms":    durationMs,
		"ok":             ok,
		"is_error":       false,
		"output_preview": previewText(text, 8000),
	}
	if params.Name == "shell" {
		resultData["shell_status"] = shellRes.Status
		resultData["exit_code"] = exitCode
		if shellRes.ProcessID != "" {
			resultData["process_id"] = shellRes.ProcessID
		}
	}
	s.publish(events.Event{
		Type: "tool_result",
		Data: resultData,
	})

	writeRPCResult(w, req.ID, map[string]any{
		"content": []map[string]any{{"type": "text", "text": text}},
	})
}

func toInt(v any) (int, bool) {
	switch t := v.(type) {
	case float64:
		return int(t), true
	case int:
		return t, true
	case json.Number:
		i, err := t.Int64()
		return int(i), err == nil
	default:
		return 0, false
	}
}

// isPrivilegedCmd returns true if cmd contains a sudo invocation, including a
// path-qualified one such as /usr/bin/sudo. The word "sudo" in a comment or in a
// quoted string that the shell does not run (echo "needs sudo") is not an invocation.
func isPrivilegedCmd(cmd string) bool {
	return hasSudoWord(maskInertText(cmd))
}

// maskInertText replaces with spaces the text that the shell never runs: comments,
// single-quoted strings and double-quoted strings without command substitution.
// Nothing is masked when the shell could run the quoted text: a heredoc, or an unquoted
// word that hands text to a shell or another command (sh, eval, ssh, xargs, exec, env).
// A quoted string also stays visible in command position ("sudo" id) and when it holds
// $( or a backtick.
func maskInertText(cmd string) string {
	if strings.Contains(cmd, "<<") {
		return cmd
	}
	masked := blankInertText(cmd)
	if hasRunnerWord(masked) {
		return cmd
	}
	return masked
}

func blankInertText(cmd string) string {
	b := []byte(cmd)
	for i := 0; i < len(b); i++ {
		switch c := b[i]; {
		case c == '\\':
			i++ // the next byte is literal
		case c == '#' && (i == 0 || isShellBoundary(b[i-1])):
			j := i
			for j < len(b) && b[j] != '\n' {
				b[j] = ' '
				j++
			}
			i = j - 1
		case c == '\'' || c == '"':
			end, substitutes := closingQuote(b, i)
			if end < 0 {
				return string(b) // unterminated: keep the rest visible
			}
			if substitutes {
				// A $( ) can hold quotes of its own, and closingQuote does not nest them:
				// echo "$(echo "x"; sudo id)" would end the string early and blank the
				// sudo. Stop masking here, so that everything after it stays visible.
				return string(b)
			}
			if !atCommandPosition(b, i) {
				for j := i + 1; j < end; j++ {
					b[j] = ' '
				}
			}
			i = end
		}
	}
	return string(b)
}

// closingQuote returns the index of the quote that closes the one at b[open], or -1.
// substitutes is true when a double-quoted string holds $( or a backtick. A $'...'
// string reads backslash escapes, so \' does not close it.
func closingQuote(b []byte, open int) (end int, substitutes bool) {
	q := b[open]
	escapes := q == '"' || (open > 0 && b[open-1] == '$')
	for j := open + 1; j < len(b); j++ {
		switch {
		case escapes && b[j] == '\\':
			j++
		case b[j] == q:
			return j, substitutes
		case q == '"' && (b[j] == '`' || (b[j] == '$' && j+1 < len(b) && b[j+1] == '(')):
			substitutes = true
		}
	}
	return -1, false
}

// atCommandPosition reports whether the quote at b[quote] starts a command word, as in
// "sudo" id, $'sudo' id or echo hi; 'sudo' id.
func atCommandPosition(b []byte, quote int) bool {
	j := quote
	if j > 0 && b[j-1] == '$' {
		j-- // $'...' and $"..." quote the word the same way
	}
	for j > 0 && (b[j-1] == ' ' || b[j-1] == '\t') {
		j--
	}
	return j == 0 || strings.IndexByte(";&|(\n`{", b[j-1]) >= 0
}

// runnerWords hand text to a shell, an interpreter or another command, so quoted text near
// them can run: bash -c "sudo id", echo "sudo id" | sh, ssh host "sudo id", xargs,
// env "sudo" id, watch 'ls; sudo id', perl -e 'system("sudo id")'. Any python* counts too.
var runnerWords = map[string]bool{
	"sh": true, "bash": true, "zsh": true, "dash": true, "ksh": true, "busybox": true,
	"eval": true, "source": true, "exec": true, "env": true, "command": true,
	"nohup": true, "time": true, "xargs": true, "ssh": true, "su": true, "watch": true,
	"perl": true, "ruby": true, "node": true, "php": true, "lua": true, "awk": true, "gawk": true,
}

func isRunnerWord(base string) bool {
	return runnerWords[base] || strings.HasPrefix(base, "python")
}

// hasRunnerWord reports whether any word of cmd is a runner word. A path counts only
// in a bin directory (/bin/sh), so that grep x /etc/ssh does not.
func hasRunnerWord(cmd string) bool {
	words := strings.FieldsFunc(cmd, func(r rune) bool {
		return r < 128 && (isShellBoundary(byte(r)) || strings.ContainsRune(`"'`+"`)", r))
	})
	for _, w := range words {
		if dir, base := path.Split(w); isRunnerWord(base) && (dir == "" || strings.HasSuffix(dir, "bin/")) {
			return true
		}
	}
	return false
}

// hasSudoWord reports whether cmd holds sudo as a word. A quote, a backtick and a
// closing parenthesis end a word too, so that bash -c "sudo id" and $(sudo) count.
func hasSudoWord(cmd string) bool {
	edge := func(b byte) bool { return isShellBoundary(b) || b == '"' || b == '\'' || b == '`' }
	for i := 0; i <= len(cmd)-4; i++ {
		if cmd[i:i+4] != "sudo" {
			continue
		}
		before := i == 0 || edge(cmd[i-1]) || cmd[i-1] == '/'
		after := i+4 >= len(cmd) || edge(cmd[i+4]) || cmd[i+4] == ')'
		if before && after {
			return true
		}
	}
	return false
}

func isShellBoundary(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == ';' || b == '&' || b == '|' || b == '('
}

// splitPrivilegedCommands splits a multi-line command into individual lines,
// keeping one-liners intact.
func splitPrivilegedCommands(cmd string) []string {
	var out []string
	for _, line := range strings.Split(cmd, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return []string{cmd}
	}
	return out
}

func writeRPCResult(w http.ResponseWriter, id json.RawMessage, result any) {
	w.Header().Set("Content-Type", "application/json")
	out := map[string]any{"jsonrpc": "2.0", "result": result}
	if id != nil {
		out["id"] = json.RawMessage(id)
	}
	_ = json.NewEncoder(w).Encode(out)
}

func writeRPCError(w http.ResponseWriter, id json.RawMessage, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	out := map[string]any{
		"jsonrpc": "2.0",
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	}
	if id != nil {
		out["id"] = json.RawMessage(id)
	}
	_ = json.NewEncoder(w).Encode(out)
}
