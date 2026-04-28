// Package render produces the lipgloss-styled tables and detail views.
//
// We use box-drawing tables (no truecolor required) and reserve color for
// status badges. The aesthetic target is port-whisperer: dense, dashboard-y,
// instantly grokkable.
package render

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

// Theme is the small palette used across the app.
type Theme struct {
	Header   lipgloss.Style
	Border   lipgloss.Style
	OK       lipgloss.Style
	Warn     lipgloss.Style
	Bad      lipgloss.Style
	Muted    lipgloss.Style
	Accent   lipgloss.Style
	Bullet   lipgloss.Style
	NoColor  bool
}

// DefaultTheme is what every rendered surface uses unless overridden.
func DefaultTheme(noColor bool) Theme {
	if noColor {
		plain := lipgloss.NewStyle()
		return Theme{
			Header: plain.Bold(true),
			Border: plain,
			OK:     plain,
			Warn:   plain,
			Bad:    plain.Bold(true),
			Muted:  plain.Faint(true),
			Accent: plain.Bold(true),
			Bullet: plain,
		}
	}
	return Theme{
		Header: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")),
		Border: lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		OK:     lipgloss.NewStyle().Foreground(lipgloss.Color("42")),  // green
		Warn:   lipgloss.NewStyle().Foreground(lipgloss.Color("214")), // amber
		Bad:    lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true),
		Muted:  lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		Accent: lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true),
		Bullet: lipgloss.NewStyle().Foreground(lipgloss.Color("39")),
	}
}

// Banner returns a centered title banner.
func Banner(t Theme, title, subtitle string) string {
	w := 50
	box := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(t.Border.GetForeground()).
		Padding(0, 2).
		Width(w)
	body := t.Accent.Render(title)
	if subtitle != "" {
		body += "\n" + t.Muted.Render(subtitle)
	}
	return box.Render(body)
}

// Table builds a simple lipgloss table with consistent styling.
func Table(t Theme, headers []string, rows [][]string) string {
	tbl := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(t.Border).
		Headers(headers...).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return t.Header.Padding(0, 1)
			}
			return lipgloss.NewStyle().Padding(0, 1)
		}).
		Rows(rows...)
	return tbl.String()
}

// StatusOK formats a green status badge.
func StatusOK(t Theme, label string) string  { return t.OK.Render("● " + label) }

// StatusWarn formats an amber status badge.
func StatusWarn(t Theme, label string) string { return t.Warn.Render("◐ " + label) }

// StatusBad formats a red status badge.
func StatusBad(t Theme, label string) string  { return t.Bad.Render("! " + label) }

// IssueLine builds a comma-separated issues column with appropriate coloring.
func IssueLine(t Theme, issues []Issue) string {
	if len(issues) == 0 {
		return t.Muted.Render("—")
	}
	parts := make([]string, 0, len(issues))
	for _, iss := range issues {
		switch iss.Severity {
		case "high":
			parts = append(parts, t.Bad.Render("! "+iss.Label))
		case "medium":
			parts = append(parts, t.Warn.Render("! "+iss.Label))
		default:
			parts = append(parts, t.Muted.Render(iss.Label))
		}
	}
	return strings.Join(parts, "  ")
}

// Issue is the shared shape used by the dashboard's issues column.
type Issue struct {
	Severity string // "high" | "medium" | "low"
	Label    string
}

// Section renders a section header above content, e.g. "PROJECTS" / "LEAKS".
func Section(t Theme, title string) string {
	return t.Accent.Render(title)
}

// Footer renders the small grey line under tables, port-whisperer-style.
func Footer(t Theme, parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return t.Muted.Render("  " + strings.Join(out, "  ·  "))
}

// IsTTY reports whether stdout is a terminal. Used to decide on color/animation.
func IsTTY() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// Pluralize is a tiny helper for the footer ("5 projects", "1 project").
func Pluralize(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}
