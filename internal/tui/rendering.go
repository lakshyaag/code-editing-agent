package tui

import (
	"fmt"
	"os"
	"strings"

	"agent/internal/config"
	"github.com/charmbracelet/lipgloss"
)

// renderConversation renders all messages in the conversation with modern styling
func (m *model) renderConversation() string {
	m.ui.clickableLines = make(map[int]int)
	var lines []string
	var currentLine int

	// Add welcome header if no messages yet
	if len(m.messages) == 0 {
		welcomeHeader := m.renderWelcomeHeader()
		lines = append(lines, welcomeHeader)
		currentLine += lipgloss.Height(welcomeHeader)
		lines = append(lines, "") // Add spacing
		currentLine++
	}

	// Add some top padding
	lines = append(lines, "")
	currentLine++

	for i, msg := range m.messages {
		var renderedBlock string
		switch msg.mType {
		case userMessage:
			renderedBlock = m.renderUserMessage(msg)
		case agentMessage:
			renderedBlock = m.renderAgentMessage(msg)
		case toolMessage:
			renderedBlock = m.renderToolMessage(msg, i, &currentLine)
		case thoughtMessage:
			renderedBlock = m.renderThoughtMessage(msg, i, &currentLine)
		}
		lines = append(lines, renderedBlock)
		currentLine += lipgloss.Height(renderedBlock)
	}

	// Add some bottom padding
	lines = append(lines, "")

	return strings.Join(lines, "\n")
}

// renderUserMessage renders a user message with modern card styling
func (m *model) renderUserMessage(msg message) string {
	// Create header with icon and label
	header := lipgloss.JoinHorizontal(
		lipgloss.Top,
		userLabelStyle.Render(userIcon+" You"),
	)

	// Render markdown content
	var content string
	renderedMarkdown, err := m.config.markdownRenderer.Render(msg.content)
	if err != nil {
		content = msg.content
	} else {
		content = strings.TrimRight(renderedMarkdown, "\n")
	}

	// Apply text color styling to content
	styledContent := lipgloss.NewStyle().
		Foreground(textSecondary).
		Width(m.ui.viewport.Width - 10). // Account for card padding and borders
		Render(content)

	// Combine header and content
	messageContent := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		styledContent,
	)

	// Apply card styling
	return userMessageStyle.
		Width(m.ui.viewport.Width - 4). // Account for viewport margins
		Render(messageContent)
}

// renderAgentMessage renders an agent message with modern card styling
func (m *model) renderAgentMessage(msg message) string {
	// Create header with icon and label
	header := lipgloss.JoinHorizontal(
		lipgloss.Top,
		agentLabelStyle.Render(agentIcon+" Assistant"),
	)

	// Add streaming indicator if message is still streaming
	if msg.isStreaming {
		streamIndicator := streamingIndicatorStyle.Render(" ●")
		header = lipgloss.JoinHorizontal(lipgloss.Top, header, streamIndicator)
	}

	// Render markdown content
	var content string
	renderedMarkdown, err := m.config.markdownRenderer.Render(msg.content)
	if err != nil {
		content = msg.content
	} else {
		content = strings.TrimRight(renderedMarkdown, "\n")
	}

	// Apply text color styling to content
	styledContent := lipgloss.NewStyle().
		Foreground(textSecondary).
		Width(m.ui.viewport.Width - 10). // Account for card padding and borders
		Render(content)

	// Combine header and content
	messageContent := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		styledContent,
	)

	// Apply card styling
	return agentMessageStyle.
		Width(m.ui.viewport.Width - 4). // Account for viewport margins
		Render(messageContent)
}

// renderToolMessage renders a tool call message with collapsible content
func (m *model) renderToolMessage(msg message, index int, currentLine *int) string {
	// Parse tool content
	lines := strings.Split(msg.content, "\n")
	toolName := "Tool Call"
	if len(lines) > 0 && strings.Contains(lines[0], "Tool Call:") {
		toolName = strings.TrimPrefix(lines[0], "🔧 Tool Call: ")
	}

	// Create header with expand/collapse icon, status icon, and tool name
	statusIcon := getStatusIcon(msg.isError)
	expandIcon := getExpandCollapseIcon(msg.isCollapsed)

	headerContent := fmt.Sprintf("%s %s %s %s", expandIcon, toolIcon, statusIcon, toolName)

	// Apply error styling if needed
	var headerStyleToUse lipgloss.Style
	if msg.isError {
		headerStyleToUse = toolHeaderStyle.Copy().Foreground(errorColor)
	} else {
		headerStyleToUse = toolHeaderStyle.Copy().Foreground(accentColor)
	}

	header := headerStyleToUse.
		Width(m.ui.viewport.Width - 6). // Account for borders
		Render(headerContent)

	// Make header clickable
	m.ui.clickableLines[*currentLine] = index

	if msg.isCollapsed {
		// Return just the header in a card
		return toolMessageStyle.
			Width(m.ui.viewport.Width - 4).
			Render(header)
	}

	// Format and render the expanded content
	formattedContent := formatToolContent(msg.content)
	renderedContent, err := m.config.markdownRenderer.Render(formattedContent)
	if err != nil {
		renderedContent = msg.content
	} else {
		renderedContent = strings.TrimRight(renderedContent, "\n")
	}

	// Style the content
	styledContent := toolContentStyle.
		Width(m.ui.viewport.Width - 10).
		Render(renderedContent)

	// Combine header and content
	fullContent := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		styledContent,
	)

	// Apply card styling
	return toolMessageStyle.
		Width(m.ui.viewport.Width - 4).
		Render(fullContent)
}

// renderThoughtMessage renders a thought message with collapsible content
func (m *model) renderThoughtMessage(msg message, index int, currentLine *int) string {
	// Create header with expand/collapse icon and thought indicator
	expandIcon := getExpandCollapseIcon(msg.isCollapsed)
	headerContent := fmt.Sprintf("%s %s Thinking...", expandIcon, thoughtIcon)

	header := thoughtHeaderStyle.
		Width(m.ui.viewport.Width - 6). // Account for borders
		Render(headerContent)

	// Make header clickable
	m.ui.clickableLines[*currentLine] = index

	if msg.isCollapsed {
		// Return just the header in a card
		return thoughtMessageStyle.
			Width(m.ui.viewport.Width - 4).
			Render(header)
	}

	// Extract and render the thought content
	content := strings.TrimPrefix(msg.content, "💭 Thinking: ")
	renderedContent, err := m.config.markdownRenderer.Render(content)
	if err != nil {
		renderedContent = content
	} else {
		renderedContent = strings.TrimRight(renderedContent, "\n")
	}

	// Style the content
	styledContent := thoughtContentStyle.
		Width(m.ui.viewport.Width - 10).
		Render(renderedContent)

	// Combine header and content
	fullContent := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		styledContent,
	)

	// Apply card styling
	return thoughtMessageStyle.
		Width(m.ui.viewport.Width - 4).
		Render(fullContent)
}

// renderWelcomeHeader renders the welcome message as a header
func (m *model) renderWelcomeHeader() string {
	// Create header with welcome icon and title
	header := lipgloss.NewStyle().
		Foreground(accentColor).
		Bold(true).
		Render("🎉 Welcome to CLI Code Assistant")

	// Format the welcome message
	welcomeContent := fmt.Sprintf(config.WelcomeMessage, len(config.SystemPrompt))
	
	// Split content into lines for better control
	lines := strings.Split(welcomeContent, "\n")
	var styledLines []string

	// Style each line separately
	contentStyle := lipgloss.NewStyle().
		Foreground(textSecondary)

	for _, line := range lines {
		if line != "" {
			styledLines = append(styledLines, contentStyle.Render(line))
		} else {
			styledLines = append(styledLines, "") // Preserve empty lines
		}
	}

	// Join the styled content
	styledContent := strings.Join(styledLines, "\n")

	// Combine header and content with proper spacing
	messageContent := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		"", // Add a blank line for spacing
		styledContent,
	)

	// Apply special welcome card styling
	welcomeCardStyle := messageCardStyle.Copy().
		BorderForeground(accentColor).
		BorderStyle(lipgloss.DoubleBorder())

	// Calculate proper width
	cardWidth := m.ui.viewport.Width - 4
	if cardWidth < 40 {
		cardWidth = 40 // Ensure minimum width
	}

	return welcomeCardStyle.
		Width(cardWidth).
		Render(messageContent)
}

// statusBarView renders the status bar with modern styling
func (m *model) statusBarView() string {
	if !m.ui.showStatusBar {
		return ""
	}

	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "n/a"
	}

	// Truncate CWD if too long
	maxCwdLen := 30
	if len(cwd) > maxCwdLen {
		cwd = "..." + cwd[len(cwd)-maxCwdLen+3:]
	}

	// Build status items with icons
	modelInfo := statusItemStyle.Render(fmt.Sprintf("🔮 %s", m.config.agent.Model))
	cwdInfo := statusItemStyle.Render(fmt.Sprintf("📁 %s", cwd))

	// Token usage with color coding and description
	tokenUsage := m.config.agent.GetTokenUsage()
	tokenStyle := statusItemStyle.Copy()
	tokenDescription := "Tokens"

	// Add warning if approaching limits
	if tokenUsage.TotalTokens > 500000 {
		tokenStyle = tokenStyle.Foreground(errorColor)
		tokenDescription = "Tokens (High!)"
	} else if tokenUsage.TotalTokens > 1000000 {
		tokenStyle = tokenStyle.Foreground(warningColor)
		tokenDescription = "Tokens (Moderate)"
	}

	tokenInfo := tokenStyle.Render(fmt.Sprintf("🪙 %s: %d in / %d out",
		tokenDescription, tokenUsage.InputTokens, tokenUsage.OutputTokens))

	// Add help text
	confirmStatus := ""
	if m.config.requireToolConfirmation {
		confirmStatus = " (Confirm: ON)"
	} else {
		confirmStatus = " (Confirm: OFF)"
	}

	thinkingStatus := ""
	if m.config.enableThinkingMode {
		thinkingStatus = " (Think: ON)"
	} else {
		thinkingStatus = " (Think: OFF)"
	}

	var helpInfo string
	if m.ui.toolConfirmationMode {
		helpInfo = lipgloss.NewStyle().
			Foreground(warningColor).
			Bold(true).
			Render("Y: Confirm | N/Esc: Deny")
	} else if m.ui.modelSelectionMode {
		helpInfo = lipgloss.NewStyle().
			Foreground(primaryColor).
			Render("↑↓ Navigate • Enter Select • Esc Cancel")
	} else {
		helpInfo = lipgloss.NewStyle().
			Foreground(textMuted).
			Render(fmt.Sprintf("F2 Model • F3 Confirm%s • F4 Think%s • Ctrl+T Toggle • Ctrl+C Exit", confirmStatus, thinkingStatus))
	}

	// Combine all status items
	leftStatus := lipgloss.JoinHorizontal(
		lipgloss.Top,
		modelInfo,
		cwdInfo,
		tokenInfo,
	)

	// Use the full width and align help text to the right
	fullStatus := lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftStatus,
		lipgloss.NewStyle().
			Width(m.ui.width-lipgloss.Width(leftStatus)-lipgloss.Width(helpInfo)-4).
			Render(" "),
		helpInfo,
	)

	return statusBarStyle.
		Width(m.ui.width).
		Render(fullStatus)
}
