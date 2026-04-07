package token

import (
	"strings"
	"testing"
)

func TestTrackerBasics(t *testing.T) {
	tr := NewTracker()
	if tr.Total() != 0 {
		t.Errorf("empty tracker Total = %d", tr.Total())
	}
	if tr.MessageCount() != 0 {
		t.Errorf("empty tracker MessageCount = %d", tr.MessageCount())
	}

	tr.Record(Usage{InputTokens: 100, OutputTokens: 50, Provider: "ollama"})
	tr.Record(Usage{InputTokens: 200, OutputTokens: 100, Provider: "ollama"})

	if tr.TotalInput() != 300 {
		t.Errorf("TotalInput = %d, want 300", tr.TotalInput())
	}
	if tr.TotalOutput() != 150 {
		t.Errorf("TotalOutput = %d, want 150", tr.TotalOutput())
	}
	if tr.Total() != 450 {
		t.Errorf("Total = %d, want 450", tr.Total())
	}
	if tr.MessageCount() != 2 {
		t.Errorf("MessageCount = %d, want 2", tr.MessageCount())
	}
}

func TestEstimatedCostLocalOnly(t *testing.T) {
	tr := NewTracker()
	tr.Record(Usage{InputTokens: 100, OutputTokens: 50, Provider: "ollama"})
	if tr.EstimatedCost() != nil {
		t.Error("local-only should return nil cost")
	}
}

func TestEstimatedCostClaude(t *testing.T) {
	tr := NewTracker()
	tr.Record(Usage{InputTokens: 1000, OutputTokens: 500, Provider: "claude"})
	cost := tr.EstimatedCost()
	if cost == nil {
		t.Fatal("claude usage should return cost")
	}
	if *cost <= 0 {
		t.Errorf("cost should be positive: %f", *cost)
	}
}

func TestFormatSummary(t *testing.T) {
	tr := NewTracker()
	tr.Record(Usage{InputTokens: 100, OutputTokens: 50, Provider: "ollama"})
	summary := tr.FormatSummary()
	if !strings.Contains(summary, "100 in") {
		t.Errorf("summary missing input: %s", summary)
	}
	if !strings.Contains(summary, "N/A (local)") {
		t.Errorf("local usage should show N/A: %s", summary)
	}
}

func TestClear(t *testing.T) {
	tr := NewTracker()
	tr.Record(Usage{InputTokens: 100, OutputTokens: 50, Provider: "ollama"})
	tr.Clear()
	if tr.Total() != 0 {
		t.Errorf("after clear Total = %d", tr.Total())
	}
}
