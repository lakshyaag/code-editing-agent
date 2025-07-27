package tools

import "agent/internal/agent"

// GetAllTools returns all available tools for the agent
func GetAllTools() []agent.ToolDefinition {
	return []agent.ToolDefinition{
		ReadFileDefinition,
		ListFilesDefinition,
		EditFileDefinition,
		WriteFileDefinition,
		SearchFileDefinition,
		RunShellCommandDefinition,
	}
}
