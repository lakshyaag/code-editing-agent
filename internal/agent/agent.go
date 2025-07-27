package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/genai"
)

type (
	MessageType int
	Message     struct {
		Type    MessageType
		Content string
		IsError bool
	}
)

const (
	UserMessage MessageType = iota
	AgentMessage
	ToolMessage
)

// Agent represents the main AI agent that can execute tools
type Agent struct {
	client       *genai.Client
	Model        string
	tools        []ToolDefinition
	Conversation []*genai.Content
}

// ToolDefinition defines the structure for a tool that the agent can use
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
	Function    func(input json.RawMessage) (string, error)
}

// New creates a new Agent instance
func New(client *genai.Client, model string, tools []ToolDefinition) *Agent {
	return &Agent{
		client: client,
		Model:  model,
		tools:  tools,
	}
}

// ProcessMessage handles a single user message and returns the agent's response
func (a *Agent) ProcessMessage(ctx context.Context, userInput string) ([]Message, error) {
	messages := []Message{}
	userMessageContent := &genai.Content{
		Role: "user",
		Parts: []*genai.Part{
			{Text: userInput},
		},
	}
	a.Conversation = append(a.Conversation, userMessageContent)

	for {
		response, err := a.runInference(ctx, a.Conversation)
		if err != nil {
			return nil, err
		}

		if len(response.Candidates) == 0 {
			return nil, fmt.Errorf("no response candidates from Gemini")
		}

		candidate := response.Candidates[0]

		// Add AI response to conversation
		aiContent := &genai.Content{
			Role:  "model",
			Parts: candidate.Content.Parts,
		}
		a.Conversation = append(a.Conversation, aiContent)

		// Check for function calls
		if len(candidate.Content.Parts) > 0 {
			hasToolCalls := false
			var toolResults []*genai.Part

			for _, part := range candidate.Content.Parts {
				if part.FunctionCall != nil {
					hasToolCalls = true
					result, isError := a.executeTool(part.FunctionCall.Name, part.FunctionCall.Args)
					messages = append(messages, Message{Type: ToolMessage, Content: result, IsError: isError})
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
				a.Conversation = append(a.Conversation, toolContent)
				continue
			}
		}

		// Return text response
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				messages = append(messages, Message{Type: AgentMessage, Content: part.Text})
				return messages, nil
			}
		}
		return messages, nil // Should not be reached
	}
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

	return a.client.Models.GenerateContent(ctx, a.Model, conversation, config)
}

// executeTool executes a specific tool by name with given arguments
func (a *Agent) executeTool(name string, args map[string]interface{}) (string, bool) {
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
		return fmt.Sprintf("Tool %s not found", name), true
	}

	// Convert args to JSON
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return fmt.Sprintf("Error marshaling arguments: %v", err), true
	}

	fmt.Printf("\u001b[92mtool\u001b[0m: %s(%s)\n", name, string(argsJSON))

	result, err := toolDef.Function(argsJSON)
	if err != nil {
		result = fmt.Sprintf("Error executing tool %s: %v", name, err)
		return result, true
	}

	fmt.Printf("\u001b[92mresult\u001b[0m: %s\n", result)
	return result, false
}
