package internal

import (
	"reflect"
	"testing"
)

func TestStripEnvVars(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		want []string
	}{
		{"no-assignment", "ls -la", []string{"ls -la"}},
		{"single", "FOO=bar ls -la", []string{"FOO=bar ls -la", "ls -la"}},
		{"multiple", "A=1 B=2 git status", []string{"A=1 B=2 git status", "git status"}},
		{"underscore-name", "_X=y cmd", []string{"_X=y cmd", "cmd"}},
		{"empty-value", "FOO= ls", []string{"FOO= ls", "ls"}},
		{"assignment-only-no-cmd", "FOO=bar", []string{"FOO=bar"}},
		{"not-an-assignment", "1FOO=bar ls", []string{"1FOO=bar ls"}}, // name can't start with digit
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StripEnvVars(tt.cmd); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("StripEnvVars(%q) = %q, want %q", tt.cmd, got, tt.want)
			}
		})
	}
}

func TestMatchesPrefix(t *testing.T) {
	tests := []struct {
		name     string
		cmd      string
		prefixes []string
		want     bool
	}{
		{"exact", "git status", []string{"git status"}, true},
		{"prefix-space", "git status --short", []string{"git status"}, true},
		{"prefix-slash", "cd /tmp/x", []string{"cd /tmp"}, true},
		{"shorter-than-prefix", "git", []string{"git status"}, false},
		{"partial-word-no-boundary", "gitstatus", []string{"git"}, false},
		{"no-match", "npm test", []string{"git", "ls"}, false},
		{"env-stripped-match", "FOO=1 git status", []string{"git status"}, true},
		{"empty-prefix-list", "git status", nil, false},
		{"first-of-many", "ls -la", []string{"ls", "cat", "grep"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchesPrefix(tt.cmd, tt.prefixes); got != tt.want {
				t.Errorf("MatchesPrefix(%q, %q) = %v, want %v", tt.cmd, tt.prefixes, got, tt.want)
			}
		})
	}
}

func TestIsAllowed(t *testing.T) {
	tests := []struct {
		name  string
		cmd   string
		allow []string
		deny  []string
		want  bool
	}{
		{"allowed", "git status", []string{"git status"}, nil, true},
		{"not-in-allow", "git push", []string{"git status"}, nil, false},
		{"denied-beats-allow", "rm -rf /", []string{"rm"}, []string{"rm"}, false},
		{"deny-more-specific-no-match", "rm foo", []string{"rm"}, []string{"rm -rf"}, true},
		{"deny-specific-match", "rm -rf /", []string{"rm"}, []string{"rm -rf"}, false},
		{"empty-allow", "git status", nil, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAllowed(tt.cmd, tt.allow, tt.deny); got != tt.want {
				t.Errorf("IsAllowed(%q, allow=%q, deny=%q) = %v, want %v", tt.cmd, tt.allow, tt.deny, got, tt.want)
			}
		})
	}
}

func TestAllAllowed(t *testing.T) {
	tests := []struct {
		name  string
		cmds  []string
		allow []string
		deny  []string
		want  bool
	}{
		{"all-allowed", []string{"cat a", "grep b"}, []string{"cat", "grep"}, nil, true},
		{"one-not-allowed", []string{"cat a", "rm b"}, []string{"cat"}, nil, false},
		{"one-denied", []string{"cat a", "rm b"}, []string{"cat", "rm"}, []string{"rm"}, false},
		{"skips-empty-segments", []string{"cat a", ""}, []string{"cat"}, nil, true},
		{"empty-list-trivially-true", nil, []string{"cat"}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AllAllowed(tt.cmds, tt.allow, tt.deny); got != tt.want {
				t.Errorf("AllAllowed(%q) = %v, want %v", tt.cmds, got, tt.want)
			}
		})
	}
}

func TestAnyDenied(t *testing.T) {
	tests := []struct {
		name string
		cmds []string
		deny []string
		want bool
	}{
		{"has-denied", []string{"cat a", "rm -rf /"}, []string{"rm"}, true},
		{"none-denied", []string{"cat a", "grep b"}, []string{"rm"}, false},
		{"empty-deny", []string{"rm -rf /"}, nil, false},
		{"skips-empty-segments", []string{"", "cat a"}, []string{"rm"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AnyDenied(tt.cmds, tt.deny); got != tt.want {
				t.Errorf("AnyDenied(%q, deny=%q) = %v, want %v", tt.cmds, tt.deny, got, tt.want)
			}
		})
	}
}
