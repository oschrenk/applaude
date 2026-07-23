/*
Copyright © 2026 Oliver Schrenk
*/
package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/oschrenk/applaude/internal"
)

var (
	flagDebug       bool
	flagPermissions string
	flagDeny        string
)

// opts collects the persistent flags into internal.Options.
func opts() internal.Options {
	return internal.Options{
		Debug:           flagDebug,
		PermissionsJSON: flagPermissions,
		DenyJSON:        flagDeny,
	}
}

var rootCmd = &cobra.Command{
	Use:   "applaude",
	Short: "Auto-approve compound Bash commands for Claude Code",
	Long: `A Claude Code PreToolUse hook that auto-approves compound Bash commands
(pipes, chains, subshells, substitutions) when every sub-command matches your
allow list and none match your deny list.

With no subcommand it runs as the hook: it reads the PreToolUse payload on
stdin and exits 0 (allow / fall through) or 2 (deny).

shfmt and jq are folded in as libraries — this is a single self-contained
binary with no external tool dependencies.`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		os.Exit(internal.RunHook(os.Stdin, os.Stdout, os.Stderr, opts()))
	},
}

func Execute() {
	// hide (but not disable) the "completion" subcommand
	rootCmd.CompletionOptions.HiddenDefaultCmd = true
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	pf := rootCmd.PersistentFlags()
	pf.BoolVar(&flagDebug, "debug", false, "log decisions to stderr")
	pf.StringVar(&flagPermissions, "permissions", "", "JSON array of allow entries (bypasses settings files)")
	pf.StringVar(&flagDeny, "deny", "", "JSON array of deny entries")
}
