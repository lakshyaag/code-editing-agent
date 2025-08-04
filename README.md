# Code Editing Agent

An interactive terminal-based AI code editing agent powered by Google's Gemini API. This tool provides an intelligent coding assistant with a modern terminal user interface.

## Quick Start

1. **Install dependencies**
   ```bash
   go mod tidy
   ```

2. **Set up your API key**
   ```bash
   export GOOGLE_API_KEY=your_api_key_here
   ```

3. **Build and run**
   ```bash
   go build -o agent
   ./agent
   ```

## Prerequisites

- **Go**: Version 1.23.0 or higher
- **Google API Key**: Get yours from [Google AI Studio](https://makersuite.google.com/app/apikey)

## Installation & Setup

### 1. Clone and Install Dependencies

```bash
git clone <repository-url>
cd code-editing-agent
go mod download
```

### 2. Configure API Key

Choose one of the following methods:

**Environment Variable** (recommended):
```bash
export GOOGLE_API_KEY=your_api_key_here
```

**Environment File**:
Create a `.env` file in the project root:
```env
GOOGLE_API_KEY=your_api_key_here
```

### 3. Build

**Unix/Linux/macOS**:
```bash
go build -o agent
```

**Windows**:
```cmd
go build -o agent.exe
```

### 4. Run

**Unix/Linux/macOS**:
```bash
./agent
```

**Windows**:
```cmd
agent.exe
```

## Development

**Run without building**:
```bash
go run main.go
```

**Code verification**:
```bash
go vet ./...
```

**Install dependencies**:
```bash
go mod tidy
```

## Usage

Once started, the agent will launch an interactive terminal interface where you can interact with the AI assistant for code editing tasks.

---

> **Note**: Make sure your API key has sufficient quota and permissions for Gemini API access.