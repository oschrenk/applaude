package internal

import (
	"regexp"
	"strings"
)

// envAssignRe matches a single leading `VAR=value ` assignment (name, value,
// then whitespace and the remainder). Applied repeatedly to strip a run of
// leading assignments.
var envAssignRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=[^\s]*\s+(.*)`)

// StripEnvVars returns the candidate command strings to match against: the full
// command, plus — when it begins with one or more `VAR=value` assignments — the
// command with those leading assignments removed.
func StripEnvVars(cmd string) []string {
	stripped := cmd
	for {
		m := envAssignRe.FindStringSubmatch(stripped)
		if m == nil {
			break
		}
		stripped = m[1]
	}
	if stripped != cmd {
		return []string{cmd, stripped}
	}
	return []string{cmd}
}

// MatchesPrefix reports whether cmd matches any prefix in the list. A command
// matches a prefix when it equals the prefix, or begins with `prefix ` or
// `prefix/`. Leading env-var assignments are stripped before comparison.
func MatchesPrefix(cmd string, prefixes []string) bool {
	if len(prefixes) == 0 {
		return false
	}
	for _, candidate := range StripEnvVars(cmd) {
		for _, prefix := range prefixes {
			if candidate == prefix ||
				strings.HasPrefix(candidate, prefix+" ") ||
				strings.HasPrefix(candidate, prefix+"/") {
				return true
			}
		}
	}
	return false
}

// IsAllowed reports whether cmd is permitted: it must not match the deny list
// and must match the allow list.
func IsAllowed(cmd string, allow, deny []string) bool {
	if MatchesPrefix(cmd, deny) {
		return false
	}
	return MatchesPrefix(cmd, allow)
}

// AllAllowed reports whether every (non-empty) command in cmds is allowed.
func AllAllowed(cmds, allow, deny []string) bool {
	for _, c := range cmds {
		if c == "" {
			continue
		}
		if !IsAllowed(c, allow, deny) {
			return false
		}
	}
	return true
}

// AnyDenied reports whether any (non-empty) command in cmds matches the deny list.
func AnyDenied(cmds, deny []string) bool {
	if len(deny) == 0 {
		return false
	}
	for _, c := range cmds {
		if c == "" {
			continue
		}
		if MatchesPrefix(c, deny) {
			return true
		}
	}
	return false
}
