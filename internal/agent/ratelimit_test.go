package agent

import (
	"testing"
)

func TestRateLimiterAllowsWithinLimits(t *testing.T) {
	rl := NewRateLimiter(5, 100, 3)

	for i := range 4 {
		if err := rl.Check("read"); err != nil {
			t.Fatalf("call %d should be allowed: %v", i, err)
		}
		rl.Record("read")
	}
}

func TestRateLimiterPerTurnLimit(t *testing.T) {
	rl := NewRateLimiter(3, 100, 10)

	for range 3 {
		if err := rl.Check("read"); err != nil {
			t.Fatal(err)
		}
		rl.Record("read")
	}

	if err := rl.Check("read"); err == nil {
		t.Error("4th call should hit per-turn limit")
	}
}

func TestRateLimiterTotalLimit(t *testing.T) {
	rl := NewRateLimiter(100, 5, 100)

	for range 5 {
		if err := rl.Check("read"); err != nil {
			t.Fatal(err)
		}
		rl.Record("read")
	}

	if err := rl.Check("read"); err == nil {
		t.Error("6th call should hit total limit")
	}
}

func TestRateLimiterBashLimit(t *testing.T) {
	rl := NewRateLimiter(100, 100, 2)

	for range 2 {
		if err := rl.Check("bash"); err != nil {
			t.Fatal(err)
		}
		rl.Record("bash")
	}

	if err := rl.Check("bash"); err == nil {
		t.Error("3rd bash should hit bash-per-turn limit")
	}

	// Non-bash should still work
	if err := rl.Check("read"); err != nil {
		t.Errorf("read should not be affected by bash limit: %v", err)
	}
}

func TestRateLimiterResetTurn(t *testing.T) {
	rl := NewRateLimiter(2, 100, 1)

	rl.Record("bash")
	rl.Record("read")
	if err := rl.Check("read"); err == nil {
		t.Error("should be at per-turn limit")
	}

	rl.ResetTurn()

	if err := rl.Check("bash"); err != nil {
		t.Errorf("after ResetTurn, bash should work: %v", err)
	}

	// Total should still be 2
	_, total := rl.Stats()
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
}

func TestRateLimiterResetAll(t *testing.T) {
	rl := NewRateLimiter(10, 10, 10)

	for range 5 {
		rl.Record("bash")
	}

	rl.ResetAll()
	turn, total := rl.Stats()
	if turn != 0 || total != 0 {
		t.Errorf("after ResetAll: turn=%d, total=%d", turn, total)
	}
}

func TestRateLimiterStats(t *testing.T) {
	rl := NewRateLimiter(100, 100, 100)

	rl.Record("read")
	rl.Record("bash")
	rl.Record("read")

	turn, total := rl.Stats()
	if turn != 3 {
		t.Errorf("turn = %d, want 3", turn)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
}
