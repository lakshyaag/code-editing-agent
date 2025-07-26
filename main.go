package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/invopop/jsonschema"
	"github.com/joho/godotenv"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/azure"
)

type ToolDefinition struct {
	Name        string                    `json:"name"`
	Description string                    `json:"description"`
	InputSchema openai.FunctionParameters `json:"input_schema"`
	Function    func(input json.RawMessage) (string, error)
}

type Agent struct {
	client         *openai.Client
	model          string
	getUserMessage func() (string, bool)
	tools          []ToolDefinition
}

var ReadFileDefinition = ToolDefinition{
	Name:        "read_file",
	Description: "Read the contents of a given relative file path. Use this when you want to see what's inside a file. Do not use this with directory names.",
	InputSchema: ReadFileInputSchema,
	Function:    ReadFile,
}

type ReadFileInput struct {
	Path string `json:"path" jsonschema_description:"The relative path of a file in the working directory."`
}

var ReadFileInputSchema = GenerateSchema[ReadFileInput]()

func ReadFile(input json.RawMessage) (string, error) {
	readFileInput := ReadFileInput{}
	err := json.Unmarshal(input, &readFileInput)
	if err != nil {
		panic(err)
	}

	content, err := os.ReadFile(readFileInput.Path)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", readFileInput.Path, err)
	}

	return string(content), nil
}

func NewAgent(client *openai.Client, model string, getUserMessage func() (string, bool), tools []ToolDefinition) *Agent {
	return &Agent{
		client:         client,
		model:          model,
		getUserMessage: getUserMessage,
		tools:          tools,
	}
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	endpoint := os.Getenv("AOAI_ENDPOINT")
	api_version := os.Getenv("AOAI_API_VERSION")

	model := os.Getenv("AOAI_MODEL")
	tenantID := os.Getenv("AZURE_TENANT_ID")

	credential, err := azidentity.NewDefaultAzureCredential(&azidentity.DefaultAzureCredentialOptions{
		TenantID: tenantID,
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		return
	}

	client := openai.NewClient(
		azure.WithEndpoint(endpoint, api_version),
		azure.WithTokenCredential(credential),
	)

	scanner := bufio.NewScanner(os.Stdin)

	getUserMessage := func() (string, bool) {
		if !scanner.Scan() {
			return "", false
		}
		return scanner.Text(), true
	}

	tools := []ToolDefinition{ReadFileDefinition}

	agent := NewAgent(&client, model, getUserMessage, tools)

	err = agent.Run(context.TODO())

	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
	}

}

func (a *Agent) Run(ctx context.Context) error {
	conversation := []openai.ChatCompletionMessageParamUnion{}

	fmt.Printf("Chat with CLI\n")

	readUserInput := true
	for {
		if readUserInput {
			fmt.Print("\u001b[94mYou\u001b[0m: ")
			userInput, ok := a.getUserMessage()

			if !ok {
				break
			}

			userMessage := openai.UserMessage(userInput)
			conversation = append(conversation, userMessage)
		}

		message, err := a.runInference(ctx, conversation)

		if err != nil {
			return err
		}

		aiMessage := message.Choices[0].Message

		conversation = append(conversation, aiMessage.ToParam())

		toolResults := []openai.ChatCompletionMessageParamUnion{}
		if len(aiMessage.ToolCalls) > 0 {
			for _, toolCall := range aiMessage.ToolCalls {
				result := a.executeTool(toolCall.ID, toolCall.Function.Name, json.RawMessage([]byte(toolCall.Function.Arguments)))

				toolResults = append(toolResults, result)
			}
		} else if len(aiMessage.ToolCalls) == 0 {
			fmt.Printf("\u001b[94mCodeAgent\u001b[0m: %s\n", aiMessage.Content)
			readUserInput = true
			continue
		}

		readUserInput = false
		conversation = append(conversation, toolResults...)
	}
	return nil
}

func (a *Agent) runInference(ctx context.Context, conversation []openai.ChatCompletionMessageParamUnion) (*openai.ChatCompletion, error) {

	openaiTools := []openai.ChatCompletionToolParam{}
	for _, tool := range a.tools {
		openaiTools = append(openaiTools, openai.ChatCompletionToolParam{
			Function: openai.FunctionDefinitionParam{
				Name:        tool.Name,
				Description: openai.String(tool.Description),
				Parameters:  tool.InputSchema,
			},
		})
	}

	message, err := a.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages:  conversation,
		Model:     openai.ChatModel(a.model),
		MaxTokens: openai.Int(1024),
		Tools:     openaiTools,
	})

	return message, err
}

func (a *Agent) executeTool(id, name string, input json.RawMessage) openai.ChatCompletionMessageParamUnion {
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
		return openai.ToolMessage(fmt.Sprintf("Tool %s not found", name), id)
	}

	fmt.Printf("\u001b[92mtool\u001b[0m: %s(%s)\n", name, input)

	result, err := toolDef.Function(input)
	if err != nil {
		return openai.ToolMessage(fmt.Sprintf("Error executing tool %s: %v", name, err), id)
	}

	fmt.Printf("\u001b[92mresult\u001b[0m: %s\n", result)
	return openai.ToolMessage(result, id)
}

func GenerateSchema[T any]() openai.FunctionParameters {
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties: true,
		DoNotReference:            true,
	}
	var v T

	schema := reflector.Reflect(v)

	// Marshal the schema to JSON
	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal schema: %v", err))
	}

	var params openai.FunctionParameters
	if err := json.Unmarshal(schemaBytes, &params); err != nil {
		panic(fmt.Sprintf("failed to unmarshal schema to FunctionParameters: %v", err))
	}
	return params
}
