package tools

import (
	"encoding/json"
	"fmt"
	"os"

	"agent/internal/agent"
	"agent/internal/schema"
)

// ReadFileInput defines the input parameters for the read_file tool
type ReadFileInput struct {
	Path string `json:"path" jsonschema_description:"The relative path of a file in the working directory."`
}

// ReadFileDefinition provides the read_file tool definition
var ReadFileDefinition = agent.ToolDefinition{
	Name:        "read_file",
	Description: "Read the contents of a given relative file path. Use this when you want to see what's inside a file. Do not use this with directory names.",
	InputSchema: schema.GenerateSchema[ReadFileInput](),
	Function:    ReadFile,
}

// ReadFile reads the contents of a file and returns it as a string
func ReadFile(input json.RawMessage) (string, error) {
	var readFileInput ReadFileInput
	err := json.Unmarshal(input, &readFileInput)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal input: %w", err)
	}

	content, err := os.ReadFile(readFileInput.Path)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", readFileInput.Path, err)
	}

	return string(content), nil
}
