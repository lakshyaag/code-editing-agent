package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"agent/internal/config"
	"github.com/charmbracelet/lipgloss"
)

// renderConversation renders all messages in the conversation
func (m *model) renderConversation() string {
	m.ui.clickableLines = make(map[int]int)
	var lines []string
	var currentLine int

	// Welcome header if no messages
	if len(m.messages) == 0 {
		welcomeHeader := m.renderWelcomeHeader()
		lines = append(lines, welcomeHeader, "")
		currentLine += lipgloss.Height(welcomeHeader) + 1
	}

	// Render messages
	for i, msg := range m.messages {
		var renderedBlock string
		switch msg.mType {
		case userMessage:
			renderedBlock = m.renderUserMessage(msg)
		case agentMessage:
			renderedBlock = m.renderAgentMessage(msg)
		case toolMessage, thoughtMessage:
			renderedBlock = m.renderCollapsibleMessage(msg, i, &currentLine)
		}
		lines = append(lines, renderedBlock)
		currentLine += lipgloss.Height(renderedBlock)
	}

	return strings.Join(lines, "\n")
}

// renderUserMessage renders a user message
func (m *model) renderUserMessage(msg message) string {
	header := labelStyle.Copy().
		Foreground(primaryColor).
		Render(userIcon + " You")

	content := m.renderMarkdown(msg.content)
	
	return cardStyle.Copy().
		BorderForeground(primaryColor).
		Width(m.ui.viewport.Width - 4).
		Render(header + "\n" + content)
}

// renderAgentMessage renders an agent message
func (m *model) renderAgentMessage(msg message) string {
	header := labelStyle.Copy().
		Foreground(secondaryColor).
		Render(agentIcon + " Assistant")

	if msg.isStreaming {
		header += lipgloss.NewStyle().
			Foreground(secondaryColor).
			Blink(true).
			Render(" ●")
	}

	content := m.renderMarkdown(msg.content)
	
	return cardStyle.Copy().
		BorderForeground(secondaryColor).
		Width(m.ui.viewport.Width - 4).
		Render(header + "\n" + content)
}

// renderCollapsibleMessage renders tool or thought messages with collapse functionality
func (m *model) renderCollapsibleMessage(msg message, index int, currentLine *int) string {
	// Determine icon and header text
	icon := toolIcon
	headerText := "Tool Call"
	isThought := msg.mType == thoughtMessage
	
	if isThought {
		icon = thoughtIcon
		headerText = "Thinking..."
	} else if strings.Contains(msg.content, "Tool Call:") {
		lines := strings.Split(msg.content, "\n")
		if len(lines) > 0 {
			headerText = strings.TrimPrefix(lines[0], "🔧 Tool Call: ")
		}
	}

	// Create header
	eIcon := collapseIcon
	if !msg.isCollapsed {
		eIcon = expandIcon
	}
	
	statusIcon := ""
	if !isThought && msg.isError {
		statusIcon = "✗ "
	} else if !isThought && !msg.isError {
		statusIcon = "✓ "
	}

	headerContent := fmt.Sprintf("%s %s %s%s", eIcon, icon, statusIcon, headerText)
	
	headerStyle := collapsibleHeaderStyle.Copy()
	if msg.isError {
		headerStyle = headerStyle.Foreground(errorColor)
	} else if !isThought {
		headerStyle = headerStyle.Foreground(accentColor)
	} else {
		headerStyle = headerStyle.Foreground(textMuted).Italic(true)
	}

	header := headerStyle.
		Width(m.ui.viewport.Width - 6).
		Render(headerContent)

	// Make header clickable
	m.ui.clickableLines[*currentLine] = index

	cardStyleToUse := collapsibleCardStyle.Copy()
	if isThought {
		cardStyleToUse = cardStyleToUse.BorderStyle(lipgloss.DoubleBorder())
	}

	if msg.isCollapsed {
		return cardStyleToUse.
			Width(m.ui.viewport.Width - 4).
			Render(header)
	}

	// Render expanded content
	var content string
	if isThought {
		content = strings.TrimPrefix(msg.content, "💭 Thinking: ")
		content = m.renderMarkdown(content)
	} else {
		content = m.renderMarkdown(formatToolContent(msg.content))
	}

	contentStyle := collapsibleContentStyle.Copy()
	if isThought {
		contentStyle = contentStyle.Italic(true)
	}

	styledContent := contentStyle.
		Width(m.ui.viewport.Width - 10).
		Render(content)

	return cardStyleToUse.
		Width(m.ui.viewport.Width - 4).
		Render(header + "\n" + styledContent)
}

// renderMarkdown renders markdown content
func (m *model) renderMarkdown(content string) string {
	if m.config.markdownRenderer == nil {
		return content
	}
	
	rendered, err := m.config.markdownRenderer.Render(content)
	if err != nil {
		return content
	}
	return strings.TrimRight(rendered, "\n")
}

// renderWelcomeHeader renders the welcome message header
func (m *model) renderWelcomeHeader() string {
	header := lipgloss.NewStyle().
		Foreground(accentColor).
		Bold(true).
		Render("🎉 Welcome to CLI Code Assistant")

	welcomeContent := fmt.Sprintf(config.WelcomeMessage, len(config.SystemPrompt))
	content := lipgloss.NewStyle().
		Foreground(textMuted).
		Render(welcomeContent)

	return cardStyle.Copy().
		BorderForeground(accentColor).
		BorderStyle(lipgloss.DoubleBorder()).
		Width(m.ui.viewport.Width - 4).
		Render(header + "\n\n" + content)
}

// statusBarView renders the status bar
func (m *model) statusBarView() string {
	if !m.ui.showStatusBar {
		return ""
	}

	// Get current working directory
	cwd, _ := os.Getwd()
	if len(cwd) > 30 {
		cwd = "..." + cwd[len(cwd)-27:]
	}

	// Build status items
	items := []string{
		fmt.Sprintf("🔮 %s", m.config.agent.Model),
		fmt.Sprintf("📁 %s", cwd),
	}

	// Token usage
	tokenUsage := m.config.agent.GetTokenUsage()
	tokenText := fmt.Sprintf("🪙 %d/%d", tokenUsage.InputTokens, tokenUsage.OutputTokens)
	if tokenUsage.TotalTokens > 500000 {
		tokenText = lipgloss.NewStyle().Foreground(errorColor).Render(tokenText)
	}
	items = append(items, tokenText)

	// Help text based on mode
	var helpText string
	if m.ui.toolConfirmationMode {
		helpText = lipgloss.NewStyle().
			Foreground(warningColor).
			Bold(true).
			Render("Y: Confirm | N/Esc: Deny")
	} else if m.ui.modelSelectionMode {
		helpText = "↑↓ Navigate • Enter Select • Esc Cancel"
	} else {
		confirmStatus := "OFF"
		if m.config.requireToolConfirmation {
			confirmStatus = "ON"
		}
		thinkStatus := "OFF"
		if m.config.enableThinkingMode {
			thinkStatus = "ON"
		}
		helpText = fmt.Sprintf("F2 Model • F3 Confirm:%s • F4 Think:%s • Ctrl+C Exit", confirmStatus, thinkStatus)
	}

	// Join items
	leftStatus := lipgloss.NewStyle().
		Foreground(textMuted).
		Render(strings.Join(items, " • "))

	// Calculate spacing
	spacerWidth := m.ui.width - lipgloss.Width(leftStatus) - lipgloss.Width(helpText) - 4
	if spacerWidth < 0 {
		spacerWidth = 1
	}
	spacer := strings.Repeat(" ", spacerWidth)

	return statusBarStyle.
		Width(m.ui.width).
		Render(leftStatus + spacer + helpText)
}

// renderModelSelector renders the model selection overlay
func (m *model) renderModelSelector(background string) string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryColor).
		MarginBottom(2).
		Render("🔮 Select AI Model")

	// Build model list
	var modelItems []string
	for i, modelName := range m.config.availableModels {
		prefix := "  "
		if modelName == m.config.agent.Model {
			prefix = "• "
		}

		style := normalItemStyle
		if i == m.ui.selectedModelIndex {
			style = selectedItemStyle
		}

		// Add capability hints
		display := modelName
		if strings.Contains(modelName, "pro") {
			display += " (Advanced)"
		} else if strings.Contains(modelName, "flash-lite") {
			display += " (Fast & Light)"
		} else if strings.Contains(modelName, "flash") {
			display += " (Fast)"
		}

		modelItems = append(modelItems, style.Render(prefix+display))
	}

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		title,
		strings.Join(modelItems, "\n"),
		"\n↑/↓ Navigate • Enter Select • Esc Cancel",
	)

	return lipgloss.Place(
		m.ui.width, m.ui.height,
		lipgloss.Center, lipgloss.Center,
		modalStyle.Copy().
			BorderForeground(primaryColor).
			Width(50).
			Render(content),
	)
}

// renderToolConfirmation renders the tool confirmation overlay
func (m *model) renderToolConfirmation(background string) string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(warningColor).
		Align(lipgloss.Center).
		Render("⚠️  Tool Execution Request")

	// Tool info
	toolInfo := fmt.Sprintf("Tool: %s\n\nArguments:\n", m.ui.toolConfirmationName)
	argsJSON, _ := json.MarshalIndent(m.ui.toolConfirmationArgs, "", "  ")
	
	argsBox := lipgloss.NewStyle().
		Foreground(secondaryColor).
		Background(bgDark).
		Padding(1).
		Border(lipgloss.NormalBorder()).
		BorderForeground(bgLight).
		Render(string(argsJSON))

	// Buttons
	buttons := lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.NewStyle().Background(accentColor).Foreground(bgDark).Bold(true).Padding(0, 2).Render("Y - Yes"),
		"  ",
		lipgloss.NewStyle().Background(errorColor).Foreground(textPrimary).Bold(true).Padding(0, 2).Render("N - No"),
		"  ",
		lipgloss.NewStyle().Background(bgLight).Foreground(textPrimary).Padding(0, 2).Render("Esc - Cancel"),
	)

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		title,
		"\n"+toolInfo,
		argsBox,
		"\nDo you want to execute this tool?\n",
		buttons,
		"\n🔒 Tool execution requires your permission",
	)

	return lipgloss.Place(
		m.ui.width, m.ui.height,
		lipgloss.Center, lipgloss.Center,
		modalStyle.Copy().
			BorderForeground(warningColor).
			Width(60).
			Render(content),
	)
}
