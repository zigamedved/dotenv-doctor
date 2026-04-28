package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zigamedved/dotenv-doctor/internal/classify"
	"github.com/zigamedved/dotenv-doctor/internal/git"
	"github.com/zigamedved/dotenv-doctor/internal/render"
)

func newLeaksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "leaks",
		Short: "Scan git history for committed .env files and secret patterns",
		Long: `Walks every commit reachable from HEAD in each project's git repository,
flags any committed .env file (excluding .env.example variants), and runs the
curated secret-pattern set against the file's contents.

This is read-only and uses pure-Go git (no system git binary required).`,
		RunE: runLeaks,
	}
	cmd.Flags().Int("limit", 5000, "max commits per repo to walk (0 = no limit)")
	return cmd
}

func runLeaks(cmd *cobra.Command, _ []string) error {
	noColor, _ := cmd.Flags().GetBool("no-color")
	theme := render.DefaultTheme(noColor || !render.IsTTY())

	projects, _, err := resolveProjects(cmd)
	if err != nil {
		return err
	}

	limit, _ := cmd.Flags().GetInt("limit")

	fmt.Println(render.Banner(theme, "envs leaks", "scanning git history for committed .env files"))
	fmt.Println()

	type repoResult struct {
		Repo  string
		Leaks []git.Leak
	}

	seenRepos := map[string]struct{}{}
	var results []repoResult
	for _, p := range projects {
		if p.GitRoot == "" {
			continue
		}
		if _, dup := seenRepos[p.GitRoot]; dup {
			continue
		}
		seenRepos[p.GitRoot] = struct{}{}
		fmt.Println(theme.Muted.Render(fmt.Sprintf("  scanning %s …", p.GitRoot)))
		leaks, err := git.ScanRepo(p.GitRoot, limit)
		if err != nil {
			fmt.Println(theme.Bad.Render(fmt.Sprintf("  ! %s: %v", p.GitRoot, err)))
			continue
		}
		results = append(results, repoResult{Repo: p.GitRoot, Leaks: leaks})
	}

	fmt.Println()

	if len(results) == 0 {
		fmt.Println(theme.Muted.Render("No git repos found in scan paths."))
		return nil
	}

	totalLeaks := 0
	totalRepos := 0
	for _, r := range results {
		if len(r.Leaks) == 0 {
			continue
		}
		totalRepos++
		totalLeaks += len(r.Leaks)

		fmt.Println(render.Section(theme, "▾ "+r.Repo))
		rows := make([][]string, 0, len(r.Leaks))
		for _, l := range r.Leaks {
			what := summarizeFindings(l.Findings)
			rows = append(rows, []string{
				l.Path,
				l.ShortSHA,
				l.When,
				truncate(l.Author, 28),
				what,
				stillExistsLabel(theme, l.StillExists),
			})
		}
		sort.SliceStable(rows, func(i, j int) bool { return rows[i][2] > rows[j][2] })
		fmt.Println(render.Table(theme, []string{"PATH", "COMMIT", "DATE", "AUTHOR", "FINDINGS", "AT HEAD"}, rows))

		worst := pickWorst(r.Leaks)
		fmt.Println(theme.Warn.Render("  remediation:"))
		fmt.Println(theme.Muted.Render(indent(git.RemediationHint(worst), "    ")))

		if link := githubSecretScanningLink(r.Repo); link != "" {
			fmt.Println(theme.Muted.Render("  GitHub secret scanning: " + link))
		}
		fmt.Println()
	}

	if totalLeaks == 0 {
		fmt.Println(theme.OK.Render("● No committed .env files found in history. You're clean."))
		return nil
	}

	fmt.Println(render.Footer(theme,
		theme.Bad.Render(render.Pluralize(totalLeaks, "leak", "leaks")),
		render.Pluralize(totalRepos, "repo", "repos"),
		"rotate the credentials first, then rewrite history",
	))
	return nil
}

func summarizeFindings(findings []classify.Finding) string {
	if len(findings) == 0 {
		return "—"
	}
	seen := map[string]struct{}{}
	var names []string
	for _, f := range findings {
		if _, dup := seen[f.Rule.ID]; dup {
			continue
		}
		seen[f.Rule.ID] = struct{}{}
		names = append(names, f.Rule.Name)
	}
	if len(names) > 3 {
		names = append(names[:3], fmt.Sprintf("+%d more", len(names)-3))
	}
	return strings.Join(names, ", ")
}

func stillExistsLabel(t render.Theme, still bool) string {
	if still {
		return t.Bad.Render("yes")
	}
	return t.Muted.Render("no")
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 1 {
		return s[:max]
	}
	return s[:max-1] + "…"
}

func indent(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}

func pickWorst(leaks []git.Leak) git.Leak {
	if len(leaks) == 0 {
		return git.Leak{}
	}
	best := leaks[0]
	bestScore := -1
	for _, l := range leaks {
		s := 0
		for _, f := range l.Findings {
			s += int(f.Rule.Severity) + 1
		}
		if l.StillExists {
			s += 10
		}
		if s > bestScore {
			best = l
			bestScore = s
		}
	}
	return best
}

// githubSecretScanningLink returns the GitHub secret-scanning UI URL for a
// repo if its origin remote points at GitHub.
func githubSecretScanningLink(repoDir string) string {
	url := git.OriginURL(repoDir)
	if url == "" {
		return ""
	}
	owner, repo, ok := git.ParseGitHubOriginURL(url)
	if !ok {
		return ""
	}
	return fmt.Sprintf("https://github.com/%s/%s/security/secret-scanning", owner, repo)
}
