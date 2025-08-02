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

// EditFileInput defines the input parameters for the edit_file tool
type EditFileInput struct {
	Path   string `json:"path" jsonschema_description:"The path to the file"`
	OldStr string `json:"old_str" jsonschema_description:"Text to search for. All occurrences will be replaced."`
	NewStr string `json:"new_str" jsonschema_description:"Text to replace old_str with"`
}

// EditFileDefinition provides the edit_file tool definition
var EditFileDefinition = agent.ToolDefinition{
	Name: "edit_file",
	Description: `Make edits to a text file.

Replaces ALL occurrences of 'old_str' with 'new_str' in the given file. 'old_str' and 'new_str' MUST be different from each other.

The file MUST exist. This tool cannot be used to create new files.
`,
	InputSchema: schema.GenerateSchema[EditFileInput](),
	Function:    EditFile,
}

// EditFile edits a file by replacing old_str with new_str
func EditFile(ctx context.Context, input json.RawMessage) (string, error) {
	var editFileInput EditFileInput
	err := json.Unmarshal(input, &editFileInput)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal input: %w", err)
	}

	if editFileInput.Path == "" || editFileInput.OldStr == "" || editFileInput.OldStr == editFileInput.NewStr {
		return "", fmt.Errorf("invalid input parameters: path and old_str must be non-empty, and old_str must be different from new_str")
	}

	content, err := os.ReadFile(editFileInput.Path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	oldContent := string(content)
	replacements := strings.Count(oldContent, editFileInput.OldStr)
	if replacements == 0 {
		return "No occurrences of `old_str` found. No changes made to the file.", nil
	}

	newContent := strings.ReplaceAll(oldContent, editFileInput.OldStr, editFileInput.NewStr)

	err = os.WriteFile(editFileInput.Path, []byte(newContent), 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return fmt.Sprintf("OK. Edited file successfully. Made %d replacement(s).", replacements), nil
}
