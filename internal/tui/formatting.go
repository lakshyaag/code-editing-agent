package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// wrapText wraps text to a specified width using lipgloss
func wrapText(text string, width int) string {
	return lipgloss.NewStyle().Width(width).Render(text)
}

// formatToolContent converts raw tool call content into structured markdown
func formatToolContent(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) < 3 {
		return content // Not enough content to format
	}

	var arguments, result string
	var inResult bool

	for i, line := range lines {
		if strings.HasPrefix(line, "Arguments:") {
			arguments = strings.TrimPrefix(line, "Arguments: ")
		} else if strings.HasPrefix(line, "Result:") {
			result = strings.TrimPrefix(line, "Result: ")
			inResult = true
		} else if inResult && i > 0 {
			// Multi-line result content
			result += "\n" + line
		}
	}

	// Build lightweight, streamlined markdown
	var formatted strings.Builder

	// Arguments section
	formatted.WriteString("**Arguments:**\n")
	if arguments != "" && arguments != "{}" {
		formatted.WriteString("```json\n")
		formatted.WriteString(arguments)
		formatted.WriteString("\n```\n")
	} else {
		formatted.WriteString("`None`\n")
	}

	// Result section with smart formatting
	formatted.WriteString("\n**Result:**\n")
	if result != "" {
		if isStructuredData(result) {
			formatted.WriteString("```json\n")
			formatted.WriteString(result)
			formatted.WriteString("\n```\n")
		} else if strings.Contains(result, "Error:") || strings.Contains(result, "error:") {
			formatted.WriteString("```\n")
			formatted.WriteString(result)
			formatted.WriteString("\n```\n")
		} else {
			formatted.WriteString(result)
		}
	} else {
		formatted.WriteString("`No output`\n")
	}

	return formatted.String()
}

// isStructuredData checks if the result contains structured data
func isStructuredData(result string) bool {
	return (strings.HasPrefix(result, "{") && strings.HasSuffix(result, "}")) ||
		(strings.HasPrefix(result, "[") && strings.HasSuffix(result, "]"))
}
