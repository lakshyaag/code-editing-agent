package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"agent/internal/agent"
	"agent/internal/schema"
)

// ReadFileInput defines the input parameters for the read_file tool
type ReadFileInput struct {
	Path      string `json:"path" jsonschema_description:"The relative path of a file in the working directory."`
	StartLine int    `json:"start_line,omitempty" jsonschema_description:"The line number to start reading from (1-indexed). Defaults to 1."`
	EndLine   int    `json:"end_line,omitempty" jsonschema_description:"The line number to end reading at (inclusive). Defaults to reading the whole file."`
	MaxLines  int    `json:"max_lines,omitempty" jsonschema_description:"The maximum number of lines to read. Defaults to 1000."`
}

// ReadFileDefinition provides the read_file tool definition
var ReadFileDefinition = agent.ToolDefinition{
	Name:        "read_file",
	Description: "Read the contents of a given relative file path. Can read the whole file or a specific range of lines. Use this when you want to see what's inside a file. Do not use this with directory names.",
	InputSchema: schema.GenerateSchema[ReadFileInput](),
	Function:    ReadFile,
}

// ReadFile reads the contents of a file
func ReadFile(ctx context.Context, input json.RawMessage) (string, error) {
	var readFileInput ReadFileInput
	err := json.Unmarshal(input, &readFileInput)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal input: %w", err)
	}

	content, err := os.ReadFile(readFileInput.Path)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", readFileInput.Path, err)
	}

	lines := strings.Split(string(content), "\n")
	maxLines := readFileInput.MaxLines
	if maxLines <= 0 {
		maxLines = 1000 // Default max lines
	}

	start := readFileInput.StartLine
	if start <= 0 {
		start = 1
	}

	end := readFileInput.EndLine
	if end <= 0 || end > len(lines) {
		end = len(lines)
	}

	if start > end {
		return "", fmt.Errorf("start line %d is greater than end line %d", start, end)
	}

	if (end - start + 1) > maxLines {
		return "", fmt.Errorf("cannot read more than %d lines at once", maxLines)
	}

	if start > len(lines) {
		return "", fmt.Errorf("start_line (%d) is greater than the total number of lines (%d)", start, len(lines))
	}

	return strings.Join(lines[start-1:end], "\n"), nil
}
