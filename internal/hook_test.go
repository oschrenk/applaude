package internal

import (
	"bytes"
	"strings"
	"testing"
)

// columnsHeaderCmd is a real-world compound command that decomposes into six
// leaves (see TestParseCompound/assignment-piped-subst-chain): two `grep`, a
// `head`, two `echo`, and a `find`.
const columnsHeaderCmd = `f=$(grep -rl "ColumnsHeader" src/components --include=*.tsx 2>/dev/null | grep -iE "ColumnsHeader" | head); echo "$f"; echo "==="; find src -iname "ColumnsHeader*" -not -name "*.test.*"`

func TestDecide(t *testing.T) {
	tests := []struct {
		name    string
		command string
		allow   []string
		deny    []string
		want    Decision
	}{
		// Simple commands never actively deny — they allow or fall through.
		{"simple-allowed", "git status", []string{"git status"}, nil, AllowDecision},
		{"simple-unknown", "git push", []string{"git status"}, nil, FallThrough},
		{"simple-denied-falls-through", "rm -rf /", []string{"ls"}, []string{"rm"}, FallThrough},

		// Compound commands: all-allowed → allow; denied segment → deny; else fall through.
		{"compound-all-allowed", "cat a | grep b", []string{"cat", "grep"}, nil, AllowDecision},
		{"compound-denied-segment", "cat a | rm -rf /", []string{"cat"}, []string{"rm"}, DenyDecision},
		{"compound-unknown-segment", "cat a | wget x", []string{"cat"}, []string{"rm"}, FallThrough},
		{"compound-chain-allowed", "make && make test", []string{"make"}, nil, AllowDecision},

		// Guard: no allow prefixes → always fall through.
		{"empty-allow", "cat a | grep b", nil, nil, FallThrough},

		// bash -c recursion must surface an inner denied command.
		{"bash-c-recursion-deny", "echo x | bash -c 'rm -rf /'", []string{"echo"}, []string{"rm"}, DenyDecision},
		{"bash-c-recursion-allow", "echo x | bash -c 'cat a && cat b'", []string{"echo", "cat"}, nil, AllowDecision},

		// Unparseable compound → fall through (never auto-approve).
		{"unparseable", "echo a | | echo b", []string{"echo"}, nil, FallThrough},

		// Same ColumnsHeader command under varying allow/deny conditions.
		{"columnsheader-all-allowed", columnsHeaderCmd, []string{"grep", "head", "echo", "find"}, nil, AllowDecision},
		{"columnsheader-echo-denied", columnsHeaderCmd, []string{"grep", "head", "echo", "find"}, []string{"echo"}, DenyDecision},
		{"columnsheader-echo-missing", columnsHeaderCmd, []string{"grep", "head", "find"}, nil, FallThrough},
		{"columnsheader-head-find-missing", columnsHeaderCmd, []string{"grep", "echo"}, nil, FallThrough},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Decide(tt.command, tt.allow, tt.deny); got != tt.want {
				t.Errorf("Decide(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

// hookInput builds a PreToolUse stdin payload for the given command.
func hookInput(command string) string {
	return `{"tool_name":"Bash","tool_input":{"command":` + jsonString(command) + `}}`
}

// jsonString minimally quotes a string for embedding in JSON test fixtures.
func jsonString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

func TestRunHook(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		permissions string
		deny        string
		wantExit    int
		wantStdout  string // substring; "" means empty stdout
		wantStderr  string // substring; "" means empty stderr
	}{
		{
			name:        "allow-compound",
			command:     "cat a | grep b",
			permissions: `["Bash(cat)","Bash(grep)"]`,
			wantExit:    0,
			wantStdout:  `"permissionDecision":"allow"`,
		},
		{
			name:        "deny-compound",
			command:     "cat a | rm -rf /",
			permissions: `["Bash(cat)"]`,
			deny:        `["Bash(rm)"]`,
			wantExit:    2,
			wantStderr:  `"permissionDecision":"deny"`,
		},
		{
			name:        "fallthrough-unknown",
			command:     "wget http://x",
			permissions: `["Bash(cat)"]`,
			wantExit:    0,
			wantStdout:  "",
		},
		{
			name:        "empty-command",
			command:     "",
			permissions: `["Bash(cat)"]`,
			wantExit:    0,
			wantStdout:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			opts := Options{PermissionsJSON: tt.permissions, DenyJSON: tt.deny}
			exit := RunHook(strings.NewReader(hookInput(tt.command)), &stdout, &stderr, opts)

			if exit != tt.wantExit {
				t.Errorf("exit = %d, want %d", exit, tt.wantExit)
			}
			if tt.wantStdout == "" {
				if stdout.Len() != 0 {
					t.Errorf("stdout = %q, want empty", stdout.String())
				}
			} else if !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Errorf("stdout = %q, want to contain %q", stdout.String(), tt.wantStdout)
			}
			if tt.wantStderr != "" && !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q, want to contain %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}

func TestRunParse(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"compound", "cat a | grep b", "cat a\ngrep b\n"},
		{"simple", "ls -la", "ls -la\n"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			exit := RunParse(strings.NewReader(tt.in), &stdout, Options{})
			if exit != 0 {
				t.Errorf("exit = %d, want 0", exit)
			}
			if stdout.String() != tt.want {
				t.Errorf("stdout = %q, want %q", stdout.String(), tt.want)
			}
		})
	}
}
