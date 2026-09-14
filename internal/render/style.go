// Package render formats documents for terminals (pretty), pipes (plain) and
// machines (JSON).
package render

import "github.com/charmbracelet/lipgloss"

var (
	typeStyles = map[string]lipgloss.Style{
		"reference": lipgloss.NewStyle().Foreground(lipgloss.Color("14")),
		"review":    lipgloss.NewStyle().Foreground(lipgloss.Color("13")),
		"task":      lipgloss.NewStyle().Foreground(lipgloss.Color("12")),
	}
	statusStyles = map[string]lipgloss.Style{
		"todo":        lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9")),
		"planned":     lipgloss.NewStyle().Foreground(lipgloss.Color("14")),
		"in-progress": lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11")),
		"blocked":     lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9")),
		"open":        lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11")),
		"done":        lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10")),
		"superseded":  lipgloss.NewStyle().Faint(true),
	}
	missingStyle = lipgloss.NewStyle().Faint(true)
	headerStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color("18"))
	pathStyle    = lipgloss.NewStyle().Bold(true)
	titleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	faintStyle   = lipgloss.NewStyle().Faint(true)
	errorStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9"))
)

// StyleValue colours a type or status; unknown values render in the default
// colour and absent values as a dim dash.
func StyleValue(value string, styles map[string]lipgloss.Style) string {
	if value == "" {
		return missingStyle.Render("—")
	}
	if style, ok := styles[value]; ok {
		return style.Render(value)
	}
	return value
}
