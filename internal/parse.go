package internal

import (
	"regexp"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// shellCRe matches a `bash -c '...'` / `sh -c "..."` wrapper (optionally prefixed
// with `env ` or an absolute path) so its inner command can be parsed
// recursively. Mirrors the bash script's recursion pattern.
var shellCRe = regexp.MustCompile(`^(env\s+)?(/\S+/)?(ba)?sh\s+-c\s+['"](.*)['"]$`)

// NeedsCompound reports whether cmd contains shell metacharacters that could
// hide sub-commands — pipes, chains (&&/||), semicolons, background/backgrounding
// (`&`), backticks, and command or process substitution ($(...), <(...), >(...)).
// Simple commands can be matched directly without invoking the parser.
func NeedsCompound(cmd string) bool {
	if strings.ContainsAny(cmd, "|&;`") {
		return true
	}
	return strings.Contains(cmd, "$(") ||
		strings.Contains(cmd, "<(") ||
		strings.Contains(cmd, ">(")
}

// ParseCompound parses a compound command into its individual sub-commands,
// recursing into `bash -c` / `sh -c` wrappers. It returns an error when the
// command cannot be parsed, so callers fall through rather than auto-approving
// an unparseable command.
func ParseCompound(cmd string) ([]string, error) {
	f, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(strings.NewReader(cmd), "")
	if err != nil {
		return nil, err
	}

	var out []string
	syntax.Walk(f, func(node syntax.Node) bool {
		call, ok := node.(*syntax.CallExpr)
		if !ok {
			return true
		}
		parts := make([]string, 0, len(call.Args))
		for _, arg := range call.Args {
			if t := wordText(arg); t != "" {
				parts = append(parts, t)
			}
		}
		if len(parts) == 0 {
			return true
		}
		command := strings.Join(parts, " ")

		// Recurse into `bash -c '...'`: emit the inner commands instead of the
		// wrapper. On inner parse failure, fall through and emit the wrapper
		// as-is so it still gets checked against the allow/deny lists.
		if m := shellCRe.FindStringSubmatch(command); m != nil {
			if inner, err := ParseCompound(m[4]); err == nil {
				out = append(out, inner...)
				return true
			}
		}
		out = append(out, command)
		return true
	})
	return out, nil
}

// wordText renders a word to text using the same rules as the bash script's
// get_part_value: literals verbatim, single/double quotes preserved, parameter
// expansions as $name, and command substitutions collapsed to a $(..)
// placeholder (their inner commands are extracted separately by the walk).
func wordText(w *syntax.Word) string {
	var b strings.Builder
	for _, part := range w.Parts {
		b.WriteString(partText(part))
	}
	return b.String()
}

func partText(part syntax.WordPart) string {
	switch p := part.(type) {
	case *syntax.Lit:
		return p.Value
	case *syntax.SglQuoted:
		return "'" + p.Value + "'"
	case *syntax.DblQuoted:
		var b strings.Builder
		b.WriteByte('"')
		for _, pp := range p.Parts {
			b.WriteString(partText(pp))
		}
		b.WriteByte('"')
		return b.String()
	case *syntax.ParamExp:
		return "$" + p.Param.Value
	case *syntax.CmdSubst:
		return "$(..)"
	default:
		return ""
	}
}
