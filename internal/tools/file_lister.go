package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"agent/internal/agent"
	"agent/internal/schema"
)

// ListFilesInput defines the input parameters for the list_files tool
type ListFilesInput struct {
	Path string `json:"path,omitempty" jsonschema_description:"Optional relative path to list files from. Defaults to current directory if not provided."`
}

// FileNode represents a file or directory in the tree structure
type FileNode struct {
	Name     string     `json:"name"`
	IsDir    bool       `json:"is_dir"`
	Children []FileNode `json:"children,omitempty"`
}

// ListFilesDefinition provides the list_files tool definition
var ListFilesDefinition = agent.ToolDefinition{
	Name:        "list_files",
	Description: "List files and directories in a given relative directory path. Use this to see the contents of a directory.",
	InputSchema: schema.GenerateSchema[ListFilesInput](),
	Function:    ListFiles,
}

// ListFiles lists files and directories in a tree structure
func ListFiles(input json.RawMessage) (string, error) {
	var listFilesInput ListFilesInput
	err := json.Unmarshal(input, &listFilesInput)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal input: %w", err)
	}

	dir := "."
	if listFilesInput.Path != "" {
		dir = listFilesInput.Path
	}

	tree, err := buildFileTree(dir, 1)
	if err != nil {
		return "", fmt.Errorf("failed to build file tree: %w", err)
	}

	result, err := json.Marshal(tree)
	if err != nil {
		return "", fmt.Errorf("failed to marshal file tree: %w", err)
	}
	return string(result), nil
}

// buildFileTree recursively builds a file tree structure
func buildFileTree(currentPath string, depth int) ([]FileNode, error) {
	if depth > 3 {
		return nil, nil
	}

	entries, err := os.ReadDir(currentPath)
	if err != nil {
		return nil, err
	}

	var nodes []FileNode
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue // skip hidden files/dirs
		}

		fullPath := filepath.Join(currentPath, name)
		node := FileNode{
			Name:  name,
			IsDir: entry.IsDir(),
		}

		if entry.IsDir() && depth < 3 {
			children, err := buildFileTree(fullPath, depth+1)
			if err != nil {
				return nil, err
			}
			node.Children = children
		}

		nodes = append(nodes, node)
	}
	return nodes, nil
}
