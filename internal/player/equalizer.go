package player

import (
	"math"
	"sync"

	"github.com/faiface/beep"
)

// NumBands is the number of EQ bands.
const NumBands = 10

// DefaultBandFrequencies are the standard ISO octave center frequencies.
var DefaultBandFrequencies = [NumBands]float64{
	31.25, 62.5, 125, 250, 500, 1000, 2000, 4000, 8000, 16000,
}

// BandLabels are display labels for each band.
var BandLabels = [NumBands]string{
	"31", "63", "125", "250", "500", "1k", "2k", "4k", "8k", "16k",
}

// EQPreset maps a name to 10 band gains in dB.
type EQPreset struct {
	Name  string
	Gains [NumBands]float64
}

// EQPresets contains built-in equalizer presets.
var EQPresets = map[string]EQPreset{
	"flat":         {Name: "Flat", Gains: [10]float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
	"bass_boost":   {Name: "Bass Boost", Gains: [10]float64{6, 5, 4, 2, 0, 0, 0, 0, 0, 0}},
	"treble_boost": {Name: "Treble Boost", Gains: [10]float64{0, 0, 0, 0, 0, 0, 2, 4, 5, 6}},
	"vocal":        {Name: "Vocal", Gains: [10]float64{-2, -1, 0, 2, 4, 4, 2, 0, -1, -2}},
	"rock":         {Name: "Rock", Gains: [10]float64{4, 3, 1, 0, -1, -1, 0, 2, 3, 4}},
	"electronic":   {Name: "Electronic", Gains: [10]float64{5, 4, 2, 0, -2, 0, 2, 4, 4, 3}},
	"acoustic":     {Name: "Acoustic", Gains: [10]float64{3, 2, 1, 0, 1, 1, 2, 3, 2, 1}},
}

// PresetOrder defines the display order for cycling through presets.
var PresetOrder = []string{"flat", "bass_boost", "treble_boost", "vocal", "rock", "electronic", "acoustic"}

// biquadState holds per-channel delay-line state for a single biquad section.
type biquadState struct {
	x1, x2 float64
	y1, y2 float64
}

// biquadCoeffs holds normalized biquad filter coefficients.
type biquadCoeffs struct {
	b0, b1, b2 float64
	a1, a2     float64
}

// biquadFilter represents a single parametric EQ band with stereo state.
type biquadFilter struct {
	coeffs biquadCoeffs
	state  [2]biquadState // 0=left, 1=right
}

// Equalizer implements beep.Streamer, applying a 10-band parametric EQ.
type Equalizer struct {
	streamer   beep.Streamer
	mu         sync.Mutex
	bands      [NumBands]biquadFilter
	gains      [NumBands]float64
	sampleRate float64
	enabled    bool
}

// NewEqualizer creates a new 10-band equalizer wrapping the given streamer.
func NewEqualizer(streamer beep.Streamer, sampleRate float64) *Equalizer {
	eq := &Equalizer{
		streamer:   streamer,
		sampleRate: sampleRate,
		enabled:    true,
	}
	for i := range NumBands {
		eq.bands[i].coeffs = calcPeakingEQ(DefaultBandFrequencies[i], 0, 1.414, sampleRate)
	}
	return eq
}

// Stream implements beep.Streamer. It reads from the underlying streamer
// and applies all 10 biquad filters in series to each sample.
func (eq *Equalizer) Stream(samples [][2]float64) (n int, ok bool) {
	n, ok = eq.streamer.Stream(samples)
	if n == 0 || !eq.enabled {
		return n, ok
	}

	eq.mu.Lock()
	for i := range n {
		for band := range NumBands {
			f := &eq.bands[band]
			c := &f.coeffs

			// Left channel
			sl := &f.state[0]
			xl := samples[i][0]
			yl := c.b0*xl + c.b1*sl.x1 + c.b2*sl.x2 - c.a1*sl.y1 - c.a2*sl.y2
			sl.x2, sl.x1 = sl.x1, xl
			sl.y2, sl.y1 = sl.y1, yl
			samples[i][0] = yl

			// Right channel
			sr := &f.state[1]
			xr := samples[i][1]
			yr := c.b0*xr + c.b1*sr.x1 + c.b2*sr.x2 - c.a1*sr.y1 - c.a2*sr.y2
			sr.x2, sr.x1 = sr.x1, xr
			sr.y2, sr.y1 = sr.y1, yr
			samples[i][1] = yr
		}
	}
	eq.mu.Unlock()

	return n, ok
}

// Err implements beep.Streamer.
func (eq *Equalizer) Err() error {
	return eq.streamer.Err()
}

// SetBandGain sets the gain for a single band in dB (-12 to +12).
func (eq *Equalizer) SetBandGain(band int, gainDB float64) {
	if band < 0 || band >= NumBands {
		return
	}
	gainDB = clampGain(gainDB)

	eq.mu.Lock()
	eq.gains[band] = gainDB
	eq.bands[band].coeffs = calcPeakingEQ(DefaultBandFrequencies[band], gainDB, 1.414, eq.sampleRate)
	eq.bands[band].state = [2]biquadState{}
	eq.mu.Unlock()
}

// SetAllGains sets all band gains at once (e.g., for applying a preset).
func (eq *Equalizer) SetAllGains(gains [NumBands]float64) {
	eq.mu.Lock()
	for i := range NumBands {
		g := clampGain(gains[i])
		eq.gains[i] = g
		eq.bands[i].coeffs = calcPeakingEQ(DefaultBandFrequencies[i], g, 1.414, eq.sampleRate)
		eq.bands[i].state = [2]biquadState{}
	}
	eq.mu.Unlock()
}

// GetGains returns a copy of the current band gains.
func (eq *Equalizer) GetGains() [NumBands]float64 {
	eq.mu.Lock()
	gains := eq.gains
	eq.mu.Unlock()
	return gains
}

// SetEnabled enables or disables the equalizer.
func (eq *Equalizer) SetEnabled(enabled bool) {
	eq.mu.Lock()
	eq.enabled = enabled
	eq.mu.Unlock()
}

// IsEnabled returns whether the equalizer is active.
func (eq *Equalizer) IsEnabled() bool {
	eq.mu.Lock()
	enabled := eq.enabled
	eq.mu.Unlock()
	return enabled
}

// UpdateSampleRate recalculates all coefficients for a new sample rate.
func (eq *Equalizer) UpdateSampleRate(sampleRate float64) {
	eq.mu.Lock()
	eq.sampleRate = sampleRate
	for i := range NumBands {
		eq.bands[i].coeffs = calcPeakingEQ(DefaultBandFrequencies[i], eq.gains[i], 1.414, sampleRate)
		eq.bands[i].state = [2]biquadState{}
	}
	eq.mu.Unlock()
}

// ResetState resets filter state to avoid transient pops after seek.
func (eq *Equalizer) ResetState() {
	eq.mu.Lock()
	for i := range NumBands {
		eq.bands[i].state = [2]biquadState{}
	}
	eq.mu.Unlock()
}

// calcPeakingEQ computes normalized biquad coefficients for an RBJ peaking EQ filter.
func calcPeakingEQ(freq, gainDB, q, sampleRate float64) biquadCoeffs {
	// Guard against Nyquist
	if freq >= sampleRate/2 {
		return biquadCoeffs{b0: 1}
	}

	A := math.Pow(10, gainDB/40.0)
	w0 := 2.0 * math.Pi * freq / sampleRate
	sinW0 := math.Sin(w0)
	cosW0 := math.Cos(w0)
	alpha := sinW0 / (2.0 * q)

	b0 := 1.0 + alpha*A
	b1 := -2.0 * cosW0
	b2 := 1.0 - alpha*A
	a0 := 1.0 + alpha/A
	a1 := -2.0 * cosW0
	a2 := 1.0 - alpha/A

	return biquadCoeffs{
		b0: b0 / a0,
		b1: b1 / a0,
		b2: b2 / a0,
		a1: a1 / a0,
		a2: a2 / a0,
	}
}

func clampGain(g float64) float64 {
	if g < -12 {
		return -12
	}
	if g > 12 {
		return 12
	}
	return g
}
