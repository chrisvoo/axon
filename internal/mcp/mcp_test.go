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
		// Quotes nested inside $( ) or a backtick must not hide the sudo.
		{`echo "$(echo "x"; sudo id)"`, true},
		{`echo "$(cat "f" && sudo id)"`, true},
		{"echo \"`echo \"x\"; sudo id`\"", true},
		{`echo "$(date)" "$(echo "x"; sudo id)"`, true},
		{`echo "$(date)" 'needs sudo'`, true}, // stops masking after the first $( ): safe side
		// Interpreters and watchers run their quoted argument.
		{`python3 -c "import os; os.system('sudo id')"`, true},
		{`python3.12 -c "import os; os.system('sudo id')"`, true},
		{`perl -e 'system("sudo id")'`, true},
		{`awk 'BEGIN{system("sudo id")}'`, true},
		{`watch 'ls; sudo id'`, true},
		{`su -c 'sudo id'`, true},
		{`bash <<< "sudo id"`, true},
		{`$'sudo' id`, true},
		// Quoted text that nothing runs stays quiet.
		{`grep -c "sudo" /etc/group`, false},
		{`grep -rn "sudo" /etc/ssh`, false},
		{`echo "sudo" "and" 'sudo'`, false},
		{`cat /etc/pythonrc 'needs sudo'`, false},
	}
	for _, tc := range cases {
		if got := isPrivilegedCmd(tc.cmd); got != tc.want {
			t.Errorf("isPrivilegedCmd(%q) = %v, want %v", tc.cmd, got, tc.want)
		}
	}
}

// These commands run sudo, and isPrivilegedCmd does not see it. They are the documented
// limits (docs/SECURITY.md, H4): the check is a guardrail, not a security boundary.
// A case that starts to return true is good news: move it to TestIsPrivilegedCmd and
// update H4.
func TestIsPrivilegedCmd_KnownLimits(t *testing.T) {
	t.Parallel()
	for _, cmd := range []string{
		`s"u"do id`,                          // quotes split the word
		`x=sudo; $x id`,                      // the word comes from a variable
		`echo c3VkbyBpZA== | base64 -d | sh`, // the word is encoded
		`./script.sh`,                        // a script that calls sudo
	} {
		if isPrivilegedCmd(cmd) {
			t.Errorf("isPrivilegedCmd(%q) = true: it was a known limit, so move it to TestIsPrivilegedCmd and update H4", cmd)
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
