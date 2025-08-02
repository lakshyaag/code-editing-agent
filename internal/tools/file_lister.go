package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

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

// FileNode represents a single file or directory entry in a tree structure.
type FileNode struct {
	Path         string      `json:"path"`
	IsDir        bool        `json:"is_dir"`
	Size         int64       `json:"size,omitempty"`
	LastModified string      `json:"last_modified,omitempty"`
	Children     []*FileNode `json:"children,omitempty"`
}

// ListFilesDefinition provides the list_files tool definition
var ListFilesDefinition = agent.ToolDefinition{
	Name:        "list_files",
	Description: "List files and directories in a tree-like structure for a given relative directory path. Use this to see the contents of a directory. By default, it lists the current directory non-recursively.",
	InputSchema: schema.GenerateSchema[ListFilesInput](),
	Function:    ListFiles,
}

// ListFiles lists files and directories as a tree
func ListFiles(ctx context.Context, input json.RawMessage) (string, error) {
	var listFilesInput ListFilesInput
	err := json.Unmarshal(input, &listFilesInput)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal input: %w", err)
	}

	dir := "."
	if listFilesInput.Path != "" {
		dir = listFilesInput.Path
	}

	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("directory not found: %s", dir)
		}
		return "", fmt.Errorf("failed to stat path %s: %w", dir, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path is not a directory: %s", dir)
	}

	maxDepth := 1
	if listFilesInput.Recursive {
		if listFilesInput.MaxDepth > 0 {
			maxDepth = listFilesInput.MaxDepth
		} else {
			maxDepth = 3 // Default recursive depth
		}
	}

	root := &FileNode{
		Path:         dir,
		IsDir:        true,
		LastModified: info.ModTime().Format(time.RFC3339),
	}

	children, err := listFilesRecursive(dir, 0, maxDepth, listFilesInput.IncludeHidden)
	if err != nil {
		return "", fmt.Errorf("failed to list files: %w", err)
	}
	root.Children = children

	result, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal file list: %w", err)
	}
	return string(result), nil
}

// listFilesRecursive recursively builds a tree of files and directories.
func listFilesRecursive(currentPath string, depth, maxDepth int, includeHidden bool) ([]*FileNode, error) {
	if depth >= maxDepth {
		return nil, nil
	}

	entries, err := os.ReadDir(currentPath)
	if err != nil {
		return nil, err
	}

	var nodes []*FileNode
	for _, entry := range entries {
		name := entry.Name()
		if !includeHidden && strings.HasPrefix(name, ".") {
			continue // skip hidden files/dirs
		}

		info, err := entry.Info()
		if err != nil {
			// Could be a fleeting file, skip it.
			continue
		}

		node := &FileNode{
			Path:         name,
			IsDir:        entry.IsDir(),
			LastModified: info.ModTime().Format(time.RFC3339),
		}

		if !entry.IsDir() {
			node.Size = info.Size()
		}

		if entry.IsDir() {
			children, err := listFilesRecursive(filepath.Join(currentPath, name), depth+1, maxDepth, includeHidden)
			if err != nil {
				return nil, err
			}
			if children != nil {
				node.Children = children
			}
		}
		nodes = append(nodes, node)
	}

	sort.Slice(nodes, func(i, j int) bool {
		// Sort by type (directories first), then by name
		if nodes[i].IsDir != nodes[j].IsDir {
			return nodes[i].IsDir
		}
		return nodes[i].Path < nodes[j].Path
	})

	return nodes, nil
}
