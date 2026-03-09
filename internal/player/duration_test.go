package player

import (
	"testing"
	"time"
)

func TestParseDurationOutputValid(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
	}{
		{"123.456", 123456 * time.Millisecond},
		{"0.5", 500 * time.Millisecond},
		{"300.0", 300 * time.Second},
		{"  42.0  \n", 42 * time.Second},
	}

	for _, tt := range tests {
		got := ParseDurationOutput(tt.input)
		// Allow small rounding differences due to float conversion
		diff := got - tt.expected
		if diff < 0 {
			diff = -diff
		}
		if diff > time.Millisecond {
			t.Errorf("ParseDurationOutput(%q): got %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestParseDurationOutputEmpty(t *testing.T) {
	if d := ParseDurationOutput(""); d != 0 {
		t.Errorf("empty string: got %v, want 0", d)
	}
}

func TestParseDurationOutputNA(t *testing.T) {
	if d := ParseDurationOutput("N/A"); d != 0 {
		t.Errorf("N/A: got %v, want 0", d)
	}
}

func TestParseDurationOutputWhitespace(t *testing.T) {
	if d := ParseDurationOutput("   "); d != 0 {
		t.Errorf("whitespace: got %v, want 0", d)
	}
}

func TestParseDurationOutputInvalid(t *testing.T) {
	if d := ParseDurationOutput("not-a-number"); d != 0 {
		t.Errorf("invalid: got %v, want 0", d)
	}
}

func TestParseDurationOutputNegative(t *testing.T) {
	if d := ParseDurationOutput("-5.0"); d != 0 {
		t.Errorf("negative: got %v, want 0", d)
	}
}

func TestParseDurationOutputZero(t *testing.T) {
	if d := ParseDurationOutput("0.0"); d != 0 {
		t.Errorf("zero: got %v, want 0", d)
	}
}
