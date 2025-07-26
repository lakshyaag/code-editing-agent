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

	"github.com/invopop/jsonschema"
	"github.com/joho/godotenv"
	"google.golang.org/genai"
)

type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
	Function    func(input json.RawMessage) (string, error)
}

type Agent struct {
	client         *genai.Client
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

func NewAgent(client *genai.Client, model string, getUserMessage func() (string, bool), tools []ToolDefinition) *Agent {
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

	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		fmt.Fprintf(os.Stderr, "ERROR: GOOGLE_API_KEY environment variable is required\n")
		os.Exit(1)
	}

	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to create Gemini client: %v\n", err)
		os.Exit(1)
	}

	model := "gemini-2.0-flash"

	scanner := bufio.NewScanner(os.Stdin)

	getUserMessage := func() (string, bool) {
		if !scanner.Scan() {
			return "", false
		}
		return scanner.Text(), true
	}

	tools := []ToolDefinition{ReadFileDefinition, ListFilesDefinition, EditFileDefinition}

	agent := NewAgent(client, model, getUserMessage, tools)

	err = agent.Run(context.TODO())

	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
	}

}

func (a *Agent) Run(ctx context.Context) error {
	conversation := []*genai.Content{}

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

func GenerateSchema[T any]() map[string]interface{} {
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

	var params map[string]interface{}
	if err := json.Unmarshal(schemaBytes, &params); err != nil {
		panic(fmt.Sprintf("failed to unmarshal schema to map: %v", err))
	}
	return params
}
