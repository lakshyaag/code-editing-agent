package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"strings"

	"agent/internal/config"

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

	// ToolMessageCallback is called when a tool message is ready to display
	ToolMessageCallback func(msg Message) error

	// ThoughtMessageCallback is called when a thought message is ready to display
	ThoughtMessageCallback func(msg Message) error

	// ToolConfirmationCallback is called to get user confirmation before executing a tool
	// Returns true if the tool should be executed, false if it should be skipped
	ToolConfirmationCallback func(toolName string, args map[string]interface{}) (bool, error)
)

const (
	UserMessage MessageType = iota
	AgentMessage
	ToolMessage
	StreamChunk
	ThoughtMessage
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

// ProcessMessage handles a single user message and streams the agent's response
func (a *Agent) ProcessMessage(ctx context.Context, userInput string, textCallback StreamingCallback, toolCallback ToolMessageCallback, thoughtCallback ThoughtMessageCallback, confirmationCallback ToolConfirmationCallback) ([]Message, error) {
	messages := []Message{}
	userMessageContent := &genai.Content{
		Role: "user",
		Parts: []*genai.Part{
			{Text: userInput},
		},
	}
	a.Conversation = append(a.Conversation, userMessageContent)

	for {
		// Count input tokens and update internal tracking
		if inputTokens, err := a.countTokens(ctx, a.Conversation); err == nil {
			a.TokenUsage.InputTokens += inputTokens
			a.TokenUsage.TotalTokens += inputTokens
		}

		streamResponse := a.runInferenceStream(ctx, a.Conversation)

		var accumulatedText string
		var accumulatedParts []*genai.Part
		var toolResults []*genai.Part
		processedToolCalls := make(map[string]bool)

		// Process streaming response
		for chunk, err := range streamResponse {
			if err != nil {
				return nil, fmt.Errorf("streaming error: %w", err)
			}

			if len(chunk.Candidates) == 0 {
				continue
			}

			candidate := chunk.Candidates[0]
			accumulatedParts = append(accumulatedParts, candidate.Content.Parts...)

			// Process each part in the chunk
			for _, part := range candidate.Content.Parts {
				// Handle thought messages immediately
				if part.Thought && part.Text != "" {
					thoughtMsg := Message{
						Type:    ThoughtMessage,
						Content: fmt.Sprintf("💭 Thinking: %s", part.Text),
					}

					messages = append(messages, thoughtMsg)

					// Send thought message immediately via callback
					if thoughtCallback != nil {
						thoughtCallback(thoughtMsg)
					}
					continue // Don't process this as regular text
				}

				// Handle tool calls immediately
				if part.FunctionCall != nil {
					callKey := fmt.Sprintf("%s:%v", part.FunctionCall.Name, part.FunctionCall.Args)
					if !processedToolCalls[callKey] {
						processedToolCalls[callKey] = true

						// Get user confirmation if callback is provided
						if confirmationCallback != nil {
							confirmed, err := confirmationCallback(part.FunctionCall.Name, part.FunctionCall.Args)
							if err != nil {
								return messages, fmt.Errorf("confirmation error: %w", err)
							}
							if !confirmed {
								// User rejected the tool call
								argsJSON, _ := json.Marshal(part.FunctionCall.Args)
								toolCallInfo := fmt.Sprintf("🚫 Tool Call Rejected: %s\nArguments: %s\nReason: User denied execution",
									part.FunctionCall.Name, string(argsJSON))

								toolMsg := Message{
									Type:    ToolMessage,
									Content: toolCallInfo,
									IsError: true,
								}

								messages = append(messages, toolMsg)

								// Send tool message immediately via callback
								if toolCallback != nil {
									toolCallback(toolMsg)
								}

								// Prepare rejection response for conversation
								toolResults = append(toolResults, &genai.Part{
									FunctionResponse: &genai.FunctionResponse{
										Name:     part.FunctionCall.Name,
										Response: map[string]interface{}{"error": "User denied tool execution"},
									},
								})
								continue
							}
						}

						// Execute tool and create message
						result, isError := a.executeTool(part.FunctionCall.Name, part.FunctionCall.Args)

						argsJSON, _ := json.Marshal(part.FunctionCall.Args)
						toolCallInfo := fmt.Sprintf("🔧 Tool Call: %s\nArguments: %s\nResult: %s",
							part.FunctionCall.Name, string(argsJSON), result)

						toolMsg := Message{
							Type:    ToolMessage,
							Content: toolCallInfo,
							IsError: isError,
						}

						messages = append(messages, toolMsg)

						// Send tool message immediately via callback
						if toolCallback != nil {
							toolCallback(toolMsg)
						}

						// Prepare tool result for conversation
						toolResults = append(toolResults, &genai.Part{
							FunctionResponse: &genai.FunctionResponse{
								Name:     part.FunctionCall.Name,
								Response: map[string]interface{}{"result": result},
							},
						})
					}
				}

				// Handle text streaming
				if part.Text != "" {
					accumulatedText += part.Text

					// Stream the text chunk
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

		// Add AI response to conversation
		aiContent := &genai.Content{
			Role:  "model",
			Parts: accumulatedParts,
		}
		a.Conversation = append(a.Conversation, aiContent)

		// Count output tokens and update internal tracking
		if outputTokens, err := a.countTokens(ctx, []*genai.Content{aiContent}); err == nil {
			a.TokenUsage.OutputTokens += outputTokens
			a.TokenUsage.TotalTokens += outputTokens
		}

		// If we have tool calls, add results to conversation and continue
		if len(toolResults) > 0 {
			toolContent := &genai.Content{
				Role:  "user",
				Parts: toolResults,
			}
			a.Conversation = append(a.Conversation, toolContent)
			continue
		}

		// Return final agent message
		if accumulatedText != "" {
			messages = append(messages, Message{Type: AgentMessage, Content: accumulatedText})
		}

		return messages, nil
	}
}

// runInferenceStream handles the AI inference with streaming and tool support
func (a *Agent) runInferenceStream(ctx context.Context, conversation []*genai.Content) iter.Seq2[*genai.GenerateContentResponse, error] {
	thinkingConfig := &genai.ThinkingConfig{}

	if strings.Contains(a.Model, "gemini-2.5") {
		thinkingBudgetVal := int32(-1)
		thinkingConfig = &genai.ThinkingConfig{
			ThinkingBudget:  &thinkingBudgetVal,
			IncludeThoughts: true,
		}
	} else {
		thinkingConfig = nil
	}

	// if model is gemini-2.5 family, then enable thinking, otherwise disable it
	config := &genai.GenerateContentConfig{
		Tools: []*genai.Tool{
			{
				FunctionDeclarations: a.functions,
			},
		},
		MaxOutputTokens:   1024,
		SystemInstruction: genai.NewContentFromText(config.SystemPrompt, genai.RoleUser),
		ThinkingConfig:    thinkingConfig,
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

	result, err := toolDef.Function(argsJSON)
	if err != nil {
		result = fmt.Sprintf("Error executing tool %s: %v", name, err)
		return result, true
	}

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
