package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zigamedved/dotenv-doctor/internal/parse"
	"github.com/zigamedved/dotenv-doctor/internal/render"
)

func newCheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check",
		Short: "CI-friendly: exit non-zero if any project is missing keys vs .env.example",
		Long: `Designed to be a pre-commit/CI check. Walks the same scan paths as the
dashboard, but only inspects projects that have a .env.example. For each
such project, ensures every key in .env.example is present in at least one
non-example .env file in the same directory.

Exit codes:
  0 — all projects are in sync
  1 — at least one project is missing keys
  2 — internal error`,
		RunE:          runCheck,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.Flags().Bool("strict", false, "in strict mode, also fail when a project has no .env.example")
	return cmd
}

func runCheck(cmd *cobra.Command, _ []string) error {
	noColor, _ := cmd.Flags().GetBool("no-color")
	strict, _ := cmd.Flags().GetBool("strict")
	theme := render.DefaultTheme(noColor || !render.IsTTY())

	projects, _, err := resolveProjects(cmd)
	if err != nil {
		// We use exit code 2 for internal errors. cobra would exit with 1
		// otherwise; we want this distinction for CI consumers.
		fmt.Println(theme.Bad.Render("! " + err.Error()))
		return cobraExit(2)
	}

	type problem struct {
		Project string
		Reason  string
	}
	var problems []problem

	checkedAtLeastOne := false
	for _, p := range projects {
		if p.ExampleFile == "" {
			if strict {
				problems = append(problems, problem{Project: p.Name, Reason: "no .env.example"})
			}
			continue
		}
		checkedAtLeastOne = true

		example, err := parse.ParseFile(p.ExampleFile)
		if err != nil {
			problems = append(problems, problem{Project: p.Name, Reason: fmt.Sprintf("parse %s: %v", filepath.Base(p.ExampleFile), err)})
			continue
		}
		have := map[string]struct{}{}
		for _, f := range p.EnvFiles {
			if f == p.ExampleFile {
				continue
			}
			parsed, err := parse.ParseFile(f)
			if err != nil {
				continue
			}
			for _, e := range parsed.Entries {
				have[e.Key] = struct{}{}
			}
		}
		var missing []string
		for _, e := range example.Entries {
			if _, ok := have[e.Key]; !ok {
				missing = append(missing, e.Key)
			}
		}
		sort.Strings(missing)
		if len(missing) > 0 {
			problems = append(problems, problem{
				Project: p.Name,
				Reason:  fmt.Sprintf("missing %d key(s): %s", len(missing), strings.Join(missing, ", ")),
			})
		}
	}

	if len(problems) == 0 {
		if !checkedAtLeastOne && !strict {
			fmt.Println(theme.Muted.Render("ok — no projects with .env.example to check"))
			return nil
		}
		fmt.Println(theme.OK.Render("● ok — every project is in sync with its .env.example"))
		return nil
	}

	for _, pr := range problems {
		fmt.Printf("%s  %s — %s\n",
			theme.Bad.Render("FAIL"),
			theme.Accent.Render(pr.Project),
			pr.Reason,
		)
	}
	return cobraExit(1)
}

// cobraExit returns an error that will cause cobra to exit with the given code.
// We don't call os.Exit directly so tests can capture behavior.
type exitErr struct{ code int }

func (e exitErr) Error() string { return fmt.Sprintf("exit %d", e.code) }
func (e exitErr) ExitCode() int { return e.code }

func cobraExit(code int) error { return exitErr{code: code} }
