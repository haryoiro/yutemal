package player

import (
	"fmt"
	"strings"
	"time"
)

// ParseDurationOutput parses ffprobe duration output into a time.Duration.
// Expects a string containing a floating-point number of seconds (e.g., "123.456").
// Returns 0 for empty, "N/A", negative, or unparseable input.
func ParseDurationOutput(output string) time.Duration {
	s := strings.TrimSpace(output)
	if s == "" || s == "N/A" {
		return 0
	}

	var seconds float64
	if _, err := fmt.Sscanf(s, "%f", &seconds); err != nil {
		return 0
	}

	if seconds > 0 {
		return time.Duration(seconds * float64(time.Second))
	}

	return 0
}
