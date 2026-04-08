package agent

import "fmt"

// RateLimiter tracks tool execution counts to prevent runaway loops.
type RateLimiter struct {
	maxCallsPerTurn int
	maxCallsTotal   int
	maxBashPerTurn  int
	turnCalls       int
	totalCalls      int
	turnBash        int
}

// NewRateLimiter creates a rate limiter with the given limits.
func NewRateLimiter(maxPerTurn, maxTotal, maxBash int) *RateLimiter {
	return &RateLimiter{
		maxCallsPerTurn: maxPerTurn,
		maxCallsTotal:   maxTotal,
		maxBashPerTurn:  maxBash,
	}
}

// Check verifies that executing the named tool is within limits.
// Returns an error describing which limit was hit, or nil if allowed.
func (r *RateLimiter) Check(toolName string) error {
	if r.turnCalls >= r.maxCallsPerTurn {
		return fmt.Errorf("rate limit: %d tool calls per turn exceeded (max %d)",
			r.turnCalls, r.maxCallsPerTurn)
	}
	if r.totalCalls >= r.maxCallsTotal {
		return fmt.Errorf("rate limit: %d total tool calls exceeded (max %d). Use /clear to reset",
			r.totalCalls, r.maxCallsTotal)
	}
	if toolName == "bash" && r.turnBash >= r.maxBashPerTurn {
		return fmt.Errorf("rate limit: %d bash calls per turn exceeded (max %d)",
			r.turnBash, r.maxBashPerTurn)
	}
	return nil
}

// Record increments counters after a successful tool execution.
func (r *RateLimiter) Record(toolName string) {
	r.turnCalls++
	r.totalCalls++
	if toolName == "bash" {
		r.turnBash++
	}
}

// ResetTurn resets per-turn counters. Called at start of each agent.Loop().
func (r *RateLimiter) ResetTurn() {
	r.turnCalls = 0
	r.turnBash = 0
}

// ResetAll resets all counters. Called on /clear.
func (r *RateLimiter) ResetAll() {
	r.turnCalls = 0
	r.totalCalls = 0
	r.turnBash = 0
}

// Stats returns current turn and total call counts.
func (r *RateLimiter) Stats() (turn, total int) {
	return r.turnCalls, r.totalCalls
}
