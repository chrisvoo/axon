package mcp

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIsPrivilegedCmd(t *testing.T) {
	t.Parallel()
	cases := []struct {
		cmd  string
		want bool
	}{
		{"sudo ls", true},
		{"sudo", true},
		{"echo hi && sudo ls", true},
		{"a;sudo id", true},
		{"(sudo id)", true},
		{"/usr/bin/sudo id", true},
		{"env /usr/bin/sudo -n true", true},
		{"echo pseudo", false},
		{"echo sudoku", false},
		{"visudo -c", false},
		{"cat /etc/sudoers", false},
		{"ls", false},
		// Text the shell does not run is not an invocation.
		{`echo "needs sudo"`, false},
		{`echo 'run sudo id'`, false},
		{"ls # sudo later", false},
		{"grep -rn 'sudo' /etc/ssh", false},
		{`echo "it's sudo time"`, false},
		{"echo $#; sudo id", true},
		// Text the shell runs stays visible.
		{`bash -c "sudo id"`, true},
		{`sh -lc 'sudo id'`, true},
		{`eval "sudo id"`, true},
		{`echo "$(sudo id)"`, true},
		{"echo \"`sudo id`\"", true},
		{`echo "x" && sudo id`, true},
		{`echo "unterminated sudo`, true},
		// Review findings: the shell reads these differently from a naive quote scan.
		{`echo $'a\'b'; sudo id; echo $'c'`, true}, // ANSI-C quoting: \' does not close the string
		{"cat <<EOF\nit's\nEOF\nsudo id\n'", true}, // heredoc body holds an apostrophe
		{`"sudo" id`, true},                        // quoted word in command position
		{`echo hi; 'sudo' id`, true},
		{`echo "sudo id" | sh`, true}, // quoted text piped into a shell
		{`ssh host "sudo id"`, true},  // runs on the remote side
		{`xargs -I{} sh -c 'sudo {}'`, true},
	}
	for _, tc := range cases {
		if got := isPrivilegedCmd(tc.cmd); got != tc.want {
			t.Errorf("isPrivilegedCmd(%q) = %v, want %v", tc.cmd, got, tc.want)
		}
	}
}

// A path-qualified sudo must be forwarded to the dashboard, never executed.
func TestHandleToolsCall_PathQualifiedSudoIsNotExecuted(t *testing.T) {
	t.Parallel()
	s := &Server{} // no Cfg: RunShell would panic if it were reached
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"shell","arguments":{"command":"/usr/bin/sudo id"}}}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/mcp", strings.NewReader(body))

	s.HandleJSONRPC(context.Background(), rec, req, "127.0.0.1:1")

	if !strings.Contains(rec.Body.String(), "requires_user_action") {
		t.Fatalf("expected requires_user_action, got %s", rec.Body.String())
	}
}
