package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderConversation renders all messages in the conversation
func (m *model) renderConversation() string {
	m.clickableLines = make(map[int]int)
	var lines []string
	var currentLine int

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
	return strings.Join(lines, "\n")
}

// renderUserMessage renders a user message with markdown support
func (m *model) renderUserMessage(msg message) string {
	prefix := "You: "

	// Render markdown content using glamour for user messages too
	renderedMarkdown, err := m.markdownRenderer.Render(msg.content)
	if err != nil {
		// Fallback to simple text wrapping if markdown rendering fails
		wrappedContent := wrapText(msg.content, m.viewport.Width-lipgloss.Width(prefix))
		return lipgloss.JoinHorizontal(lipgloss.Top, userStyle.Render(prefix), wrappedContent)
	} else {
		// Use rendered markdown, ensuring proper width constraints
		maxWidth := m.viewport.Width - lipgloss.Width(prefix)
		// Trim any trailing newlines from the markdown output
		renderedMarkdown = strings.TrimRight(renderedMarkdown, "\n")
		// Apply width constraint to the rendered markdown
		constrainedMarkdown := lipgloss.NewStyle().Width(maxWidth).Render(renderedMarkdown)
		return lipgloss.JoinHorizontal(lipgloss.Top, userStyle.Render(prefix), constrainedMarkdown)
	}
}

// renderAgentMessage renders an agent message with markdown support
func (m *model) renderAgentMessage(msg message) string {
	prefix := "Agent: "

	// Render markdown content using glamour
	renderedMarkdown, err := m.markdownRenderer.Render(msg.content)
	if err != nil {
		// Fallback to simple text wrapping if markdown rendering fails
		wrappedContent := wrapText(msg.content, m.viewport.Width-lipgloss.Width(prefix))
		return lipgloss.JoinHorizontal(lipgloss.Top, agentStyle.Render(prefix), wrappedContent)
	} else {
		// Use rendered markdown, ensuring proper width constraints
		maxWidth := m.viewport.Width - lipgloss.Width(prefix)
		// Trim any trailing newlines from the markdown output
		renderedMarkdown = strings.TrimRight(renderedMarkdown, "\n")
		// Apply width constraint to the rendered markdown
		constrainedMarkdown := lipgloss.NewStyle().Width(maxWidth).Render(renderedMarkdown)
		return lipgloss.JoinHorizontal(lipgloss.Top, agentStyle.Render(prefix), constrainedMarkdown)
	}
}

// renderToolMessage renders a tool call message with modern styling
func (m *model) renderToolMessage(msg message, index int, currentLine *int) string {
	// Parse tool content for better display
	lines := strings.Split(msg.content, "\n")
	toolName := "Tool Call"
	if len(lines) > 0 && strings.Contains(lines[0], "Tool Call:") {
		toolName = strings.TrimPrefix(lines[0], "🔧 Tool Call: ")
	}

	// Get status icons and styling
	statusIcon, _ := getStatusIcon(msg.isError)
	var headerStyle lipgloss.Style
	if msg.isError {
		headerStyle = errorStyle
	} else {
		headerStyle = successStyle
	}

	expandIcon := getExpandCollapseIcon(msg.isCollapsed)
	headerText := fmt.Sprintf("%s %s %s", expandIcon, statusIcon, toolName)
	header := headerStyle.Render(headerText)

	// Make the entire header line clickable
	m.clickableLines[*currentLine] = index

	if msg.isCollapsed {
		return toolBackgroundStyle.Width(m.width).Render(header)
	} else {
		// Format the content as markdown for better structure
		formattedContent := formatToolContent(msg.content)
		renderedContent, err := m.markdownRenderer.Render(formattedContent)
		if err != nil {
			// Fallback to simple text if markdown fails
			renderedContent = msg.content
		} else {
			renderedContent = strings.TrimRight(renderedContent, "\n")
		}

		// Indent the content for a cleaner look
		indentedContent := lipgloss.NewStyle().PaddingLeft(4).Render(renderedContent)

		fullBlock := lipgloss.JoinVertical(lipgloss.Left, header, indentedContent)
		return toolBackgroundStyle.Width(m.width).Render(fullBlock)
	}
}

// renderThoughtMessage renders a thought message with modern styling
func (m *model) renderThoughtMessage(msg message, index int, currentLine *int) string {
	// Get thinking icon and styling
	thinkingIcon := "💭"

	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244")) // Gray color for thoughts

	expandIcon := getExpandCollapseIcon(msg.isCollapsed)
	headerText := fmt.Sprintf("%s %s Thinking", expandIcon, thinkingIcon)
	header := headerStyle.Render(headerText)

	// Make the entire header line clickable
	m.clickableLines[*currentLine] = index

	if msg.isCollapsed {
		return thoughtBackgroundStyle.Width(m.width).Render(header)
	} else {
		// Extract the actual thought content (remove the "💭 Thinking: " prefix if present)
		content := strings.TrimPrefix(msg.content, "💭 Thinking: ")

		// Render the thought content as markdown
		renderedContent, err := m.markdownRenderer.Render(content)
		if err != nil {
			// Fallback to simple text if markdown fails
			renderedContent = content
		} else {
			renderedContent = strings.TrimRight(renderedContent, "\n")
		}

		// Indent the content for a cleaner look
		indentedContent := lipgloss.NewStyle().PaddingLeft(4).Render(renderedContent)

		fullBlock := lipgloss.JoinVertical(lipgloss.Left, header, indentedContent)
		return thoughtBackgroundStyle.Width(m.width).Render(fullBlock)
	}
}

// statusBarView renders the status bar with current information
func (m *model) statusBarView() string {
	if !m.showStatusBar {
		return ""
	}

	cwd, err := os.Getwd()
	if err != nil {
		cwd = "n/a"
	}

	modelInfo := fmt.Sprintf("Model: %s", m.agent.Model)
	cwdInfo := fmt.Sprintf("CWD: %s", cwd)

	// Add token usage information
	tokenUsage := m.agent.GetTokenUsage()
	tokenInfo := fmt.Sprintf("Tokens: %d in, %d out, %d total",
		tokenUsage.InputTokens, tokenUsage.OutputTokens, tokenUsage.TotalTokens)

	// Add help text
	helpInfo := "F2: Model | Ctrl+T: Tools/Thoughts"
	if m.modelSelectionMode {
		helpInfo = "↑/↓: Navigate | Enter: Select | Esc: Cancel"
	}

	status := lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.NewStyle().Padding(0, 1).Render(modelInfo),
		lipgloss.NewStyle().Padding(0, 1).Render(cwdInfo),
		lipgloss.NewStyle().Padding(0, 1).Render(tokenInfo),
		lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("241")).Render(helpInfo),
	)

	return statusBarStyle.Width(m.width).Render(status)
}
