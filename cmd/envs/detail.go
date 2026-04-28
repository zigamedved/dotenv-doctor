package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zigamedved/dotenv-doctor/internal/classify"
	"github.com/zigamedved/dotenv-doctor/internal/discover"
	"github.com/zigamedved/dotenv-doctor/internal/framework"
	"github.com/zigamedved/dotenv-doctor/internal/parse"
	"github.com/zigamedved/dotenv-doctor/internal/render"
)

func newDetailCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <project>",
		Short: "Detail view for a single project",
		Args:  cobra.ExactArgs(1),
		RunE:  runDetail,
	}
	return cmd
}

func runDetail(cmd *cobra.Command, args []string) error {
	noColor, _ := cmd.Flags().GetBool("no-color")
	reveal, _ := cmd.Flags().GetBool("reveal")
	theme := render.DefaultTheme(noColor || !render.IsTTY())

	projects, _, err := resolveProjects(cmd)
	if err != nil {
		return err
	}

	target := args[0]
	matches := matchProjects(projects, target)
	switch len(matches) {
	case 0:
		return fmt.Errorf("no project matching %q (try `envs` to list)", target)
	case 1:
		// happy path
	default:
		names := make([]string, 0, len(matches))
		for _, m := range matches {
			names = append(names, m.Name)
		}
		return fmt.Errorf("ambiguous project %q matched: %s", target, strings.Join(names, ", "))
	}
	p := matches[0]

	if reveal {
		if !confirmReveal(p.Name) {
			return fmt.Errorf("reveal cancelled")
		}
	}

	fw := framework.Detect(p.Dir)
	fmt.Println(render.Banner(theme, p.Name, p.Dir))
	fmt.Println()

	meta := []string{
		fmt.Sprintf("framework: %s", labelOrDash(theme, string(fw))),
	}
	if p.GitRoot != "" {
		meta = append(meta, fmt.Sprintf("git: %s", p.GitRoot))
	}
	if p.ExampleFile != "" {
		meta = append(meta, fmt.Sprintf("example: %s", filepath.Base(p.ExampleFile)))
	}
	fmt.Println(theme.Muted.Render("  " + strings.Join(meta, "  ·  ")))
	fmt.Println()

	exampleKeys := exampleKeySet(p)

	for _, f := range p.EnvFiles {
		parsed, err := parse.ParseFile(f)
		if err != nil {
			fmt.Println(theme.Bad.Render(fmt.Sprintf("! could not parse %s: %v", f, err)))
			continue
		}
		fmt.Println(render.Section(theme, "▾ "+filepath.Base(f)) + theme.Muted.Render(fmt.Sprintf("  (%d keys)", len(parsed.Entries))))

		rows := make([][]string, 0, len(parsed.Entries))
		for _, e := range parsed.Entries {
			value := parse.Mask(e.Value)
			if reveal {
				value = e.Value
			}
			notes := keyNotes(theme, p, fw, e, exampleKeys, f == p.ExampleFile)
			rows = append(rows, []string{
				fmt.Sprintf("%d", e.Line),
				e.Key,
				value,
				notes,
			})
		}
		fmt.Println(render.Table(theme, []string{"LINE", "KEY", "VALUE", "NOTES"}, rows))

		// Show missing-vs-example diff for non-example files.
		if f != p.ExampleFile && p.ExampleFile != "" {
			missing := missingKeys(parsed, exampleKeys)
			if len(missing) > 0 {
				fmt.Println(theme.Warn.Render(fmt.Sprintf("  missing %d key(s) from .env.example: %s", len(missing), strings.Join(missing, ", "))))
			}
		}
		fmt.Println()
	}

	return nil
}

func matchProjects(projects []discover.Project, target string) []discover.Project {
	var exact, prefix []discover.Project
	for _, p := range projects {
		if p.Name == target {
			exact = append(exact, p)
		} else if strings.HasPrefix(p.Name, target) || strings.Contains(p.Name, target) {
			prefix = append(prefix, p)
		}
	}
	if len(exact) > 0 {
		return exact
	}
	return prefix
}

func exampleKeySet(p discover.Project) map[string]struct{} {
	if p.ExampleFile == "" {
		return nil
	}
	parsed, err := parse.ParseFile(p.ExampleFile)
	if err != nil {
		return nil
	}
	out := map[string]struct{}{}
	for _, e := range parsed.Entries {
		out[e.Key] = struct{}{}
	}
	return out
}

func missingKeys(f parse.File, exampleKeys map[string]struct{}) []string {
	if exampleKeys == nil {
		return nil
	}
	have := map[string]struct{}{}
	for _, e := range f.Entries {
		have[e.Key] = struct{}{}
	}
	var missing []string
	for k := range exampleKeys {
		if _, ok := have[k]; !ok {
			missing = append(missing, k)
		}
	}
	sort.Strings(missing)
	return missing
}

func keyNotes(t render.Theme, _ discover.Project, fw framework.Kind, e parse.Entry, exampleKeys map[string]struct{}, isExample bool) string {
	var notes []string

	findings := classify.ClassifyValue(e.Key, e.Value)
	for _, f := range findings {
		notes = append(notes, t.Bad.Render("! "+f.Rule.Name))
	}

	if framework.IsPublic(fw, e.Key) {
		if framework.LooksSecret(e.Key) || len(findings) > 0 {
			notes = append(notes, t.Bad.Render("! exposed to client bundle"))
		} else {
			notes = append(notes, t.Muted.Render("public"))
		}
	}

	if !isExample && exampleKeys != nil {
		if _, ok := exampleKeys[e.Key]; !ok {
			notes = append(notes, t.Muted.Render("not in .env.example"))
		}
	}

	if len(notes) == 0 {
		return t.Muted.Render("—")
	}
	return strings.Join(notes, "  ")
}

func confirmReveal(name string) bool {
	fmt.Fprintf(os.Stderr, "Reveal will print unmasked values to your terminal.\n")
	fmt.Fprintf(os.Stderr, "Type the project name (%s) to confirm: ", name)
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return false
	}
	return strings.TrimSpace(scanner.Text()) == name
}

func labelOrDash(t render.Theme, s string) string {
	if s == "" {
		return t.Muted.Render("—")
	}
	return s
}
