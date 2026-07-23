package internal

import (
	"reflect"
	"testing"
)

func TestNeedsCompound(t *testing.T) {
	tests := []struct {
		cmd  string
		want bool
	}{
		{"ls -la", false},
		{"git status", false},
		{"echo hello world", false},
		{"cat a | grep b", true},    // pipe
		{"make && make test", true}, // and-chain
		{"a || b", true},            // or-chain
		{"a; b", true},              // semicolon
		{"sleep 1 & echo bg", true}, // background
		{"echo `date`", true},       // backtick
		{"echo $(date)", true},      // command substitution
		{"diff <(a) <(b)", true},    // process substitution (in)
		{"tee >(cat)", true},        // process substitution (out)
		{"[[ $x = y ]]", false},     // test clause, no metachar
	}
	for _, tt := range tests {
		if got := NeedsCompound(tt.cmd); got != tt.want {
			t.Errorf("NeedsCompound(%q) = %v, want %v", tt.cmd, got, tt.want)
		}
	}
}

func TestParseCompound(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		want []string
	}{
		{"simple", "ls -la", []string{"ls -la"}},
		{"pipe+chain", "cat foo | grep bar && echo done", []string{"cat foo", "grep bar", "echo done"}},
		{"cmd-subst", "echo $(git rev-parse HEAD)", []string{"echo $(..)", "git rev-parse HEAD"}},
		{"subshell", "(cd /tmp; ls -la)", []string{"cd /tmp", "ls -la"}},
		{"proc-subst", "diff <(cat a) <(cat b)", []string{"diff", "cat a", "cat b"}},
		{"background", "sleep 1 & echo bg", []string{"sleep 1", "echo bg"}},
		{"dquote-cmdsubst", `echo "val: $(id -u)"`, []string{`echo "val: $(..)"`, "id -u"}},
		{"if-clause", "if true; then rm -rf /x; fi", []string{"true", "rm -rf /x"}},
		{"for-loop", "for f in a b; do cat $f; done", []string{"cat $f"}},
		{"bash-c-recursion", "bash -c 'npm install && npm test'", []string{"npm install", "npm test"}},
		{"sh-c-dquote-recursion", `sh -c "ls -la | wc -l"`, []string{"ls -la", "wc -l"}},
		{"test-clause-no-segment", "[[ ! $x =~ foo ]] && echo yes", []string{"echo yes"}},
		{"dquoted-operator-not-split", `echo "a && b"`, []string{`echo "a && b"`}},
		{"squoted-subst-is-literal", `echo 'literal $(rm z)'`, []string{`echo 'literal $(rm z)'`}},
		{"backtick-subst", "echo `rm y`", []string{"echo $(..)", "rm y"}},
		{"semicolon-chain", "echo a; ls; pwd", []string{"echo a", "ls", "pwd"}},
		{"or-chain", "false || echo fallback", []string{"false", "echo fallback"}},
		{"assignment-subst", "x=$(ls) && echo $x", []string{"ls", "echo $x"}},
		{"nested-subst", "echo nested $(echo $(whoami))", []string{"echo nested $(..)", "echo $(..)", "whoami"}},
		{"redirects-stripped", "echo a > /tmp/f 2>&1", []string{"echo a"}},
		{"if-else", "if true; then echo yes; else echo no; fi", []string{"true", "echo yes", "echo no"}},
		{"heredoc-body-is-data", "cat <<EOF\nsome && text\nEOF\n", []string{"cat"}},
		{
			"assignment-piped-subst-chain",
			`f=$(grep -rl "ColumnsHeader" src/components --include=*.tsx 2>/dev/null | grep -iE "ColumnsHeader" | head); echo "$f"; echo "==="; find src -iname "ColumnsHeader*" -not -name "*.test.*"`,
			[]string{
				`grep -rl "ColumnsHeader" src/components --include=*.tsx`,
				`grep -iE "ColumnsHeader"`,
				"head",
				`echo "$f"`,
				`echo "==="`,
				`find src -iname "ColumnsHeader*" -not -name "*.test.*"`,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseCompound(tt.cmd)
			if err != nil {
				t.Fatalf("ParseCompound(%q) unexpected error: %v", tt.cmd, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseCompound(%q) = %q, want %q", tt.cmd, got, tt.want)
			}
		})
	}
}

func TestParseCompoundSyntaxError(t *testing.T) {
	for _, cmd := range []string{"echo a | | echo b", "cat <(echo", "foo && "} {
		if _, err := ParseCompound(cmd); err == nil {
			t.Errorf("ParseCompound(%q) expected error, got nil", cmd)
		}
	}
}
