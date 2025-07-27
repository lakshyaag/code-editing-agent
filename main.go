package main

import (
	"context"
	"fmt"
	"os"

	"agent/internal/agent"
	"agent/internal/config"
	"agent/internal/tools"
	"agent/internal/tui"
)

func main() {

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
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

	// Create and run the agent in TUI mode
	tuiAgent := agent.New(client, cfg.Model, availableTools)
	tui.Start(tuiAgent)
}
