package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/joho/godotenv"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/azure"
)

type Agent struct {
	client         *openai.Client
	model          string
	getUserMessage func() (string, bool)
}

func NewAgent(client *openai.Client, model string, getUserMessage func() (string, bool)) *Agent {
	return &Agent{
		client:         client,
		model:          model,
		getUserMessage: getUserMessage,
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

	agent := NewAgent(&client, model, getUserMessage)

	err = agent.Run(context.TODO())

	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
	}

}

func (a *Agent) Run(ctx context.Context) error {
	conversation := []openai.ChatCompletionMessageParamUnion{}

	fmt.Printf("Chat with CLI\n")

	for {
		fmt.Print("\u001b[94mYou\u001b[0m: ")
		userInput, ok := a.getUserMessage()

		if !ok {
			break
		}

		userMessage := openai.UserMessage(userInput)
		conversation = append(conversation, userMessage)

		aiMessage, err := a.runInference(ctx, conversation)

		if err != nil {
			return err
		}

		conversation = append(conversation, aiMessage.Choices[0].Message.ToParam())

		fmt.Printf("\u001b[94mCodeAgent\u001b[0m: %s\n", aiMessage.Choices[0].Message.Content)

	}
	return nil
}

func (a *Agent) runInference(ctx context.Context, conversation []openai.ChatCompletionMessageParamUnion) (*openai.ChatCompletion, error) {
	message, err := a.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages:  conversation,
		Model:     openai.ChatModel(a.model),
		MaxTokens: openai.Int(1024),
	})

	return message, err
}
