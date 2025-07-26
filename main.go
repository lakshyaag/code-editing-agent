package main

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"agent/internal/agent"
	"agent/internal/config"
	"agent/internal/tools"
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

	// Set up user input scanner
	scanner := bufio.NewScanner(os.Stdin)
	getUserMessage := func() (string, bool) {
		if !scanner.Scan() {
			return "", false
		}
		return scanner.Text(), true
	}

	// Get all available tools
	availableTools := tools.GetAllTools()

	// Create and run the agent
	codeAgent := agent.New(client, cfg.Model, getUserMessage, availableTools)

	err = codeAgent.Run(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}
}
