package config

import (
	"context"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"google.golang.org/genai"
)

// Config holds the application configuration
type Config struct {
	APIKey string
	Model  string
}

const (
	defaultModel = "gemini-2.5-flash"
)

// Load loads configuration from environment variables
func Load() (*Config, error) {
	// Try to load .env, but don't fail if it's missing
	_ = godotenv.Load()

	// Required: API Key
	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GOOGLE_API_KEY environment variable is required")
	}

	// Optional: Model Name (with default)
	model := os.Getenv("GOOGLE_MODEL")
	if model == "" {
		model = defaultModel
	}

	return &Config{
		APIKey: apiKey,
		Model:  model,
	}, nil
}

// CreateClient creates a new Gemini client using the configuration
func (c *Config) CreateClient(ctx context.Context) (*genai.Client, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  c.APIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	return client, nil
}
