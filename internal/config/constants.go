package config

import (
	_ "embed"
)

// SystemPrompt is loaded from sys.md at compile time
//
//go:embed SYSTEM.md
var SystemPrompt string

// WelcomeMessage is the initial greeting shown to users
const WelcomeMessage = `Welcome to the CLI Code Assistant! Type your request below or press F2 to select a different model. 

System prompt loaded: %d characters`
