package config

import (
	_ "embed"
)

// SystemPrompt is loaded from sys.md at compile time
//
//go:embed SYSTEM.md
var SystemPrompt string

// WelcomeMessage is the initial greeting shown to users
const WelcomeMessage = `Type your request below or use:
• F2: Select model  • F3: Toggle tool confirm  • F4: Toggle thinking mode

System prompt loaded (%d chars)`
