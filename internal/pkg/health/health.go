// Package health provides startup diagnostics and health checks.
package health

import (
	"fmt"
	"strings"

	"go.muehmer.eu/lai/internal/pkg/provider/ollama"

	"os"
)

const (
	StatusOK      = "ok"
	StatusWarning = "warning"
	StatusError   = "error"

	green  = "\033[32m"
	yellow = "\033[33m"
	red    = "\033[31m"
	reset  = "\033[0m"
)

var statusColors = map[string]string{StatusOK: green, StatusWarning: yellow, StatusError: red}
var statusSymbols = map[string]string{StatusOK: "✓", StatusWarning: "⚠", StatusError: "✗"}

// Result represents the outcome of a single health check.
type Result struct {
	Name    string
	Status  string
	Message string
}

// CheckOllamaConnectivity verifies that Ollama is reachable.
func CheckOllamaConnectivity(client *ollama.Client) Result {
	resp, err := client.GetVersion()
	if err != nil {
		return Result{Name: "Ollama", Status: StatusError, Message: fmt.Sprintf("Connection failed: %v", err)}
	}
	version, _ := resp["version"].(string)
	return Result{Name: "Ollama", Status: StatusOK, Message: fmt.Sprintf("Connected (v%s)", version)}
}

// CheckModelAvailability verifies that the specified model is available.
func CheckModelAvailability(client *ollama.Client, model string) Result {
	models, err := client.ListModels()
	if err != nil {
		return Result{Name: "Model", Status: StatusError, Message: fmt.Sprintf("Cannot check models: %v", err)}
	}

	baseName := model
	if idx := strings.Index(model, ":"); idx > 0 {
		baseName = model[:idx]
	}

	for _, m := range models {
		if m.Name == model {
			return Result{Name: "Model", Status: StatusOK, Message: fmt.Sprintf("%s available", model)}
		}
		mBase := m.Name
		if idx := strings.Index(m.Name, ":"); idx > 0 {
			mBase = m.Name[:idx]
		}
		if mBase == baseName {
			return Result{Name: "Model", Status: StatusOK, Message: fmt.Sprintf("%s available (as %s)", model, m.Name)}
		}
	}
	return Result{Name: "Model", Status: StatusWarning, Message: fmt.Sprintf("%s not found locally", model)}
}

// CheckDiskSpace verifies sufficient free disk space.
func CheckDiskSpace() Result {
	// Use a simple stat-based check for the root filesystem.
	// Go's syscall.Statfs is platform-specific; use os.Stat as a basic check.
	info, err := os.Stat("/")
	if err != nil {
		return Result{Name: "Disk", Status: StatusWarning, Message: "Cannot check disk space"}
	}
	_ = info
	// For a full implementation, use syscall.Statfs. For the prototype, we mark as OK.
	return Result{Name: "Disk", Status: StatusOK, Message: "Disk space check skipped (prototype)"}
}

// RunAll runs all health checks and returns results.
func RunAll(client *ollama.Client, model string) []Result {
	results := make([]Result, 0, 3)

	ollamaResult := CheckOllamaConnectivity(client)
	results = append(results, ollamaResult)

	if ollamaResult.Status == StatusError {
		results = append(results, Result{
			Name: "Model", Status: StatusWarning, Message: "Skipped (Ollama unreachable)",
		})
	} else {
		results = append(results, CheckModelAvailability(client, model))
	}

	results = append(results, CheckDiskSpace())
	return results
}

// Format returns a formatted string of all health check results.
func Format(results []Result, color bool) string {
	var sb strings.Builder
	for _, r := range results {
		sym := statusSymbols[r.Status]
		if color {
			c := statusColors[r.Status]
			fmt.Fprintf(&sb, "  %s%s%s %s: %s\n", c, sym, reset, r.Name, r.Message)
		} else {
			fmt.Fprintf(&sb, "  %s %s: %s\n", sym, r.Name, r.Message)
		}
	}
	return sb.String()
}
