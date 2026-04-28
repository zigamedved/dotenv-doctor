package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zigamedved/dotenv-doctor/internal/config"
	"github.com/zigamedved/dotenv-doctor/internal/render"
)

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Run the first-time setup wizard",
		RunE:  runInit,
	}
	return cmd
}

// runInit is a small, dependency-free prompt-based wizard. We deliberately
// avoid pulling in a TUI form lib for two questions; it bloats the binary and
// breaks in non-TTY contexts (CI, piped input).
func runInit(cmd *cobra.Command, _ []string) error {
	noColor, _ := cmd.Flags().GetBool("no-color")
	theme := render.DefaultTheme(noColor || !render.IsTTY())

	fmt.Println(render.Banner(theme, "envs · setup", "one-time scan path configuration"))
	fmt.Println()
	fmt.Println(theme.Muted.Render("  Everything stays local — envs makes zero network calls."))
	fmt.Println()

	home, _ := os.UserHomeDir()
	defaultPath := filepath.Join(home, "code")
	if !dirExists(defaultPath) {
		for _, candidate := range []string{
			filepath.Join(home, "src"),
			filepath.Join(home, "projects"),
			filepath.Join(home, "dev"),
			filepath.Join(home, "Documents", "code"),
			filepath.Join(home, "Documents", "projects"),
		} {
			if dirExists(candidate) {
				defaultPath = candidate
				break
			}
		}
	}

	in := bufio.NewReader(os.Stdin)

	pathsRaw := promptDefault(in, theme,
		"Where should envs scan for projects?",
		"Comma-separated paths. Tilde (~) is expanded.",
		defaultPath)

	depthRaw := promptDefault(in, theme,
		"Max walk depth?",
		"How deep to walk under each scan path. 4 is right for most layouts.",
		"4")

	c := config.Defaults()
	c.ScanPaths = splitPaths(pathsRaw)
	c.MaxDepth = atoiOr(depthRaw, 4)

	if err := config.Save(c); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	cfgPath, _ := config.Path()
	fmt.Println()
	fmt.Println(theme.OK.Render("● saved " + cfgPath))
	fmt.Println(theme.Muted.Render("  scan paths:  " + strings.Join(c.ScanPaths, ", ")))
	fmt.Println(theme.Muted.Render(fmt.Sprintf("  max depth:   %d", c.MaxDepth)))
	fmt.Println()
	fmt.Println(theme.Accent.Render("→ run `envs` to see your dashboard."))
	return nil
}

func promptDefault(in *bufio.Reader, t render.Theme, question, hint, def string) string {
	fmt.Println(t.Accent.Render("? ") + t.Header.Render(question))
	if hint != "" {
		fmt.Println(t.Muted.Render("  " + hint))
	}
	fmt.Print(t.Muted.Render(fmt.Sprintf("  [%s] ", def)))
	line, err := in.ReadString('\n')
	if err != nil {
		return def
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}

func splitPaths(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		out = []string{"~/code"}
	}
	return out
}

func atoiOr(s string, fallback int) int {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
