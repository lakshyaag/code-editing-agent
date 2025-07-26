package agent

import (
	"agent/internal/ui"
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/genai"
)

// TUIAgent represents an AI agent that works with the TUI interface
type TUIAgent struct {
	client       *genai.Client
	model        string
	tools        []ToolDefinition
	app          *ui.App
	conversation []*genai.Content
	ctx          context.Context
}

// NewTUIAgent creates a new TUI-aware agent instance
func NewTUIAgent(client *genai.Client, model string, tools []ToolDefinition, app *ui.App, ctx context.Context) *TUIAgent {
	agent := &TUIAgent{
		client:       client,
		model:        model,
		tools:        tools,
		app:          app,
		conversation: make([]*genai.Content, 0),
		ctx:          ctx,
	}

	// Set up UI callbacks
	app.SetOnSendMessage(agent.handleUserMessage)
	app.SetOnModelChanged(agent.handleModelChange)

	return agent
}

// Start initializes the agent and shows welcome message
func (a *TUIAgent) Start() {
	cwd := getCurrentWorkingDirectory()
	welcomeMsg := fmt.Sprintf("Welcome! AI Agent is ready.\nCurrent working directory: %s\nUsing model: %s", cwd, a.model)
	a.app.AddSystemMessage(welcomeMsg)
}

// handleUserMessage processes a user message from the UI
func (a *TUIAgent) handleUserMessage(message string) {
	// Add user message to conversation
	userMessage := &genai.Content{
		Role: "user",
		Parts: []*genai.Part{
			{Text: message},
		},
	}
	a.conversation = append(a.conversation, userMessage)

	// Add user message to UI
	a.app.AddUserMessage(message)

	// Process the message
	go a.processMessage()
}

// handleModelChange updates the agent's model when changed via UI
func (a *TUIAgent) handleModelChange(newModel string) {
	a.model = newModel
	a.app.AddSystemMessage(fmt.Sprintf("Model changed to: %s", newModel))
}

// processMessage handles the AI inference and responses
func (a *TUIAgent) processMessage() {
	response, err := a.runInference()
	if err != nil {
		a.app.AddSystemMessage(fmt.Sprintf("Error: %v", err))
		return
	}

	if len(response.Candidates) == 0 {
		a.app.AddSystemMessage("No response received from AI")
		return
	}

	candidate := response.Candidates[0]

	// Add AI response to conversation
	aiContent := &genai.Content{
		Role:  "model",
		Parts: candidate.Content.Parts,
	}
	a.conversation = append(a.conversation, aiContent)

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
			a.conversation = append(a.conversation, toolContent)
			
			// Continue processing with tool results
			go a.processMessage()
			return
		}
	}

	// Display text response
	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			a.app.AddAssistantMessage(part.Text)
		}
	}
}

// runInference handles the AI inference with tool support
func (a *TUIAgent) runInference() (*genai.GenerateContentResponse, error) {
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

	return a.client.Models.GenerateContent(a.ctx, a.model, a.conversation, config)
}

// executeTool executes a specific tool by name with given arguments
func (a *TUIAgent) executeTool(name string, args map[string]interface{}) string {
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

	// Display tool execution in UI
	a.app.AddToolMessage(name, string(argsJSON), "Executing...")

	result, err := toolDef.Function(argsJSON)
	if err != nil {
		result = fmt.Sprintf("Error executing tool %s: %v", name, err)
	}

	// Update the tool message with the result
	a.app.AddToolMessage(name, string(argsJSON), result)
	
	return result
}

// UpdateModel updates the agent's current model
func (a *TUIAgent) UpdateModel(newModel string) {
	a.model = newModel
}

// ClearConversation clears the conversation history
func (a *TUIAgent) ClearConversation() {
	a.conversation = make([]*genai.Content, 0)
}

// GetConversationLength returns the number of messages in the conversation
func (a *TUIAgent) GetConversationLength() int {
	return len(a.conversation)
}