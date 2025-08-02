package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Color palette using standard terminal colors for better compatibility
var (
	// Base colors - using standard ANSI colors
	primaryColor   = lipgloss.Color("14") // Bright Cyan
	secondaryColor = lipgloss.Color("12") // Bright Blue
	accentColor    = lipgloss.Color("10") // Bright Green
	errorColor     = lipgloss.Color("9")  // Bright Red
	warningColor   = lipgloss.Color("11") // Bright Yellow

	// Background colors - using grayscale
	bgDark    = lipgloss.Color("235") // Dark gray
	bgMedium  = lipgloss.Color("237") // Medium gray
	bgLight   = lipgloss.Color("239") // Light gray
	bgLighter = lipgloss.Color("241") // Lighter gray

	// Text colors
	textPrimary   = lipgloss.Color("15") // Bright White
	textSecondary = lipgloss.Color("7")  // White
	textMuted     = lipgloss.Color("8")  // Bright Black (gray)
)

// Message block styles with modern card-like appearance
var (
	// Base message card style
	messageCardStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(bgLighter).
				Padding(1, 2).
				MarginBottom(1)

	// User message style - clean and prominent
	userMessageStyle = messageCardStyle.Copy().
				BorderForeground(primaryColor)

	userLabelStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true).
			MarginRight(1)

	// Agent message style - professional and readable
	agentMessageStyle = messageCardStyle.Copy().
				BorderForeground(secondaryColor)

	agentLabelStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Bold(true).
			MarginRight(1)

	// Tool message style - technical but accessible
	toolMessageStyle = messageCardStyle.Copy().
				BorderForeground(bgLighter).
				BorderStyle(lipgloss.NormalBorder()).
				Padding(0)

	toolHeaderStyle = lipgloss.NewStyle().
			Background(bgLight).
			Foreground(textPrimary).
			Padding(0, 2).
			Bold(true)

	toolContentStyle = lipgloss.NewStyle().
				Padding(1, 2).
				Foreground(textSecondary)

	// Thought message style - subtle and elegant
	thoughtMessageStyle = messageCardStyle.Copy().
				BorderForeground(bgLighter).
				BorderStyle(lipgloss.DoubleBorder()).
				Padding(0)

	thoughtHeaderStyle = lipgloss.NewStyle().
				Background(bgLight).
				Foreground(textMuted).
				Padding(0, 2).
				Italic(true)

	thoughtContentStyle = lipgloss.NewStyle().
				Padding(1, 2).
				Foreground(textSecondary).
				Italic(true)

	// Success and error styles
	successStyle = lipgloss.NewStyle().
			Foreground(accentColor).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true)

	// Text input style - modern and inviting
	textInputContainerStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(primaryColor).
				Padding(1, 2).
				MarginTop(1)

	// Spinner style
	spinnerStyle = lipgloss.NewStyle().
			Foreground(secondaryColor)

	// Status bar style - minimal and informative
	statusBarStyle = lipgloss.NewStyle().
			Background(bgDark).
			Foreground(textSecondary).
			Padding(0, 1)

	statusItemStyle = lipgloss.NewStyle().
			Foreground(textMuted).
			MarginRight(2)

	// Model selector styles
	modelSelectorStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(primaryColor).
				Background(bgDark).
				Padding(2, 3)

	modelItemStyle = lipgloss.NewStyle().
			Padding(0, 2).
			MarginBottom(1)

	modelItemSelectedStyle = modelItemStyle.Copy().
				Background(primaryColor).
				Foreground(bgDark).
				Bold(true)

	// Streaming indicator style
	streamingIndicatorStyle = lipgloss.NewStyle().
				Foreground(secondaryColor).
				Blink(true)
)

// Modern icons
const (
	userIcon     = "👤"
	agentIcon    = "🤖"
	toolIcon     = "🔧"
	thoughtIcon  = "💭"
	successIcon  = "✓"
	errorIcon    = "✗"
	expandIcon   = "▼"
	collapseIcon = "▶"
	bulletIcon   = "•"
	arrowIcon    = "→"
)

// getExpandCollapseIcon returns the appropriate icon for expand/collapse state
func getExpandCollapseIcon(isCollapsed bool) string {
	if isCollapsed {
		return collapseIcon
	}
	return expandIcon
}

// getStatusIcon returns the appropriate status icon
func getStatusIcon(isError bool) string {
	if isError {
		return errorIcon
	}
	return successIcon
}

// getMessageIcon returns the appropriate icon for a message type
func getMessageIcon(mType messageType) string {
	switch mType {
	case userMessage:
		return userIcon
	case agentMessage:
		return agentIcon
	case toolMessage:
		return toolIcon
	case thoughtMessage:
		return thoughtIcon
	default:
		return bulletIcon
	}
}
