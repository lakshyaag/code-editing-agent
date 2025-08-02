package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// UserPreferences stores user-specific settings
type UserPreferences struct {
	SelectedModel           string `json:"selected_model,omitempty"`
	RequireToolConfirmation bool   `json:"require_tool_confirmation"`
	EnableThinkingMode      bool   `json:"enable_thinking_mode"`
}

// GetPreferencesPath returns the path to the preferences file
func GetPreferencesPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".code-agent")
	return filepath.Join(configDir, "config.json"), nil
}

// LoadPreferences loads user preferences from disk
func LoadPreferences() (*UserPreferences, error) {
	prefsPath, err := GetPreferencesPath()
	if err != nil {
		return nil, err
	}

	// If file doesn't exist, return default preferences
	if _, err := os.Stat(prefsPath); os.IsNotExist(err) {
		return &UserPreferences{
			RequireToolConfirmation: true,  // Default to true for safety
			EnableThinkingMode:      false, // Default to false
		}, nil
	}

	data, err := os.ReadFile(prefsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read preferences: %w", err)
	}

	var prefs UserPreferences
	if err := json.Unmarshal(data, &prefs); err != nil {
		return nil, fmt.Errorf("failed to parse preferences: %w", err)
	}

	// Set default values for fields that weren't in the config
	if prefs.RequireToolConfirmation == false && prefs.SelectedModel == "" && !prefs.EnableThinkingMode {
		// If the config file exists but doesn't have this field, default to true
		prefs.RequireToolConfirmation = true
	}

	return &prefs, nil
}

// SavePreferences saves user preferences to disk
func SavePreferences(prefs *UserPreferences) error {
	prefsPath, err := GetPreferencesPath()
	if err != nil {
		return err
	}

	// Create config directory if it doesn't exist
	configDir := filepath.Dir(prefsPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal preferences with indentation for readability
	data, err := json.MarshalIndent(prefs, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal preferences: %w", err)
	}

	// Write to file
	if err := os.WriteFile(prefsPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write preferences: %w", err)
	}

	return nil
}
