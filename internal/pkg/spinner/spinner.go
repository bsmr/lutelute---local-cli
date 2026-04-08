// Package spinner provides a terminal spinner animation using goroutines.
package spinner

import (
	"fmt"
	"os"
	"sync"
	"time"
)

const (
	frames   = "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏"
	interval = 80 * time.Millisecond
)

// runeFrames pre-splits the braille frames for indexed access.
var runeFrames = []rune(frames)

// Spinner displays a braille-dot animation on stderr.
type Spinner struct {
	message string
	stop    chan struct{}
	done    chan struct{}
	once    sync.Once
}

// New creates a spinner with the given message.
func New(message string) *Spinner {
	return &Spinner{
		message: message,
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}
}

// Start begins the spinner animation in a background goroutine.
func (s *Spinner) Start() {
	go s.animate()
}

// Stop halts the spinner and clears the line.
func (s *Spinner) Stop() {
	s.once.Do(func() {
		close(s.stop)
		<-s.done
	})
}

func (s *Spinner) animate() {
	defer close(s.done)
	idx := 0
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stop:
			// Clear the spinner line
			fmt.Fprintf(os.Stderr, "\r%*s\r", len(s.message)+10, "")
			return
		case <-ticker.C:
			frame := runeFrames[idx%len(runeFrames)]
			fmt.Fprintf(os.Stderr, "\r  %c %s...", frame, s.message)
			idx++
		}
	}
}
