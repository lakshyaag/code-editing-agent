package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

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

var ListFilesDefinition = ToolDefinition{
	Name:        "list_files",
	Description: "List files and directories in a given relative directory path. Use this to see the contents of a directory.",
	InputSchema: ListFilesInputSchema,
	Function:    ListFiles,
}

type ListFilesInput struct {
	Path string `json:"path,omitempty" jsonschema_description:"Optional relative path to list files from. Defaults to current directory if not provided."`
}

var ListFilesInputSchema = GenerateSchema[ListFilesInput]()

func ListFiles(input json.RawMessage) (string, error) {
	listFilesInput := ListFilesInput{}

	err := json.Unmarshal(input, &listFilesInput)
	if err != nil {
		panic(err)
	}

	dir := "."
	if listFilesInput.Path != "" {
		dir = listFilesInput.Path
	}

	type FileNode struct {
		Name     string     `json:"name"`
		IsDir    bool       `json:"is_dir"`
		Children []FileNode `json:"children,omitempty"`
	}

	var buildTree func(string, int) ([]FileNode, error)
	buildTree = func(currentPath string, depth int) ([]FileNode, error) {
		if depth > 3 {
			return nil, nil
		}
		entries, err := os.ReadDir(currentPath)
		if err != nil {
			return nil, err
		}
		var nodes []FileNode
		for _, entry := range entries {
			name := entry.Name()
			if strings.HasPrefix(name, ".") {
				continue // skip hidden files/dirs
			}
			fullPath := filepath.Join(currentPath, name)
			node := FileNode{
				Name:  name,
				IsDir: entry.IsDir(),
			}
			if entry.IsDir() && depth < 3 {
				children, err := buildTree(fullPath, depth+1)
				if err != nil {
					return nil, err
				}
				node.Children = children
			}
			nodes = append(nodes, node)
		}
		return nodes, nil
	}

	tree, err := buildTree(dir, 1)
	if err != nil {
		return "", err
	}
	result, err := json.Marshal(tree)
	if err != nil {
		return "", fmt.Errorf("failed to marshal file tree: %w", err)
	}
	return string(result), nil
}

var EditFileDefinition = ToolDefinition{
	Name: "edit_file",
	Description: `Make edits to a text file.

Replaces 'old_str' with 'new_str' in the given file. 'old_str' and 'new_str' MUST be different from each other.

If the file specified with path doesn't exist, it will be created.
`,
	InputSchema: EditFileInputSchema,
	Function:    EditFile,
}

type EditFileInput struct {
	Path   string `json:"path" jsonschema_description:"The path to the file"`
	OldStr string `json:"old_str" jsonschema_description:"Text to search for - must match exactly and must only have one match exactly"`
	NewStr string `json:"new_str" jsonschema_description:"Text to replace old_str with"`
}

var EditFileInputSchema = GenerateSchema[EditFileInput]()

func EditFile(input json.RawMessage) (string, error) {
	editFileInput := EditFileInput{}
	err := json.Unmarshal(input, &editFileInput)
	if err != nil {
		return "", err
	}

	if editFileInput.Path == "" || editFileInput.OldStr == editFileInput.NewStr {
		return "", fmt.Errorf("invalid input parameters")
	}

	content, err := os.ReadFile(editFileInput.Path)
	if err != nil {
		if os.IsNotExist(err) && editFileInput.OldStr == "" {
			return createNewFile(editFileInput.Path, editFileInput.NewStr)
		}
		return "", err
	}

	oldContent := string(content)
	newContent := strings.Replace(oldContent, editFileInput.OldStr, editFileInput.NewStr, -1)

	if oldContent == newContent && editFileInput.OldStr != "" {
		return "", fmt.Errorf("`old_str` not found. No changes made to the file")
	}

	err = os.WriteFile(editFileInput.Path, []byte(newContent), 0644)
	if err != nil {
		return "", err
	}

	return "OK. Edited file successfully", nil
}

func createNewFile(filePath, content string) (string, error) {
	dir := path.Dir(filePath)
	if dir != "." {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return "", fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		return "", fmt.Errorf("failed to create file %s: %w", filePath, err)
	}

	return fmt.Sprintf("File %s created successfully", filePath), nil
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
	// Try to load .env, but do not fail if missing
	_ = godotenv.Load()

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

	tools := []ToolDefinition{ReadFileDefinition, ListFilesDefinition, EditFileDefinition}

	agent := NewAgent(&client, model, getUserMessage, tools)

	err = agent.Run(context.TODO())

	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
	}

}

func (a *Agent) Run(ctx context.Context) error {
	conversation := []openai.ChatCompletionMessageParamUnion{}

	fmt.Printf("Chat with CLI\n")
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get current working directory: %v\n", err)
	} else {
		fmt.Printf("Current working directory: %s\n", cwd)
	}

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
