package models

import "fmt"

// Model represents a Gemini AI model configuration
type Model struct {
	ID          string
	Name        string
	Description string
	MaxTokens   int
	IsDefault   bool
}

// Available Gemini models
var AvailableModels = []Model{
	{
		ID:          "gemini-2.5-flash",
		Name:        "Gemini 2.5 Flash",
		Description: "Latest fast model with improved reasoning and multimodal capabilities",
		MaxTokens:   8192,
		IsDefault:   true,
	},
	{
		ID:          "gemini-2.5-flash-exp",
		Name:        "Gemini 2.5 Flash Experimental",
		Description: "Experimental version with cutting-edge features",
		MaxTokens:   8192,
		IsDefault:   false,
	},
	{
		ID:          "gemini-2.0-flash-exp",
		Name:        "Gemini 2.0 Flash Experimental",
		Description: "Experimental Gemini 2.0 model with enhanced performance",
		MaxTokens:   8192,
		IsDefault:   false,
	},
	{
		ID:          "gemini-2.0-flash-thinking-exp",
		Name:        "Gemini 2.0 Flash Thinking",
		Description: "Experimental model with enhanced reasoning capabilities",
		MaxTokens:   8192,
		IsDefault:   false,
	},
	{
		ID:          "gemini-1.5-pro",
		Name:        "Gemini 1.5 Pro",
		Description: "High-performance model for complex reasoning tasks",
		MaxTokens:   2097152,
		IsDefault:   false,
	},
	{
		ID:          "gemini-1.5-flash",
		Name:        "Gemini 1.5 Flash",
		Description: "Fast and efficient model for quick responses",
		MaxTokens:   1048576,
		IsDefault:   false,
	},
}

// GetModelByID returns a model by its ID
func GetModelByID(id string) (*Model, error) {
	for _, model := range AvailableModels {
		if model.ID == id {
			return &model, nil
		}
	}
	return nil, fmt.Errorf("model with ID '%s' not found", id)
}

// GetDefaultModel returns the default model
func GetDefaultModel() *Model {
	for _, model := range AvailableModels {
		if model.IsDefault {
			return &model
		}
	}
	// Fallback to first model if no default is set
	if len(AvailableModels) > 0 {
		return &AvailableModels[0]
	}
	return nil
}

// GetModelNames returns a slice of model names for UI display
func GetModelNames() []string {
	names := make([]string, len(AvailableModels))
	for i, model := range AvailableModels {
		names[i] = fmt.Sprintf("%s - %s", model.Name, model.Description)
	}
	return names
}

// GetModelIDs returns a slice of model IDs
func GetModelIDs() []string {
	ids := make([]string, len(AvailableModels))
	for i, model := range AvailableModels {
		ids[i] = model.ID
	}
	return ids
}