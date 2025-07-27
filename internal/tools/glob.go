package tools

import (
	"agent/internal/agent"
	"agent/internal/schema"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GlobInput represents the input parameters for the glob tool
type GlobInput struct {
	Pattern string `json:"pattern" description:"Glob pattern to match files (e.g., '*.go' for all Go files, '**/*.txt' for all text files recursively)"`
	Path    string `json:"path,omitempty" description:"Base path to search from (defaults to current directory)"`
}

// GlobDefinition provides the glob tool definition
var GlobDefinition = agent.ToolDefinition{
	Name:        "glob",
	Description: "Find files matching a glob pattern (e.g., '*.go', '**/*.txt'). Supports recursive patterns with **.",
	InputSchema: schema.GenerateSchema[GlobInput](),
	Function:    Glob,
}

// Glob finds files matching a pattern
func Glob(input json.RawMessage) (string, error) {
	var params GlobInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	if params.Pattern == "" {
		return "", fmt.Errorf("pattern is required")
	}

	basePath := params.Path
	if basePath == "" {
		basePath = "."
	}

	// Convert ** to filepath walking pattern
	if strings.Contains(params.Pattern, "**") {
		return walkPattern(basePath, params.Pattern)
	}

	// Simple glob pattern
	pattern := filepath.Join(basePath, params.Pattern)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("failed to glob pattern: %w", err)
	}

	// Convert to relative paths and format output
	var result []string
	for _, match := range matches {
		relPath, err := filepath.Rel(".", match)
		if err != nil {
			result = append(result, match)
		} else {
			result = append(result, relPath)
		}
	}

	if len(result) == 0 {
		return "No files found matching pattern: " + params.Pattern, nil
	}

	return formatFileList(result), nil
}

func walkPattern(basePath, pattern string) (string, error) {
	// Split pattern by ** to handle recursive matching
	parts := strings.Split(pattern, "**")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid pattern with **: %s", pattern)
	}

	prefix := strings.TrimSuffix(parts[0], "/")
	suffix := strings.TrimPrefix(parts[1], "/")

	var matches []string
	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		relPath, err := filepath.Rel(basePath, path)
		if err != nil {
			return nil
		}

		// Check if path matches the pattern
		if matchesRecursivePattern(relPath, prefix, suffix) {
			matches = append(matches, relPath)
		}

		return nil
	})

	if err != nil {
		return "", fmt.Errorf("failed to walk directory: %w", err)
	}

	if len(matches) == 0 {
		return "No files found matching pattern: " + pattern, nil
	}

	return formatFileList(matches), nil
}

func matchesRecursivePattern(path, prefix, suffix string) bool {
	// Normalize path separators
	path = filepath.ToSlash(path)

	// Check prefix if not empty
	if prefix != "" && !strings.HasPrefix(path, prefix) {
		return false
	}

	// Check suffix
	if suffix != "" {
		matched, _ := filepath.Match(suffix, filepath.Base(path))
		return matched
	}

	return true
}

func formatFileList(files []string) string {
	if len(files) == 0 {
		return "No files found"
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Found %d file(s):\n", len(files)))
	for _, file := range files {
		result.WriteString(fmt.Sprintf("- %s\n", file))
	}
	return strings.TrimSpace(result.String())
}
