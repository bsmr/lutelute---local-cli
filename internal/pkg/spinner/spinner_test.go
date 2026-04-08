package spinner

import (
	"testing"
	"time"
)

func TestSpinnerStartStop(t *testing.T) {
	s := New("Testing")
	s.Start()
	time.Sleep(200 * time.Millisecond)
	s.Stop()
	// Double stop should not panic
	s.Stop()
}
