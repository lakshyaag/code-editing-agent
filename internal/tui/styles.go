package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Style definitions for the TUI components
var (
	// Conversation styles
	userStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("99"))
	agentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))

	// Tool call styles
	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("35")). // Green
			Bold(true)
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")). // Red
			Bold(true)

	// Spinner style
	spinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("69"))

	// Tool call background style
	toolBackgroundStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("237")). // A lighter gray
				Padding(0, 1)

	// Status bar style
	statusBarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("235")).
			Foreground(lipgloss.Color("250"))
)

// getExpandCollapseIcon returns the appropriate icon for expand/collapse state
func getExpandCollapseIcon(isCollapsed bool) string {
	if isCollapsed {
		return "▶"
	}
	return "▼"
}

// getStatusIcon returns the appropriate status icon and text
func getStatusIcon(isError bool) (string, string) {
	if isError {
		return "✗", "Failed"
	}
	return "✓", "Success"
}
