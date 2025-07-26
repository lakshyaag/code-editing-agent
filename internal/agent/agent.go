package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"google.golang.org/genai"
)

// Agent represents the main AI agent that can execute tools
type Agent struct {
	client         *genai.Client
	model          string
	getUserMessage func() (string, bool)
	tools          []ToolDefinition
}

// ToolDefinition defines the structure for a tool that the agent can use
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
	Function    func(input json.RawMessage) (string, error)
}

// New creates a new Agent instance
func New(client *genai.Client, model string, getUserMessage func() (string, bool), tools []ToolDefinition) *Agent {
	return &Agent{
		client:         client,
		model:          model,
		getUserMessage: getUserMessage,
		tools:          tools,
	}
}

// Run starts the main conversation loop
func (a *Agent) Run(ctx context.Context) error {
	conversation := []*genai.Content{}

	fmt.Printf("Chat with CLI\n")
	cwd := getCurrentWorkingDirectory()
	fmt.Printf("Current working directory: %s\n", cwd)
	fmt.Printf("Using model: %s\n", a.model)

	readUserInput := true
	for {
		if readUserInput {
			fmt.Print("\u001b[94mYou\u001b[0m: ")
			userInput, ok := a.getUserMessage()

			if !ok {
				break
			}

			userMessage := &genai.Content{
				Role: "user",
				Parts: []*genai.Part{
					{Text: userInput},
				},
			}
			conversation = append(conversation, userMessage)
		}

		response, err := a.runInference(ctx, conversation)
		if err != nil {
			return err
		}

		if len(response.Candidates) == 0 {
			return fmt.Errorf("no response candidates from Gemini")
		}

		candidate := response.Candidates[0]

		// Add AI response to conversation
		aiContent := &genai.Content{
			Role:  "model",
			Parts: candidate.Content.Parts,
		}
		conversation = append(conversation, aiContent)

		// Check for function calls
		if len(candidate.Content.Parts) > 0 {
			hasToolCalls := false
			var toolResults []*genai.Part

			for _, part := range candidate.Content.Parts {
				if part.FunctionCall != nil {
					hasToolCalls = true
					result := a.executeTool(part.FunctionCall.Name, part.FunctionCall.Args)
					toolResults = append(toolResults, &genai.Part{
						FunctionResponse: &genai.FunctionResponse{
							Name:     part.FunctionCall.Name,
							Response: map[string]interface{}{"result": result},
						},
					})
				}
			}

			if hasToolCalls {
				// Add tool results to conversation
				toolContent := &genai.Content{
					Role:  "user",
					Parts: toolResults,
				}
				conversation = append(conversation, toolContent)
				readUserInput = false
				continue
			}
		}

		// Display text response
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				fmt.Printf("\u001b[94mCodeAgent\u001b[0m: %s\n", part.Text)
			}
		}
		readUserInput = true
	}
	return nil
}

// runInference handles the AI inference with tool support
func (a *Agent) runInference(ctx context.Context, conversation []*genai.Content) (*genai.GenerateContentResponse, error) {
	// Convert tool definitions to Gemini function declarations
	var functions []*genai.FunctionDeclaration
	for _, tool := range a.tools {
		// Convert map[string]interface{} to genai.Schema
		schemaBytes, err := json.Marshal(tool.InputSchema)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal schema: %v", err)
		}

		var schema genai.Schema
		if err := json.Unmarshal(schemaBytes, &schema); err != nil {
			return nil, fmt.Errorf("failed to unmarshal schema: %v", err)
		}

		functions = append(functions, &genai.FunctionDeclaration{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  &schema,
		})
	}

	// Configure the generation with tools
	config := &genai.GenerateContentConfig{
		Tools: []*genai.Tool{
			{
				FunctionDeclarations: functions,
			},
		},
		MaxOutputTokens: 1024,
	}

	return a.client.Models.GenerateContent(ctx, a.model, conversation, config)
}

// executeTool executes a specific tool by name with given arguments
func (a *Agent) executeTool(name string, args map[string]interface{}) string {
	var toolDef ToolDefinition
	var found bool

	for _, tool := range a.tools {
		if tool.Name == name {
			toolDef = tool
			found = true
			break
		}
	}
	if !found {
		return fmt.Sprintf("Tool %s not found", name)
	}

	// Convert args to JSON
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return fmt.Sprintf("Error marshaling arguments: %v", err)
	}

	fmt.Printf("\u001b[92mtool\u001b[0m: %s(%s)\n", name, string(argsJSON))

	result, err := toolDef.Function(argsJSON)
	if err != nil {
		result = fmt.Sprintf("Error executing tool %s: %v", name, err)
	}

	fmt.Printf("\u001b[92mresult\u001b[0m: %s\n", result)
	return result
}

// getCurrentWorkingDirectory returns the current working directory or a default message
func getCurrentWorkingDirectory() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "Failed to get current working directory"
	}
	return cwd
}
