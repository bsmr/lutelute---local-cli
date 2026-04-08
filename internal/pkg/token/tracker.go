// Package token provides token usage tracking and cost estimation.
package token

import (
	"fmt"
	"strings"
)

const (
	defaultInputCostPerToken  = 0.000003  // $3.00 per 1M tokens
	defaultOutputCostPerToken = 0.000015  // $15.00 per 1M tokens
)

// Usage records token counts for a single LLM call.
type Usage struct {
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
	Provider     string `json:"provider"`
}

// TotalTokens returns the sum of input and output tokens.
func (u Usage) TotalTokens() int { return u.InputTokens + u.OutputTokens }

// Tracker accumulates token usage across multiple LLM calls.
type Tracker struct {
	records            []Usage
	inputCostPerToken  float64
	outputCostPerToken float64
}

// NewTracker creates a tracker with default cost rates.
func NewTracker() *Tracker {
	return &Tracker{
		inputCostPerToken:  defaultInputCostPerToken,
		outputCostPerToken: defaultOutputCostPerToken,
	}
}

// Record adds a usage entry.
func (t *Tracker) Record(u Usage) {
	t.records = append(t.records, u)
}

// RecordFromOllama extracts token counts from an Ollama response.
func (t *Tracker) RecordFromOllama(resp map[string]any) {
	input, _ := resp["prompt_eval_count"].(float64)
	output, _ := resp["eval_count"].(float64)
	t.Record(Usage{
		InputTokens:  int(input),
		OutputTokens: int(output),
		Provider:     "ollama",
	})
}

// TotalInput returns cumulative input tokens.
func (t *Tracker) TotalInput() int {
	total := 0
	for _, r := range t.records {
		total += r.InputTokens
	}
	return total
}

// TotalOutput returns cumulative output tokens.
func (t *Tracker) TotalOutput() int {
	total := 0
	for _, r := range t.records {
		total += r.OutputTokens
	}
	return total
}

// Total returns cumulative total tokens.
func (t *Tracker) Total() int {
	return t.TotalInput() + t.TotalOutput()
}

// MessageCount returns the number of recorded exchanges.
func (t *Tracker) MessageCount() int { return len(t.records) }

// EstimatedCost returns estimated cost for Claude usage, or nil for local-only.
func (t *Tracker) EstimatedCost() *float64 {
	hasClaude := false
	for _, r := range t.records {
		if r.Provider == "claude" {
			hasClaude = true
			break
		}
	}
	if !hasClaude {
		return nil
	}
	var cost float64
	for _, r := range t.records {
		if r.Provider == "claude" {
			cost += float64(r.InputTokens)*t.inputCostPerToken +
				float64(r.OutputTokens)*t.outputCostPerToken
		}
	}
	return &cost
}

// FormatSummary returns a one-line summary of token usage.
func (t *Tracker) FormatSummary() string {
	costStr := "N/A (local)"
	if c := t.EstimatedCost(); c != nil {
		costStr = fmt.Sprintf("$%.4f", *c)
	}
	return fmt.Sprintf("Tokens: %d in / %d out (%d total) | Est. cost: %s",
		t.TotalInput(), t.TotalOutput(), t.Total(), costStr)
}

// FormatTable returns a multi-line table of per-message usage.
func (t *Tracker) FormatTable() string {
	if len(t.records) == 0 {
		return "No token usage recorded."
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "  # %-10s %8s %8s %8s\n", "Provider", "Input", "Output", "Total")
	fmt.Fprintf(&sb, "  %s\n", strings.Repeat("-", 42))
	for i, r := range t.records {
		fmt.Fprintf(&sb, " %2d %-10s %8d %8d %8d\n",
			i+1, r.Provider, r.InputTokens, r.OutputTokens, r.TotalTokens())
	}
	fmt.Fprintf(&sb, "  %s\n", strings.Repeat("-", 42))
	fmt.Fprintf(&sb, "    %-10s %8d %8d %8d\n",
		"TOTAL", t.TotalInput(), t.TotalOutput(), t.Total())
	return sb.String()
}

// Clear resets all records.
func (t *Tracker) Clear() { t.records = nil }
