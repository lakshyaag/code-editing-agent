package tui

import (
	"strings"
)

// formatToolContent converts raw tool call content into structured markdown
func formatToolContent(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) < 3 {
		return content
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
			result += "\n" + line
		}
	}

	// Build markdown
	var formatted strings.Builder
	formatted.WriteString("**Arguments:**\n")
	
	if arguments != "" && arguments != "{}" {
		formatted.WriteString("```json\n" + arguments + "\n```\n")
	} else {
		formatted.WriteString("`None`\n")
	}

	formatted.WriteString("\n**Result:**\n")
	if result != "" {
		// Detect if it's JSON-like data
		isJSON := (strings.HasPrefix(result, "{") && strings.HasSuffix(result, "}")) ||
			(strings.HasPrefix(result, "[") && strings.HasSuffix(result, "]"))
		
		if isJSON {
			formatted.WriteString("```json\n" + result + "\n```\n")
		} else if strings.Contains(result, "Error:") || strings.Contains(result, "error:") {
			formatted.WriteString("```\n" + result + "\n```\n")
		} else {
			formatted.WriteString(result)
		}
	} else {
		formatted.WriteString("`No output`\n")
	}

	return formatted.String()
}
