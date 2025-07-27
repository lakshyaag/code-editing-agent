package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"agent/internal/agent"
	"agent/internal/schema"
)

// SearchFileInput defines the input parameters for the search_file tool
type SearchFileInput struct {
	Path          string `json:"path" jsonschema_description:"The relative path of the file to search in."`
	Query         string `json:"query" jsonschema_description:"The string or regex pattern to search for."`
	IsRegex       bool   `json:"is_regex,omitempty" jsonschema_description:"Treat the query as a regular expression. Defaults to false."`
	CaseSensitive bool   `json:"case_sensitive,omitempty" jsonschema_description:"Perform a case-sensitive search. Defaults to false."`
	Line          int    `json:"line,omitempty" jsonschema_description:"If provided, only this line number will be searched."`
}

// SearchFileResult defines the structure of a search result
type SearchFileResult struct {
	LineNumber int    `json:"line_number"`
	Line       string `json:"line"`
}

// SearchFileDefinition provides the search_file tool definition
var SearchFileDefinition = agent.ToolDefinition{
	Name:        "search_file",
	Description: "Search for a string or regex pattern in a file. Returns a list of matching lines with their line numbers.",
	InputSchema: schema.GenerateSchema[SearchFileInput](),
	Function:    SearchFile,
}

// SearchFile searches for a query string in a file and returns matching lines.
func SearchFile(input json.RawMessage) (string, error) {
	var searchFileInput SearchFileInput
	err := json.Unmarshal(input, &searchFileInput)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal input: %w", err)
	}

	if searchFileInput.Path == "" || searchFileInput.Query == "" {
		return "", fmt.Errorf("path and query must be provided")
	}

	content, err := os.ReadFile(searchFileInput.Path)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", searchFileInput.Path, err)
	}

	lines := strings.Split(string(content), "\n")
	var results []SearchFileResult
	var matcher func(string) bool

	if searchFileInput.IsRegex {
		query := searchFileInput.Query
		if !searchFileInput.CaseSensitive {
			query = "(?i)" + query
		}
		re, err := regexp.Compile(query)
		if err != nil {
			return "", fmt.Errorf("invalid regular expression: %w", err)
		}
		matcher = re.MatchString
	} else {
		matcher = func(line string) bool {
			if !searchFileInput.CaseSensitive {
				return strings.Contains(strings.ToLower(line), strings.ToLower(searchFileInput.Query))
			}
			return strings.Contains(line, searchFileInput.Query)
		}
	}

	for i, line := range lines {
		lineNumber := i + 1
		if searchFileInput.Line != 0 && searchFileInput.Line != lineNumber {
			continue
		}

		if matcher(line) {
			results = append(results, SearchFileResult{
				LineNumber: lineNumber,
				Line:       line,
			})
		}
	}

	resultJSON, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal search results: %w", err)
	}

	return string(resultJSON), nil
}
