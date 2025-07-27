package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"syscall"

	"agent/internal/agent"
	"agent/internal/schema"
	"runtime"
)

// RunShellCommandInput defines the input parameters for the run_shell_command tool
type RunShellCommandInput struct {
	Command   string `json:"command" jsonschema_description:"The shell command to execute."`
	Directory string `json:"directory,omitempty" jsonschema_description:"The directory to run the command in. Defaults to the current directory."`
}

// RunShellCommandOutput defines the output of the run_shell_command tool
type RunShellCommandOutput struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
	Error    string `json:"error,omitempty"`
}

// RunShellCommandDefinition provides the run_shell_command tool definition
var RunShellCommandDefinition = agent.ToolDefinition{
	Name: "run_shell_command",
	Description: `Executes a shell command.
**DANGER**: This tool allows the execution of arbitrary shell commands. This can be very dangerous. Only use it with trusted commands.
The command is executed within a bash shell.
It returns the stdout, stderr, and exit code.`,
	InputSchema: schema.GenerateSchema[RunShellCommandInput](),
	Function:    RunShellCommand,
}

// RunShellCommand executes a shell command and returns its output.
func RunShellCommand(input json.RawMessage) (string, error) {
	var runShellCommandInput RunShellCommandInput
	err := json.Unmarshal(input, &runShellCommandInput)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal input: %w", err)
	}

	if runShellCommandInput.Command == "" {
		return "", fmt.Errorf("command cannot be empty")
	}

	var shell, shellArg string
	if runtime.GOOS == "windows" {
		shell = "cmd"
		shellArg = "/c"
	} else {
		shell = "sh"
		shellArg = "-c"
	}

	cmd := exec.Command(shell, shellArg, runShellCommandInput.Command)

	if runShellCommandInput.Directory != "" {
		cmd.Dir = runShellCommandInput.Directory
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

	output := RunShellCommandOutput{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: 0,
	}

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
				output.ExitCode = status.ExitStatus()
			} else {
				output.ExitCode = -1
			}
			output.Error = exitError.Error()
		} else {
			output.ExitCode = -1
			output.Error = err.Error()
		}
	}

	resultJSON, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal shell command output: %w", err)
	}

	return string(resultJSON), nil
}
