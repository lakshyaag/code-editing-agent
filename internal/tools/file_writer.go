package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path"

	"agent/internal/agent"
	"agent/internal/schema"
)

// WriteFileInput defines the input parameters for the write_file tool
type WriteFileInput struct {
	Path    string `json:"path" jsonschema_description:"The relative path of the file to write to."`
	Content string `json:"content" jsonschema_description:"The content to write to the file."`
	Append  bool   `json:"append,omitempty" jsonschema_description:"If true, appends the content to the file. If false (default), overwrites the file."`
}

// WriteFileDefinition provides the write_file tool definition
var WriteFileDefinition = agent.ToolDefinition{
	Name: "write_file",
	Description: `Write content to a file.
This tool can create a new file, overwrite an existing file, or append to an existing file.
Use the 'append' parameter to control the behavior. By default, it overwrites.
`,
	InputSchema: schema.GenerateSchema[WriteFileInput](),
	Function:    WriteFile,
}

// WriteFile writes content to a file, with options to overwrite or append.
func WriteFile(input json.RawMessage) (string, error) {
	var writeFileInput WriteFileInput
	err := json.Unmarshal(input, &writeFileInput)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal input: %w", err)
	}

	if writeFileInput.Path == "" {
		return "", fmt.Errorf("path cannot be empty")
	}

	dir := path.Dir(writeFileInput.Path)
	if dir != "." && dir != "/" {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return "", fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	if writeFileInput.Append {
		return appendToFile(writeFileInput.Path, writeFileInput.Content)
	}

	return createOrOverwriteFile(writeFileInput.Path, writeFileInput.Content)
}

func createOrOverwriteFile(filePath, content string) (string, error) {
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write to file %s: %w", filePath, err)
	}
	return fmt.Sprintf("File %s written successfully.", filePath), nil
}

func appendToFile(filePath, content string) (string, error) {
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to open file for appending: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(content); err != nil {
		return "", fmt.Errorf("failed to append to file %s: %w", filePath, err)
	}

	return fmt.Sprintf("Content appended to file %s successfully.", filePath), nil
}
