package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Preferences holds user preferences that persist across sessions
type Preferences struct {
	LastSelectedModel string `json:"last_selected_model"`
	Theme            string `json:"theme,omitempty"`
	ShowTimestamps   bool   `json:"show_timestamps"`
	AutoSave         bool   `json:"auto_save"`
}

// DefaultPreferences returns the default preferences
func DefaultPreferences() *Preferences {
	return &Preferences{
		LastSelectedModel: "gemini-2.5-flash",
		Theme:            "default",
		ShowTimestamps:   true,
		AutoSave:         true,
	}
}

// LoadPreferences loads user preferences from the config file
func LoadPreferences() (*Preferences, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return DefaultPreferences(), nil
	}

	configFile := filepath.Join(configDir, "preferences.json")
	
	// If file doesn't exist, return defaults
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return DefaultPreferences(), nil
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		return DefaultPreferences(), nil
	}

	var prefs Preferences
	if err := json.Unmarshal(data, &prefs); err != nil {
		return DefaultPreferences(), nil
	}

	return &prefs, nil
}

// SavePreferences saves user preferences to the config file
func (p *Preferences) Save() error {
	configDir, err := getConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configFile := filepath.Join(configDir, "preferences.json")
	
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal preferences: %w", err)
	}

	if err := os.WriteFile(configFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write preferences file: %w", err)
	}

	return nil
}

// getConfigDir returns the application config directory
func getConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	
	return filepath.Join(homeDir, ".config", "cli-agent"), nil
}

// UpdateSelectedModel updates the last selected model and saves preferences
func (p *Preferences) UpdateSelectedModel(modelID string) error {
	p.LastSelectedModel = modelID
	return p.Save()
}