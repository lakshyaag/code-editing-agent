package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"strings"
	"time"

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

// AgentConfig holds configuration for the agent
type AgentConfig struct {
	MaxOutputTokens      int32
	Temperature          float32
	TopK                 float32  // Changed from int32 to float32
	TopP                 float32
	ThinkingBudget       int32 // -1 for unlimited
	SupportedThinkingModels []string // Models that support thinking mode
}

// DefaultAgentConfig returns sensible defaults
func DefaultAgentConfig() *AgentConfig {
	return &AgentConfig{
		MaxOutputTokens: 8192, // Increased from 1024 for better responses
		Temperature:     0.7,
		TopK:            40,   // This is still valid as a float32
		TopP:            0.95,
		ThinkingBudget:  -1, // Unlimited by default
		SupportedThinkingModels: []string{
			"gemini-2.5-pro",
			"gemini-2.5-flash",
			"gemini-2.5-flash-lite",
			// Add new models here as they support thinking
		},
	}
}

// Agent represents the main AI agent that can execute tools
type Agent struct {
	client       *genai.Client
	Model        string
	tools        []ToolDefinition
	Conversation []*genai.Content
	TokenUsage   TokenUsage
	functions    []*genai.FunctionDeclaration // Pre-computed function declarations
	config       *AgentConfig
}

// ToolDefinition defines the structure for a tool that the agent can use
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
	Function    func(ctx context.Context, input json.RawMessage) (string, error)
}

// New creates a new Agent instance
func New(client *genai.Client, model string, tools []ToolDefinition) *Agent {
	return NewWithConfig(client, model, tools, DefaultAgentConfig())
}

// NewWithConfig creates a new Agent instance with custom configuration
func NewWithConfig(client *genai.Client, model string, tools []ToolDefinition, config *AgentConfig) *Agent {
	agent := &Agent{
		client: client,
		Model:  model,
		tools:  tools,
		config: config,
	}

	// Pre-compute function declarations for efficiency
	if err := agent.precomputeFunctionDeclarations(); err != nil {
		// Log error but don't fail - tools will be unavailable
		fmt.Printf("Warning: Failed to initialize function declarations: %v\n", err)
	}

	return agent
}

// precomputeFunctionDeclarations converts tool definitions to Gemini function declarations once
func (a *Agent) precomputeFunctionDeclarations() error {
	var functions []*genai.FunctionDeclaration
	for _, tool := range a.tools {
		// Convert map[string]interface{} to genai.Schema
		schemaBytes, err := json.Marshal(tool.InputSchema)
		if err != nil {
			return fmt.Errorf("failed to marshal schema for tool %s: %w", tool.Name, err)
		}

		var schema genai.Schema
		if err := json.Unmarshal(schemaBytes, &schema); err != nil {
			return fmt.Errorf("failed to unmarshal schema for tool %s: %w", tool.Name, err)
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

// Helper function to create pointers
func ptr[T any](v T) *T {
	return &v
}

// isThinkingSupported checks if the current model supports thinking mode
func (a *Agent) isThinkingSupported() bool {
	if a.Model == "" {
		return false
	}
	
	for _, model := range a.config.SupportedThinkingModels {
		if strings.Contains(a.Model, model) {
			return true
		}
	}
	return false
}

// runInferenceStream runs the model inference and handles streaming
func (a *Agent) runInferenceStream(ctx context.Context, conversation []*genai.Content, enableThinking bool) iter.Seq2[*genai.GenerateContentResponse, error] {
	// Determine thinking config if applicable
	var thinkingConfig *genai.ThinkingConfig
	if enableThinking && a.isThinkingSupported() {
		thinkingConfig = &genai.ThinkingConfig{
			IncludeThoughts: true,  // Use direct bool value
			ThinkingBudget:  ptr(a.config.ThinkingBudget),
		}
	}

	config := &genai.GenerateContentConfig{
		Tools: []*genai.Tool{
			{
				FunctionDeclarations: a.functions,
			},
		},
		MaxOutputTokens:   a.config.MaxOutputTokens,
		Temperature:       ptr(a.config.Temperature),
		TopK:              ptr(a.config.TopK),
		TopP:              ptr(a.config.TopP),
		SystemInstruction: &genai.Content{
			Role: "user",
			Parts: []*genai.Part{
				{Text: config.SystemPrompt},
			},
		},
		ThinkingConfig:    thinkingConfig,
	}

	return a.client.Models.GenerateContentStream(ctx, a.Model, conversation, config)
}

// ProcessMessage handles a single user message and streams the agent's response
func (a *Agent) ProcessMessage(ctx context.Context, userInput string, textCallback StreamingCallback, toolCallback ToolMessageCallback, thoughtCallback ThoughtMessageCallback, confirmationCallback ToolConfirmationCallback, enableThinking bool) ([]Message, error) {
	// Ensure we have a deadline on the context
	if _, ok := ctx.Deadline(); !ok {
		// Set a reasonable timeout if none exists
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
	}

	messages := []Message{}
	userMessageContent := &genai.Content{
		Role: "user",
		Parts: []*genai.Part{
			{Text: userInput},
		},
	}
	a.Conversation = append(a.Conversation, userMessageContent)

	for {
		// Check context before proceeding
		if err := ctx.Err(); err != nil {
			return messages, fmt.Errorf("context cancelled: %w", err)
		}

		// Count input tokens and update internal tracking
		if inputTokens, err := a.countTokens(ctx, a.Conversation); err == nil {
			a.TokenUsage.InputTokens += inputTokens
			a.TokenUsage.TotalTokens += inputTokens
		}

		streamResponse := a.runInferenceStream(ctx, a.Conversation, enableThinking)

		var accumulatedText string
		var accumulatedParts []*genai.Part
		var toolResults []*genai.Part
		processedToolCalls := make(map[string]bool)

		// Process streaming response
		for chunk, err := range streamResponse {
			if err != nil {
				return messages, fmt.Errorf("streaming error: %w", err)
			}

			if len(chunk.Candidates) == 0 {
				continue
			}

			candidate := chunk.Candidates[0]
			
			// Check for finish reason
			if candidate.FinishReason != "" && candidate.FinishReason != "STOP" {
				// Handle specific finish reasons
				switch candidate.FinishReason {
				case "MAX_TOKENS":
					messages = append(messages, Message{
						Type:    AgentMessage,
						Content: "\n\n[Response truncated due to length limit]",
						IsError: true,
					})
				case "SAFETY":
					messages = append(messages, Message{
						Type:    AgentMessage,
						Content: "\n\n[Response blocked by safety filters]",
						IsError: true,
					})
				}
			}

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
						if err := thoughtCallback(thoughtMsg); err != nil {
							// Log but don't fail on callback errors
							fmt.Printf("Warning: thought callback error: %v\n", err)
						}
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
						result, err := a.executeTool(ctx, part.FunctionCall.Name, part.FunctionCall.Args)
						
						argsJSON, _ := json.Marshal(part.FunctionCall.Args)
						var toolCallInfo string
						var isError bool
						
						if err != nil {
							toolCallInfo = fmt.Sprintf("🔧 Tool Call: %s\nArguments: %s\nError: %v",
								part.FunctionCall.Name, string(argsJSON), err)
							isError = true
							result = fmt.Sprintf("Error: %v", err)
						} else {
							toolCallInfo = fmt.Sprintf("🔧 Tool Call: %s\nArguments: %s\nResult: %s",
								part.FunctionCall.Name, string(argsJSON), result)
							isError = false
						}

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
							// Log but don't fail on callback errors
							fmt.Printf("Warning: text callback error: %v\n", err)
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
func (a *Agent) executeTool(ctx context.Context, name string, args map[string]interface{}) (string, error) {
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
		return "", fmt.Errorf("tool %s not found", name)
	}

	// Convert args to JSON
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return "", fmt.Errorf("failed to marshal arguments: %w", err)
	}

	// Execute with context
	result, err := toolDef.Function(ctx, argsJSON)
	if err != nil {
		return "", fmt.Errorf("tool execution failed: %w", err)
	}

	return result, nil
}

// GetTokenUsage returns the current token usage statistics
func (a *Agent) GetTokenUsage() TokenUsage {
	return a.TokenUsage
}

// ResetTokenUsage resets the token usage counters
func (a *Agent) ResetTokenUsage() {
	a.TokenUsage = TokenUsage{}
}

// ClearConversation clears the conversation history
func (a *Agent) ClearConversation() {
	a.Conversation = nil
	a.ResetTokenUsage()
}

// GetConfig returns the agent configuration
func (a *Agent) GetConfig() *AgentConfig {
	return a.config
}

// UpdateConfig updates the agent configuration
func (a *Agent) UpdateConfig(config *AgentConfig) {
	a.config = config
}
