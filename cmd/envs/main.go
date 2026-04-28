package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version is set at build time via -ldflags.
var Version = "dev"

func main() {
	root := newRootCmd()
	if err := root.Execute(); err != nil {
		// Custom exit codes (e.g. from `envs check`) bubble up via exitErr;
		// suppress the printed message for those because the command already
		// rendered its own output.
		if ec, ok := err.(interface{ ExitCode() int }); ok {
			os.Exit(ec.ExitCode())
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "envs [project]",
		Short: "The dashboard for .env files you've lost track of",
		Long: `envs (dotenv-doctor) — a glanceable dashboard for every .env file across
your projects, with built-in git-history leak detection.

Privacy: zero network calls. Everything stays local.`,
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: false,
		Args:          cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				return runDetail(cmd, args)
			}
			return runList(cmd, args)
		},
	}

	cmd.PersistentFlags().Bool("no-color", false, "disable colored output")
	cmd.PersistentFlags().Bool("reveal", false, "reveal masked secret values (requires confirmation)")
	cmd.PersistentFlags().StringSlice("path", nil, "override scan path (can be repeated); skips config")

	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newDetailCmd())
	cmd.AddCommand(newLeaksCmd())
	cmd.AddCommand(newCheckCmd())
	cmd.AddCommand(newInitCmd())

	return cmd
}
