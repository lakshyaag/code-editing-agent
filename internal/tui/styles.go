package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Core colors - improved visibility
var (
	primaryColor   = lipgloss.Color("87")  // Light Cyan (more visible)
	secondaryColor = lipgloss.Color("75")  // Light Blue (more visible)
	accentColor    = lipgloss.Color("120") // Light Green (more visible)
	errorColor     = lipgloss.Color("203") // Light Red/Pink (softer)
	warningColor   = lipgloss.Color("221") // Light Yellow/Orange
	
	bgDark     = lipgloss.Color("236") // Slightly lighter dark gray
	bgLight    = lipgloss.Color("244") // Medium gray (more visible)
	
	textPrimary = lipgloss.Color("15")  // Bright White
	textMuted   = lipgloss.Color("250") // Light gray (much more visible than 8)
)

// Base styles
var (
	// Base card style for all messages
	cardStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		MarginBottom(1)

	// Header styles
	labelStyle = lipgloss.NewStyle().
		Bold(true).
		MarginRight(1)

	// Tool/thought card style (collapsible)
	collapsibleCardStyle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		Padding(0).
		MarginBottom(1)

	collapsibleHeaderStyle = lipgloss.NewStyle().
		Background(bgLight).
		Padding(0, 2).
		Bold(true)

	collapsibleContentStyle = lipgloss.NewStyle().
		Padding(1, 2)

	// Input and UI elements
	textInputStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(primaryColor).
		Padding(1, 2).
		MarginTop(1)

	spinnerStyle = lipgloss.NewStyle().
		Foreground(secondaryColor)

	statusBarStyle = lipgloss.NewStyle().
		Background(bgDark).
		Padding(0, 1)

	// Modal/overlay styles
	modalStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Background(bgDark).
		Padding(2, 3)

	selectedItemStyle = lipgloss.NewStyle().
		Background(primaryColor).
		Foreground(bgDark).
		Bold(true).
		Padding(0, 2).
		MarginBottom(1)

	normalItemStyle = lipgloss.NewStyle().
		Padding(0, 2).
		MarginBottom(1)
)

// Icons
const (
	userIcon     = "👤"
	agentIcon    = "🤖"
	toolIcon     = "🔧"
	thoughtIcon  = "💭"
	expandIcon   = "▼"
	collapseIcon = "▶"
)
