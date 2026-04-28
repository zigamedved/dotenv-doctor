package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zigamedved/dotenv-doctor/internal/classify"
	"github.com/zigamedved/dotenv-doctor/internal/config"
	"github.com/zigamedved/dotenv-doctor/internal/discover"
	"github.com/zigamedved/dotenv-doctor/internal/framework"
	"github.com/zigamedved/dotenv-doctor/internal/git"
	"github.com/zigamedved/dotenv-doctor/internal/parse"
	"github.com/zigamedved/dotenv-doctor/internal/render"
)

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Dashboard of every project and its .env files (default command)",
		RunE:  runList,
	}
	return cmd
}

func runList(cmd *cobra.Command, _ []string) error {
	noColor, _ := cmd.Flags().GetBool("no-color")
	theme := render.DefaultTheme(noColor || !render.IsTTY())

	// First-run trigger: if there's no config and no override and we're not
	// inside a git project, drop the user into the wizard. Without this, a
	// user who runs `envs` from their home directory would get an empty table.
	overridePaths, _ := cmd.Flags().GetStringSlice("path")
	if len(overridePaths) == 0 && !config.Exists() {
		cwd, _ := os.Getwd()
		smart := discover.SmartWalk(cwd)
		// If smart-walk would land on the user's home dir, force the wizard.
		home, _ := os.UserHomeDir()
		if len(smart) == 1 && filepath.Clean(smart[0]) == filepath.Clean(home) {
			fmt.Fprintln(os.Stderr, theme.Muted.Render("No config found and you're in your home directory; running first-run setup."))
			fmt.Fprintln(os.Stderr)
			if err := runInit(cmd, nil); err != nil {
				return err
			}
		}
	}

	projects, paths, err := resolveProjects(cmd)
	if err != nil {
		return err
	}

	fmt.Println(render.Banner(theme, "envs · dotenv-doctor", "the .env files you've lost track of"))
	fmt.Println()

	if len(projects) == 0 {
		fmt.Println(theme.Muted.Render(fmt.Sprintf("No .env files found under: %s", strings.Join(paths, ", "))))
		fmt.Println(theme.Muted.Render("Tip: run `envs init` to configure scan paths."))
		return nil
	}

	rows := make([][]string, 0, len(projects))
	totalLeaks := 0
	totalMissing := 0

	for _, p := range projects {
		fw := framework.Detect(p.Dir)
		issues := projectIssues(p, fw)
		missing := missingKeyCount(p)
		totalMissing += missing
		leaks := quickLeakCount(p)
		totalLeaks += leaks
		issuesCol := render.IssueLine(theme, issues)
		fwLabel := string(fw)
		if fwLabel == "" {
			fwLabel = theme.Muted.Render("—")
		}

		// File column shows the primary .env file (shortest non-example name).
		fileLabel := primaryFile(p)

		rows = append(rows, []string{
			p.Name,
			fwLabel,
			fileLabel,
			fmt.Sprintf("%d", totalKeys(p)),
			missingLabel(theme, missing, p.ExampleFile != ""),
			issuesCol,
		})
	}

	headers := []string{"PROJECT", "FRAMEWORK", "FILE", "KEYS", "MISSING", "ISSUES"}
	fmt.Println(render.Table(theme, headers, rows))
	fmt.Println()

	footer := []string{
		render.Pluralize(len(projects), "project", "projects"),
	}
	if totalLeaks > 0 {
		footer = append(footer, theme.Bad.Render(render.Pluralize(totalLeaks, "leak", "leaks")))
	}
	if totalMissing > 0 {
		footer = append(footer, theme.Warn.Render(render.Pluralize(totalMissing, "missing key", "missing keys")))
	}
	footer = append(footer, "envs <project> for detail", "envs leaks for git scan")
	fmt.Println(render.Footer(theme, footer...))
	return nil
}

func primaryFile(p discover.Project) string {
	// Prefer ".env" if present, otherwise the lex-smallest non-example.
	var nonExample []string
	for _, f := range p.EnvFiles {
		base := filepath.Base(f)
		if !strings.HasSuffix(strings.ToLower(base), ".example") &&
			!strings.HasSuffix(strings.ToLower(base), ".sample") &&
			!strings.HasSuffix(strings.ToLower(base), ".template") {
			nonExample = append(nonExample, f)
		}
	}
	if len(nonExample) == 0 && len(p.EnvFiles) > 0 {
		return filepath.Base(p.EnvFiles[0])
	}
	for _, f := range nonExample {
		if filepath.Base(f) == ".env" {
			return ".env"
		}
	}
	if len(nonExample) > 0 {
		// Show count if multiple.
		if len(nonExample) == 1 {
			return filepath.Base(nonExample[0])
		}
		return fmt.Sprintf("%s (+%d)", filepath.Base(nonExample[0]), len(nonExample)-1)
	}
	return "—"
}

func totalKeys(p discover.Project) int {
	seen := map[string]struct{}{}
	for _, f := range p.EnvFiles {
		// Skip example files in the count.
		base := strings.ToLower(filepath.Base(f))
		if strings.HasSuffix(base, ".example") || strings.HasSuffix(base, ".sample") || strings.HasSuffix(base, ".template") {
			continue
		}
		parsed, err := parse.ParseFile(f)
		if err != nil {
			continue
		}
		for _, e := range parsed.Entries {
			seen[e.Key] = struct{}{}
		}
	}
	return len(seen)
}

func missingKeyCount(p discover.Project) int {
	if p.ExampleFile == "" {
		return 0
	}
	example, err := parse.ParseFile(p.ExampleFile)
	if err != nil {
		return 0
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
	missing := 0
	for _, e := range example.Entries {
		if _, ok := have[e.Key]; !ok {
			missing++
		}
	}
	return missing
}

func missingLabel(t render.Theme, n int, hasExample bool) string {
	if !hasExample {
		return t.Muted.Render("—")
	}
	if n == 0 {
		return t.OK.Render("0")
	}
	return t.Warn.Render(fmt.Sprintf("%d", n))
}

func projectIssues(p discover.Project, fw framework.Kind) []render.Issue {
	var issues []render.Issue

	if p.ExampleFile == "" {
		issues = append(issues, render.Issue{Severity: "low", Label: "no .env.example"})
	}

	// Framework-aware exposure check: secrets in NEXT_PUBLIC_*/VITE_*/REACT_APP_*.
	if exposed := exposedSecretCount(p, fw); exposed > 0 {
		label := "exposed secret in client bundle"
		if exposed > 1 {
			label = fmt.Sprintf("%d secrets exposed in client bundle", exposed)
		}
		issues = append(issues, render.Issue{Severity: "high", Label: label})
	}

	// Pattern matches in current .env files.
	if hits := classifyHitsCount(p); hits > 0 {
		// We don't elevate to "leak" unless it's in git history. This is just a heads-up.
		_ = hits
	}

	// Quick leak count is shown in footer; for the per-row issues column we
	// only flag committed .env files that still exist at HEAD.
	if l := quickLeakCount(p); l > 0 {
		label := "committed to git"
		if l > 1 {
			label = fmt.Sprintf("%d files committed to git", l)
		}
		issues = append(issues, render.Issue{Severity: "high", Label: label})
	}

	return issues
}

func exposedSecretCount(p discover.Project, fw framework.Kind) int {
	prefixes := framework.PublicPrefixes(fw)
	if len(prefixes) == 0 {
		return 0
	}
	count := 0
	for _, f := range p.EnvFiles {
		parsed, err := parse.ParseFile(f)
		if err != nil {
			continue
		}
		for _, e := range parsed.Entries {
			if !framework.IsPublic(fw, e.Key) {
				continue
			}
			if framework.LooksSecret(e.Key) {
				count++
				continue
			}
			// Also flag if the value pattern-matches a known secret type.
			if findings := classify.ClassifyValue(e.Key, e.Value); len(findings) > 0 {
				count++
			}
		}
	}
	return count
}

func classifyHitsCount(p discover.Project) int {
	count := 0
	for _, f := range p.EnvFiles {
		parsed, err := parse.ParseFile(f)
		if err != nil {
			continue
		}
		for _, e := range parsed.Entries {
			if findings := classify.ClassifyValue(e.Key, e.Value); len(findings) > 0 {
				count++
			}
		}
	}
	return count
}

// quickLeakCount checks only the current HEAD tree for committed .env files;
// it does NOT walk history. The full git-history walk is reserved for `envs leaks`
// to keep the dashboard fast.
func quickLeakCount(p discover.Project) int {
	if p.GitRoot == "" {
		return 0
	}
	count := 0
	for _, f := range p.EnvFiles {
		base := strings.ToLower(filepath.Base(f))
		if strings.HasSuffix(base, ".example") || strings.HasSuffix(base, ".sample") || strings.HasSuffix(base, ".template") {
			continue
		}
		// If the file is not in .gitignore AND the file is tracked, it's a leak.
		if git.IsTracked(p.GitRoot, f) {
			count++
		}
	}
	return count
}
