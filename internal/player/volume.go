package player

import "math"

// LinearToDecibel converts a linear volume (0.0-1.0) to decibels.
// Uses a quadratic curve for natural volume perception.
// Returns the dB value and whether the output should be silent.
func LinearToDecibel(linear float64) (db float64, silent bool) {
	switch {
	case linear <= 0:
		return -60.0, true
	case linear < 0.001:
		return -60.0, false
	default:
		adjusted := linear * linear
		db = 20.0 * math.Log10(adjusted)
		if db < -60.0 {
			db = -60.0
		}
		return db, false
	}
}

// DecibelToLinear converts a decibel volume back to linear (0.0-1.0).
func DecibelToLinear(db float64) float64 {
	return math.Pow(10, db/40.0)
}

// ClampVolume clamps a volume value to the valid range [0.0, 1.0].
func ClampVolume(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1.0 {
		return 1.0
	}
	return v
}
