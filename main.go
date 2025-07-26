package main

import (
	"context"
	"fmt"
	"os"

	"agent/internal/agent"
	"agent/internal/config"
	"agent/internal/tools"
	"agent/internal/ui"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}

	// Load user preferences
	prefs, err := config.LoadPreferences()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not load preferences: %s\n", err)
		prefs = config.DefaultPreferences()
	}

	// Use the model from preferences if available, otherwise fall back to config
	currentModel := prefs.LastSelectedModel
	if currentModel == "" {
		currentModel = cfg.Model
	}

	// Create Gemini client
	ctx := context.Background()
	client, err := cfg.CreateClient(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}

	// Get all available tools
	availableTools := tools.GetAllTools()

	// Create the TUI application
	app := ui.NewApp(cfg, prefs)

	// Create the TUI-aware agent
	tuiAgent := agent.NewTUIAgent(client, currentModel, availableTools, app, ctx)

	// Set up quit handler to save preferences
	app.SetOnQuit(func() {
		// Save current preferences
		if err := prefs.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not save preferences: %s\n", err)
		}
	})

	// Set up model change handler to update agent
	app.SetOnModelChanged(func(newModel string) {
		tuiAgent.UpdateModel(newModel)
	})

	// Start the agent (initializes welcome message)
	tuiAgent.Start()

	// Run the TUI application
	err = app.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}
}
