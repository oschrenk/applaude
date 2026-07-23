package internal

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// prefixTrimRe strips a trailing wildcard marker (` *`, `*`, or `:*`) together
// with the closing paren from a permission entry, mirroring the bash script's
// jq `sub("( \\*|\\*|:\\*)\\)$"; "")`.
var prefixTrimRe = regexp.MustCompile(`( \*|\*|:\*)\)$`)

// ConfigDir returns the Claude Code configuration directory: $CLAUDE_CONFIG_DIR
// when set and non-empty, otherwise $HOME/.claude.
func ConfigDir() string {
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return d
	}
	return filepath.Join(os.Getenv("HOME"), ".claude")
}

// GitRoot returns the repository root for the current directory, or "" when not
// inside a git repository. It is worktree-aware: when the git-dir differs from
// the git-common-dir, the common dir's parent is used (matching the bash
// script's find_git_root).
func GitRoot() string {
	toplevel, err := runGit("rev-parse", "--show-toplevel")
	if err != nil || toplevel == "" {
		return ""
	}
	gitDir, _ := runGit("rev-parse", "--git-dir")
	commonDir, _ := runGit("rev-parse", "--git-common-dir")
	if commonDir != "" && gitDir != commonDir {
		return filepath.Dir(commonDir)
	}
	return toplevel
}

func runGit(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// SettingsFiles returns the ordered list of settings files to read: user
// (ConfigDir) settings then project (.claude) settings, each in
// settings.json / settings.local.json order.
func SettingsFiles() []string {
	cfg := ConfigDir()
	files := []string{
		filepath.Join(cfg, "settings.json"),
		filepath.Join(cfg, "settings.local.json"),
	}
	base := ".claude"
	if root := GitRoot(); root != "" {
		base = filepath.Join(root, ".claude")
	}
	return append(files,
		filepath.Join(base, "settings.json"),
		filepath.Join(base, "settings.local.json"),
	)
}

// ExtractPrefix converts a single permission entry such as `Bash(git status:*)`
// into a bare command prefix such as `git status`. It returns "" for entries
// that are not `Bash(...)` permissions or that carry no concrete prefix.
func ExtractPrefix(entry string) string {
	if !strings.HasPrefix(entry, "Bash(") {
		return ""
	}
	s := strings.TrimPrefix(entry, "Bash(")
	s = prefixTrimRe.ReplaceAllString(s, "")
	return strings.TrimSuffix(s, ")")
}

// ExtractPrefixes maps ExtractPrefix over entries, dropping empty results.
func ExtractPrefixes(entries []string) []string {
	var out []string
	for _, e := range entries {
		if p := ExtractPrefix(e); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// settingsFile is the minimal shape read from each settings.json.
type settingsFile struct {
	Permissions struct {
		Allow []string `json:"allow"`
		Deny  []string `json:"deny"`
	} `json:"permissions"`
}

// LoadPrefixes reads every file in SettingsFiles and returns the merged,
// de-duplicated, sorted allow and deny command prefixes.
func LoadPrefixes() (allow, deny []string) {
	seenAllow := map[string]bool{}
	seenDeny := map[string]bool{}
	for _, file := range SettingsFiles() {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		var s settingsFile
		if json.Unmarshal(data, &s) != nil {
			continue
		}
		for _, p := range ExtractPrefixes(s.Permissions.Allow) {
			if !seenAllow[p] {
				seenAllow[p] = true
				allow = append(allow, p)
			}
		}
		for _, p := range ExtractPrefixes(s.Permissions.Deny) {
			if !seenDeny[p] {
				seenDeny[p] = true
				deny = append(deny, p)
			}
		}
	}
	sort.Strings(allow)
	sort.Strings(deny)
	return allow, deny
}
