package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"

	"google.golang.org/genai"
)

type (
	MessageType int
	Message     struct {
		Type     MessageType
		Content  string
		IsError  bool
		IsStream bool
	}

	// TokenUsage tracks token consumption for a conversation
	TokenUsage struct {
		InputTokens  int
		OutputTokens int
		TotalTokens  int
	}

	// StreamingCallback is called for each chunk of streaming content
	StreamingCallback func(chunk string) error

	// StreamingMessageCallback is called for any message during streaming (chunks, tool calls, etc.)
	StreamingMessageCallback func(msg Message) error
)

const (
	UserMessage MessageType = iota
	AgentMessage
	ToolMessage
	StreamChunk
	TokenInfo
)

// Agent represents the main AI agent that can execute tools
type Agent struct {
	client       *genai.Client
	Model        string
	tools        []ToolDefinition
	Conversation []*genai.Content
	TokenUsage   TokenUsage
	functions    []*genai.FunctionDeclaration // Pre-computed function declarations
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
	agent := &Agent{
		client: client,
		Model:  model,
		tools:  tools,
	}

	// Pre-compute function declarations for efficiency
	agent.precomputeFunctionDeclarations()

	return agent
}

// precomputeFunctionDeclarations converts tool definitions to Gemini function declarations once
func (a *Agent) precomputeFunctionDeclarations() error {
	var functions []*genai.FunctionDeclaration
	for _, tool := range a.tools {
		// Convert map[string]interface{} to genai.Schema
		schemaBytes, err := json.Marshal(tool.InputSchema)
		if err != nil {
			return fmt.Errorf("failed to marshal schema for tool %s: %v", tool.Name, err)
		}

		var schema genai.Schema
		if err := json.Unmarshal(schemaBytes, &schema); err != nil {
			return fmt.Errorf("failed to unmarshal schema for tool %s: %v", tool.Name, err)
		}

		functions = append(functions, &genai.FunctionDeclaration{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  &schema,
		})
	}

	a.functions = functions
	return nil
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

// ProcessMessageStream handles a single user message and streams the agent's response with tool calls appearing immediately
func (a *Agent) ProcessMessageStream(ctx context.Context, userInput string, textCallback StreamingCallback) ([]Message, error) {
	return a.ProcessMessageStreamWithMessageCallback(ctx, userInput, textCallback, nil)
}

// ProcessMessageStreamWithMessageCallback handles streaming with both text and message callbacks
func (a *Agent) ProcessMessageStreamWithMessageCallback(ctx context.Context, userInput string, textCallback StreamingCallback, messageCallback StreamingMessageCallback) ([]Message, error) {
	messages := []Message{}
	userMessageContent := &genai.Content{
		Role: "user",
		Parts: []*genai.Part{
			{Text: userInput},
		},
	}
	a.Conversation = append(a.Conversation, userMessageContent)

	for {
		// Count tokens before inference
		tokensUsed, err := a.countTokens(ctx, a.Conversation)
		if err != nil {
			fmt.Printf("Warning: Could not count tokens: %v\n", err)
		} else {
			a.TokenUsage.InputTokens += tokensUsed
			a.TokenUsage.TotalTokens += tokensUsed
			messages = append(messages, Message{
				Type:    TokenInfo,
				Content: fmt.Sprintf("Input tokens: %d, Total: %d", tokensUsed, a.TokenUsage.TotalTokens),
			})
		}

		streamResponse := a.runInferenceStream(ctx, a.Conversation)

		var accumulatedText string
		var accumulatedParts []*genai.Part

		// Track which tool calls we've already processed to avoid duplicates
		processedToolCalls := make(map[string]bool)

		// Iterate through the streaming response
		for chunk, err := range streamResponse {
			if err != nil {
				return nil, fmt.Errorf("streaming error: %w", err)
			}

			if len(chunk.Candidates) == 0 {
				continue
			}

			candidate := chunk.Candidates[0]

			// Accumulate parts for later processing
			accumulatedParts = append(accumulatedParts, candidate.Content.Parts...)

			// Process parts in order: tool calls first, then text
			for _, part := range candidate.Content.Parts {
				// Handle tool calls immediately as they appear (before text streaming)
				if part.FunctionCall != nil {
					// Create a unique key for this tool call to avoid duplicates
					callKey := fmt.Sprintf("%s:%v", part.FunctionCall.Name, part.FunctionCall.Args)
					if !processedToolCalls[callKey] {
						processedToolCalls[callKey] = true

						// Create detailed tool call message with args
						argsJSON, _ := json.Marshal(part.FunctionCall.Args)
						toolCallInfo := fmt.Sprintf("🔧 Tool Call: %s\nArguments: %s\n", part.FunctionCall.Name, string(argsJSON))

						// Show tool call immediately (before execution)
						toolMsg := Message{Type: ToolMessage, Content: toolCallInfo + "Status: Executing...", IsError: false}
						messages = append(messages, toolMsg)
						if messageCallback != nil {
							messageCallback(toolMsg)
						}

						// Execute the tool immediately
						result, isError := a.executeTool(part.FunctionCall.Name, part.FunctionCall.Args)
						toolCallInfo += fmt.Sprintf("Result: %s", result)

						// Update the message with the result
						finalToolMsg := Message{Type: ToolMessage, Content: toolCallInfo, IsError: isError}
						messages[len(messages)-1] = finalToolMsg
						if messageCallback != nil {
							messageCallback(finalToolMsg)
						}
					}
				}

				// Then handle text content streaming
				if part.Text != "" {
					accumulatedText += part.Text

					// Add stream chunk to messages for display
					messages = append(messages, Message{
						Type:     StreamChunk,
						Content:  part.Text,
						IsStream: true,
					})

					if textCallback != nil {
						if err := textCallback(part.Text); err != nil {
							return messages, fmt.Errorf("streaming callback error: %w", err)
						}
					}
				}
			}
		}

		// Add accumulated AI response to conversation
		aiContent := &genai.Content{
			Role:  "model",
			Parts: accumulatedParts,
		}
		a.Conversation = append(a.Conversation, aiContent)

		// Check for function calls in accumulated parts
		hasToolCalls := false
		var toolResults []*genai.Part

		for _, part := range accumulatedParts {
			if part.FunctionCall != nil {
				hasToolCalls = true

				// Create detailed tool call message with args and result
				argsJSON, _ := json.Marshal(part.FunctionCall.Args)
				toolCallInfo := fmt.Sprintf("🔧 Tool Call: %s\nArguments: %s\n", part.FunctionCall.Name, string(argsJSON))

				result, isError := a.executeTool(part.FunctionCall.Name, part.FunctionCall.Args)
				toolCallInfo += fmt.Sprintf("Result: %s", result)

				messages = append(messages, Message{Type: ToolMessage, Content: toolCallInfo, IsError: isError})
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

		// Return accumulated text response
		if accumulatedText != "" {
			messages = append(messages, Message{Type: AgentMessage, Content: accumulatedText})

			// Count output tokens
			outputTokens, err := a.countTokens(ctx, []*genai.Content{aiContent})
			if err != nil {
				fmt.Printf("Warning: Could not count output tokens: %v\n", err)
			} else {
				a.TokenUsage.OutputTokens += outputTokens
				a.TokenUsage.TotalTokens += outputTokens
				messages = append(messages, Message{
					Type:    TokenInfo,
					Content: fmt.Sprintf("Output tokens: %d, Total: %d", outputTokens, a.TokenUsage.TotalTokens),
				})
			}

			return messages, nil
		}
		return messages, nil // Should not be reached
	}
}

// runInference handles the AI inference with tool support
func (a *Agent) runInference(ctx context.Context, conversation []*genai.Content) (*genai.GenerateContentResponse, error) {
	// Configure the generation with tools
	config := &genai.GenerateContentConfig{
		Tools: []*genai.Tool{
			{
				FunctionDeclarations: a.functions,
			},
		},
		MaxOutputTokens: 1024,
	}

	return a.client.Models.GenerateContent(ctx, a.Model, conversation, config)
}

// runInferenceStream handles the AI inference with streaming and tool support
func (a *Agent) runInferenceStream(ctx context.Context, conversation []*genai.Content) iter.Seq2[*genai.GenerateContentResponse, error] {
	// Configure the generation with tools
	config := &genai.GenerateContentConfig{
		Tools: []*genai.Tool{
			{
				FunctionDeclarations: a.functions,
			},
		},
		MaxOutputTokens: 1024,
	}

	return a.client.Models.GenerateContentStream(ctx, a.Model, conversation, config)
}

// countTokens counts the tokens in the given conversation
func (a *Agent) countTokens(ctx context.Context, conversation []*genai.Content) (int, error) {
	config := &genai.CountTokensConfig{}

	response, err := a.client.Models.CountTokens(ctx, a.Model, conversation, config)
	if err != nil {
		return 0, fmt.Errorf("failed to count tokens: %w", err)
	}

	return int(response.TotalTokens), nil
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

	// fmt.Printf("\u001b[92mtool\u001b[0m: %s(%s)\n", name, string(argsJSON))

	result, err := toolDef.Function(argsJSON)
	if err != nil {
		result = fmt.Sprintf("Error executing tool %s: %v", name, err)
		return result, true
	}

	// fmt.Printf("\u001b[92mresult\u001b[0m: %s\n", result)
	return result, false
}

// GetTokenUsage returns the current token usage statistics
func (a *Agent) GetTokenUsage() TokenUsage {
	return a.TokenUsage
}

// ResetTokenUsage resets the token usage counters
func (a *Agent) ResetTokenUsage() {
	a.TokenUsage = TokenUsage{}
}
