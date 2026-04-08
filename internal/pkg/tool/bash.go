package tool

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"go.muehmer.eu/lai/internal/pkg/security"
)

const (
	maxOutputBytes = 100 * 1024 // 100 KB
	defaultTimeout = 120        // seconds
)

// BashTool executes shell commands via bash.
type BashTool struct{}

func (t *BashTool) Name() string        { return "bash" }
func (t *BashTool) Cacheable() bool      { return false }
func (t *BashTool) Description() string {
	return "Execute a shell command via bash. Returns combined stdout and stderr. " +
		"Use for running programs, installing packages, or system operations."
}

func (t *BashTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "The shell command to execute.",
			},
			"timeout": map[string]any{
				"type":        "integer",
				"description": fmt.Sprintf("Maximum execution time in seconds. Defaults to %d.", defaultTimeout),
			},
		},
		"required": []string{"command"},
	}
}

func (t *BashTool) Execute(args map[string]any) string {
	command, _ := args["command"].(string)
	if command == "" {
		return "Error: command is required and must be a non-empty string."
	}

	timeout := defaultTimeout
	if v, ok := args["timeout"]; ok {
		if n, ok := ToInt(v); ok {
			timeout = n
		}
	}

	if security.IsCommandDangerous(command) {
		return "Error: command blocked by security policy."
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	cmd.Env = security.SanitizedEnv()

	output, err := cmd.CombinedOutput()

	result := string(output)
	if len(result) > maxOutputBytes {
		result = result[:maxOutputBytes] + "\n... [output truncated at 100KB]"
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Sprintf("%s\nError: command timed out after %d seconds.", result, timeout)
		}
		if result == "" {
			return fmt.Sprintf("Error: %v", err)
		}
		return strings.TrimRight(result, "\n")
	}

	if result == "" {
		return "(no output)"
	}
	return strings.TrimRight(result, "\n")
}
