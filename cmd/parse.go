/*
Copyright © 2026 Oliver Schrenk
*/
package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/oschrenk/applaude/internal"
)

var parseCmd = &cobra.Command{
	Use:   "parse",
	Short: "Print the sub-commands extracted from a command read on stdin",
	Long: `Read a single command from stdin and print its extracted sub-commands,
one per line. Intended for debugging what the hook sees.`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		os.Exit(internal.RunParse(os.Stdin, os.Stdout, opts()))
	},
}

func init() {
	rootCmd.AddCommand(parseCmd)
}
