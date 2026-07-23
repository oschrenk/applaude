package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Decision is the outcome of evaluating a Bash command against the allow/deny
// prefixes.
type Decision int

const (
	// FallThrough hands control back to Claude Code's native permission prompt
	// (exit 0, no output). It is the fail-safe default for unknown or
	// unparseable commands.
	FallThrough Decision = iota
	// AllowDecision auto-approves the command (exit 0, ALLOW JSON on stdout).
	AllowDecision
	// DenyDecision actively blocks the command (exit 2, DENY JSON on stderr).
	DenyDecision
)

// allowJSON is the fixed PreToolUse "allow" payload written to stdout.
const allowJSON = `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"allow"}}`

// denyMessage is the system message emitted with a deny decision.
const denyMessage = "Compound command contains a denied sub-command"

// HookInput mirrors the PreToolUse hook payload received on stdin.
type HookInput struct {
	ToolInput struct {
		Command string `json:"command"`
	} `json:"tool_input"`
}

// Options carries the parsed CLI flags into a run.
type Options struct {
	Debug           bool
	PermissionsJSON string // JSON array of allow entries; bypasses settings files when set
	DenyJSON        string // JSON array of deny entries
}

// denyOutput is the PreToolUse "deny" payload; field order gives stable JSON.
type denyOutput struct {
	HookSpecificOutput struct {
		HookEventName      string `json:"hookEventName"`
		PermissionDecision string `json:"permissionDecision"`
	} `json:"hookSpecificOutput"`
	SystemMessage string `json:"systemMessage"`
}

func denyJSON() string {
	var out denyOutput
	out.HookSpecificOutput.HookEventName = "PreToolUse"
	out.HookSpecificOutput.PermissionDecision = "deny"
	out.SystemMessage = denyMessage
	b, _ := json.Marshal(out)
	return string(b)
}

// Decide evaluates command against the allow/deny prefixes and returns the hook
// decision. It is the pure core of the hook, independent of I/O.
//
// Simple commands only ever allow or fall through — they never actively deny;
// active denial is reserved for compound commands that hide a denied
// sub-command. With no allow prefixes configured, every command falls through.
func Decide(command string, allow, deny []string) Decision {
	if len(allow) == 0 {
		return FallThrough
	}

	if !NeedsCompound(command) {
		if IsAllowed(command, allow, deny) {
			return AllowDecision
		}
		return FallThrough
	}

	cmds, err := ParseCompound(command)
	if err != nil || len(cmds) == 0 {
		return FallThrough
	}
	if AllAllowed(cmds, allow, deny) {
		return AllowDecision
	}
	if AnyDenied(cmds, deny) {
		return DenyDecision
	}
	return FallThrough
}

// resolvePrefixes returns the allow/deny prefixes for a run: from the
// --permissions/--deny flags when set (used for testing), otherwise loaded from
// the settings files.
func resolvePrefixes(opts Options) (allow, deny []string) {
	if opts.PermissionsJSON == "" {
		return LoadPrefixes()
	}
	var a []string
	if json.Unmarshal([]byte(opts.PermissionsJSON), &a) == nil {
		allow = ExtractPrefixes(a)
	}
	if opts.DenyJSON != "" {
		var d []string
		if json.Unmarshal([]byte(opts.DenyJSON), &d) == nil {
			deny = ExtractPrefixes(d)
		}
	}
	return allow, deny
}

func debugf(w io.Writer, opts Options, format string, args ...any) {
	if opts.Debug {
		fmt.Fprintf(w, "[applaude] "+format+"\n", args...)
	}
}

// RunHook executes hook mode: it reads the PreToolUse payload from stdin,
// resolves the allow/deny prefixes, evaluates the command, writes any decision
// output, and returns the process exit code (0 for allow/fall-through, 2 for
// deny).
func RunHook(stdin io.Reader, stdout, stderr io.Writer, opts Options) int {
	data, err := io.ReadAll(stdin)
	if err != nil {
		return 0
	}
	var input HookInput
	if json.Unmarshal(data, &input) != nil {
		return 0
	}
	command := input.ToolInput.Command
	if command == "" {
		return 0
	}
	debugf(stderr, opts, "command: %s", command)

	allow, deny := resolvePrefixes(opts)
	debugf(stderr, opts, "loaded %d allow, %d deny prefixes", len(allow), len(deny))
	if len(allow) == 0 {
		return 0
	}

	switch Decide(command, allow, deny) {
	case AllowDecision:
		fmt.Fprintln(stdout, allowJSON)
		return 0
	case DenyDecision:
		fmt.Fprintln(stderr, denyJSON())
		return 2
	default:
		return 0
	}
}

// RunParse executes parse mode: it reads a single command from stdin and writes
// its extracted sub-commands to stdout, one per line. Used for debugging.
func RunParse(stdin io.Reader, stdout io.Writer, opts Options) int {
	data, err := io.ReadAll(stdin)
	if err != nil {
		return 0
	}
	cmd := strings.TrimRight(string(data), "\n")
	if cmd == "" {
		return 0
	}
	if !NeedsCompound(cmd) {
		fmt.Fprintln(stdout, cmd)
		return 0
	}
	cmds, err := ParseCompound(cmd)
	if err != nil {
		return 0
	}
	for _, c := range cmds {
		if c != "" {
			fmt.Fprintln(stdout, c)
		}
	}
	return 0
}
