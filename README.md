# CLI Agent with Rich TUI

A rich terminal-based chat interface for interacting with Google's Gemini AI models, built with Go and the tview library.

## Features

✨ **Rich Terminal UI**: Beautiful, interactive terminal interface with multiple panels
🤖 **Multiple Gemini Models**: Support for various Gemini models with easy switching
💾 **Persistent Preferences**: Saves your preferred model selection across sessions
🛠️ **Built-in Tools**: File operations, code editing, and more
⌨️ **Keyboard Shortcuts**: Efficient navigation and controls

## Installation

1. Clone the repository
2. Install dependencies:
   ```bash
   go mod tidy
   ```
3. Build the application:
   ```bash
   go build -o cli-agent
   ```

## Setup

1. Get your Google AI API key from https://ai.google.dev/
2. Set the environment variable:
   ```bash
   export GOOGLE_API_KEY="your-api-key-here"
   ```

## Usage

```bash
./cli-agent
```

### Keyboard Shortcuts

- **Ctrl+Enter**: Send message
- **Esc**: Show options menu / Open model selector
- **Tab**: Navigate between UI elements
- **Ctrl+C**: Quit application

### Model Selection

- Press **Esc** to open the model selector
- Use arrow keys to navigate available models
- View detailed information about each model
- Select your preferred model for the session

## Available Models

- **Gemini 2.5 Flash**: Latest fast model with improved reasoning
- **Gemini 2.5 Flash Experimental**: Cutting-edge experimental features
- **Gemini 2.0 Flash Experimental**: Newest experimental version
- **Gemini 1.5 Flash**: Proven reliable performance
- **Gemini 1.5 Pro**: Best for complex reasoning tasks

## Project Structure

```
code-editing-agent/
├── main.go                 # Main entry point
├── internal/
│   ├── agent/             # AI agent logic
│   │   ├── agent.go       # Core agent implementation
│   │   └── tui_agent.go   # TUI-aware agent
│   ├── config/            # Configuration management
│   │   ├── config.go      # App configuration
│   │   └── preferences.go # User preferences persistence
│   ├── models/            # Model definitions
│   │   └── models.go      # Available Gemini models
│   ├── tools/             # Built-in tools
│   │   ├── file_editor.go # File editing capabilities
│   │   ├── file_lister.go # Directory listing
│   │   ├── file_reader.go # File reading
│   │   └── tools.go       # Tool registry
│   └── ui/                # User interface components
│       ├── app.go         # Main application UI
│       ├── chat.go        # Chat interface
│       ├── input.go       # Message input field
│       └── model_selector.go # Model selection dialog
└── go.mod                 # Go module definition
```

## Configuration

The application stores user preferences in `~/.config/agent/preferences.json`:

```json
{
  "last_selected_model": "gemini-2.5-flash",
  "theme": "default",
  "show_timestamps": true,
  "auto_save": true
}
```

## Development

### Building
```bash
go build -o cli-agent
```

### Running Tests
```bash
go test ./...
```

### Adding New Models

Edit `internal/models/models.go` to add new model configurations:

```go
{
    ID:          "new-model-id",
    Name:        "Display Name",
    Description: "Model description",
    MaxTokens:   8192,
    IsDefault:   false,
}
```

## Dependencies

- [rivo/tview](https://github.com/rivo/tview): Rich terminal UI components
- [gdamore/tcell](https://github.com/gdamore/tcell): Terminal handling
- [google.golang.org/genai](https://pkg.go.dev/google.golang.org/genai): Google AI Go SDK

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Test thoroughly
5. Submit a pull request

## License

This project is open source. Please check the LICENSE file for details.