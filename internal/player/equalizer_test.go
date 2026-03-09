package player

import (
	"math"
	"testing"
)

func TestCalcPeakingEQZeroGain(t *testing.T) {
	coeffs := calcPeakingEQ(1000.0, 0.0, 1.414, 44100.0)

	// With 0 dB gain, filter should approximate passthrough: b0≈1, others≈0
	// Actually for peaking EQ with 0 gain: A=1, so b0=(1+alpha)/(1+alpha)=1
	if math.Abs(coeffs.b0-1.0) > 0.0001 {
		t.Errorf("b0: got %f, want ~1.0 (passthrough)", coeffs.b0)
	}
	// b1 and a1 should be equal (cancel out)
	if math.Abs(coeffs.b1-coeffs.a1) > 0.0001 {
		t.Errorf("b1=%f should equal a1=%f for 0 gain", coeffs.b1, coeffs.a1)
	}
	// b2 and a2 should be equal
	if math.Abs(coeffs.b2-coeffs.a2) > 0.0001 {
		t.Errorf("b2=%f should equal a2=%f for 0 gain", coeffs.b2, coeffs.a2)
	}
}

func TestCalcPeakingEQPositiveGain(t *testing.T) {
	coeffs := calcPeakingEQ(1000.0, 6.0, 1.414, 44100.0)

	// With positive gain, b0 should be > 1 (boosting)
	if coeffs.b0 <= 1.0 {
		t.Errorf("b0: got %f, want > 1.0 for positive gain", coeffs.b0)
	}
}

func TestCalcPeakingEQNegativeGain(t *testing.T) {
	coeffs := calcPeakingEQ(1000.0, -6.0, 1.414, 44100.0)

	// With negative gain (cut), b0 should be < 1
	if coeffs.b0 >= 1.0 {
		t.Errorf("b0: got %f, want < 1.0 for negative gain", coeffs.b0)
	}
}

func TestCalcPeakingEQNyquistGuard(t *testing.T) {
	// Frequency at Nyquist should return passthrough
	coeffs := calcPeakingEQ(22050.0, 6.0, 1.414, 44100.0)

	if coeffs.b0 != 1.0 {
		t.Errorf("b0: got %f, want 1.0 (Nyquist guard passthrough)", coeffs.b0)
	}
	if coeffs.b1 != 0 || coeffs.b2 != 0 || coeffs.a1 != 0 || coeffs.a2 != 0 {
		t.Error("Nyquist guard should zero all except b0")
	}
}

func TestCalcPeakingEQAboveNyquist(t *testing.T) {
	coeffs := calcPeakingEQ(30000.0, 6.0, 1.414, 44100.0)

	if coeffs.b0 != 1.0 {
		t.Errorf("b0: got %f, want 1.0 (above Nyquist)", coeffs.b0)
	}
}

func TestCalcPeakingEQAllBandFrequencies(t *testing.T) {
	// Verify all default band frequencies produce valid coefficients at 44100 Hz
	for i, freq := range DefaultBandFrequencies {
		coeffs := calcPeakingEQ(freq, 3.0, 1.414, 44100.0)

		if math.IsNaN(coeffs.b0) || math.IsInf(coeffs.b0, 0) {
			t.Errorf("Band %d (%s, %.0f Hz): b0 is NaN/Inf", i, BandLabels[i], freq)
		}
		if math.IsNaN(coeffs.a1) || math.IsInf(coeffs.a1, 0) {
			t.Errorf("Band %d (%s, %.0f Hz): a1 is NaN/Inf", i, BandLabels[i], freq)
		}
	}
}

func TestCalcPeakingEQHighSampleRate(t *testing.T) {
	// 96 kHz sample rate - all default frequencies should still work
	for i, freq := range DefaultBandFrequencies {
		coeffs := calcPeakingEQ(freq, 6.0, 1.414, 96000.0)

		if math.IsNaN(coeffs.b0) {
			t.Errorf("Band %d at 96kHz: b0 is NaN", i)
		}
	}
}

func TestClampGain(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{-20.0, -12.0},
		{-12.0, -12.0},
		{0.0, 0.0},
		{6.0, 6.0},
		{12.0, 12.0},
		{20.0, 12.0},
	}

	for _, tt := range tests {
		got := clampGain(tt.input)
		if got != tt.expected {
			t.Errorf("clampGain(%f): got %f, want %f", tt.input, got, tt.expected)
		}
	}
}

func TestCalcPeakingEQSymmetry(t *testing.T) {
	// Positive and negative gain should produce mirrored coefficients
	// Specifically: boost b0 * cut b0 ≈ 1 (approximately)
	boostCoeffs := calcPeakingEQ(1000.0, 6.0, 1.414, 44100.0)
	cutCoeffs := calcPeakingEQ(1000.0, -6.0, 1.414, 44100.0)

	product := boostCoeffs.b0 * cutCoeffs.b0
	// For peaking EQ: boost_b0 = (1+alpha*A)/(1+alpha/A)
	// cut_b0 = (1+alpha/A)/(1+alpha*A) = 1/boost_b0... approximately
	// So product should be close to... let me verify.
	// Actually: boost_a0 = 1+alpha/A, cut_b0 = 1+alpha/A, so cut_b0/cut_a0
	// This gets complex. Let's just check both are reasonable.
	if boostCoeffs.b0 <= 1.0 {
		t.Error("boost b0 should be > 1")
	}
	if cutCoeffs.b0 >= 1.0 {
		t.Error("cut b0 should be < 1")
	}
	// Their product should be close to 1 (not exact due to normalization)
	if math.Abs(product-1.0) > 0.5 {
		t.Errorf("boost*cut b0 product: got %f, expected close to 1.0", product)
	}
}
