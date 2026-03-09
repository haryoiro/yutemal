package player

import (
	"math"
	"testing"
)

func TestLinearToDecibelSilent(t *testing.T) {
	db, silent := LinearToDecibel(0.0)
	if !silent {
		t.Error("volume 0 should be silent")
	}
	if db != -60.0 {
		t.Errorf("db: got %f, want -60.0", db)
	}
}

func TestLinearToDecibelNegative(t *testing.T) {
	db, silent := LinearToDecibel(-0.5)
	if !silent {
		t.Error("negative volume should be silent")
	}
	if db != -60.0 {
		t.Errorf("db: got %f, want -60.0", db)
	}
}

func TestLinearToDecibelNearZero(t *testing.T) {
	db, silent := LinearToDecibel(0.0005)
	if silent {
		t.Error("near-zero should not be silent flag")
	}
	if db != -60.0 {
		t.Errorf("db: got %f, want -60.0", db)
	}
}

func TestLinearToDecibelFull(t *testing.T) {
	db, silent := LinearToDecibel(1.0)
	if silent {
		t.Error("full volume should not be silent")
	}
	// 1.0^2 = 1.0, 20*log10(1.0) = 0
	if db != 0.0 {
		t.Errorf("db: got %f, want 0.0", db)
	}
}

func TestLinearToDecibelHalf(t *testing.T) {
	db, silent := LinearToDecibel(0.5)
	if silent {
		t.Error("half volume should not be silent")
	}
	// 0.5^2 = 0.25, 20*log10(0.25) ≈ -12.04
	expected := 20.0 * math.Log10(0.25)
	if math.Abs(db-expected) > 0.001 {
		t.Errorf("db: got %f, want %f", db, expected)
	}
}

func TestDecibelToLinear(t *testing.T) {
	tests := []struct {
		db       float64
		expected float64
	}{
		{0.0, 1.0},
		{-60.0, math.Pow(10, -60.0/40.0)},
	}

	for _, tt := range tests {
		got := DecibelToLinear(tt.db)
		if math.Abs(got-tt.expected) > 0.0001 {
			t.Errorf("DecibelToLinear(%f): got %f, want %f", tt.db, got, tt.expected)
		}
	}
}

func TestVolumeRoundTrip(t *testing.T) {
	// Test that linear → dB → linear is approximately identity
	testVolumes := []float64{0.1, 0.25, 0.5, 0.7, 0.9, 1.0}

	for _, v := range testVolumes {
		db, _ := LinearToDecibel(v)
		back := DecibelToLinear(db)
		if math.Abs(back-v) > 0.01 {
			t.Errorf("RoundTrip(%f): got %f (db=%f)", v, back, db)
		}
	}
}

func TestClampVolume(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{-0.5, 0.0},
		{0.0, 0.0},
		{0.5, 0.5},
		{1.0, 1.0},
		{1.5, 1.0},
	}

	for _, tt := range tests {
		got := ClampVolume(tt.input)
		if got != tt.expected {
			t.Errorf("ClampVolume(%f): got %f, want %f", tt.input, got, tt.expected)
		}
	}
}
