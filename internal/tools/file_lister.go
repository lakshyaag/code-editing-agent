package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"agent/internal/agent"
	"agent/internal/schema"
)

// ListFilesInput defines the input parameters for the list_files tool
type ListFilesInput struct {
	Path          string `json:"path,omitempty" jsonschema_description:"Optional relative path to list files from. Defaults to current directory if not provided."`
	Recursive     bool   `json:"recursive,omitempty" jsonschema_description:"Whether to list files recursively. Defaults to false."`
	MaxDepth      int    `json:"max_depth,omitempty" jsonschema_description:"Maximum recursion depth. Only used if recursive is true. Defaults to 3."`
	IncludeHidden bool   `json:"include_hidden,omitempty" jsonschema_description:"Whether to include hidden files and directories (those starting with a dot). Defaults to false."`
}

// FileNode represents a single file or directory entry.
type FileNode struct {
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
}

// ListFilesDefinition provides the list_files tool definition
var ListFilesDefinition = agent.ToolDefinition{
	Name:        "list_files",
	Description: "List files and directories in a given relative directory path. Use this to see the contents of a directory. By default, it lists the current directory non-recursively.",
	InputSchema: schema.GenerateSchema[ListFilesInput](),
	Function:    ListFiles,
}

// ListFiles lists files and directories as a flat list
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

	maxDepth := 1
	if listFilesInput.Recursive {
		if listFilesInput.MaxDepth > 0 {
			maxDepth = listFilesInput.MaxDepth
		} else {
			maxDepth = 3 // Default recursive depth
		}
	}

	files, err := listFilesRecursive(dir, 0, maxDepth, listFilesInput.IncludeHidden)
	if err != nil {
		return "", fmt.Errorf("failed to list files: %w", err)
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})

	result, err := json.MarshalIndent(files, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal file list: %w", err)
	}
	return string(result), nil
}

// listFilesRecursive recursively builds a flat list of files and directories.
func listFilesRecursive(currentPath string, depth, maxDepth int, includeHidden bool) ([]FileNode, error) {
	if depth >= maxDepth {
		return nil, nil
	}

	entries, err := os.ReadDir(currentPath)
	if err != nil {
		return nil, err
	}

	var nodes []FileNode
	for _, entry := range entries {
		name := entry.Name()
		if !includeHidden && strings.HasPrefix(name, ".") {
			continue // skip hidden files/dirs
		}

		fullPath := filepath.Join(currentPath, name)
		node := FileNode{
			Path:  fullPath,
			IsDir: entry.IsDir(),
		}
		nodes = append(nodes, node)

		if entry.IsDir() {
			children, err := listFilesRecursive(fullPath, depth+1, maxDepth, includeHidden)
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, children...)
		}
	}
	return nodes, nil
}
