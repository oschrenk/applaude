package internal

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestExtractPrefix(t *testing.T) {
	tests := []struct {
		entry string
		want  string
	}{
		{"Bash(git status:*)", "git status"},
		{"Bash(npm run *)", "npm run"},
		{"Bash(ls)", "ls"},
		{"Bash(rm:*)", "rm"},
		{"Bash(echo*)", "echo"},
		{"Bash(git commit -m *)", "git commit -m"},
		{"Bash(*)", ""},          // wildcard-only → empty prefix
		{"Bash()", ""},           // empty
		{"Read(/etc/hosts)", ""}, // not a Bash entry
		{"WebFetch", ""},         // not a Bash entry
		{"mcp__foo__bar", ""},    // not a Bash entry
	}
	for _, tt := range tests {
		if got := ExtractPrefix(tt.entry); got != tt.want {
			t.Errorf("ExtractPrefix(%q) = %q, want %q", tt.entry, got, tt.want)
		}
	}
}

func TestExtractPrefixes(t *testing.T) {
	in := []string{"Bash(git status:*)", "Read(x)", "Bash(ls)", "Bash(*)", "WebFetch"}
	want := []string{"git status", "ls"}
	if got := ExtractPrefixes(in); !reflect.DeepEqual(got, want) {
		t.Errorf("ExtractPrefixes(%q) = %q, want %q", in, got, want)
	}
}

func TestConfigDir(t *testing.T) {
	t.Run("uses CLAUDE_CONFIG_DIR when set", func(t *testing.T) {
		t.Setenv("CLAUDE_CONFIG_DIR", "/custom/claude")
		if got := ConfigDir(); got != "/custom/claude" {
			t.Errorf("ConfigDir() = %q, want /custom/claude", got)
		}
	})

	t.Run("falls back to $HOME/.claude", func(t *testing.T) {
		t.Setenv("CLAUDE_CONFIG_DIR", "")
		t.Setenv("HOME", "/home/tester")
		want := filepath.Join("/home/tester", ".claude")
		if got := ConfigDir(); got != want {
			t.Errorf("ConfigDir() = %q, want %q", got, want)
		}
	})
}
