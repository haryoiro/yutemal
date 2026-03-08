package player

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"time"
	"unsafe"

	"github.com/faiface/beep"
	"github.com/tosone/minimp3"

	"github.com/haryoiro/yutemal/internal/logger"
)

// minimp3Decoder wraps minimp3 decoder to implement beep.StreamSeekCloser.
type minimp3Decoder struct {
	decoder      *minimp3.Decoder
	data         []byte
	format       beep.Format
	position     int // Current position in samples
	TotalSamples int // Will be updated when EOF is discovered
	buffer       []int16
	bufferIndex  int

	// Callback to notify player of actual duration changes
	durationUpdateCallback func(actualSamples int)

	// Debug fields for detecting buffer issues
	underrunCount int
	lastReadTime  time.Time

	// Pre-allocated buffers to reduce GC pressure
	readBuffer   []byte
	decodeBuffer []byte       // Reusable decode input buffer
	pcmBuffer    []int16      // Reusable PCM output buffer
	samplePool   [][2]float64 // Pool of sample buffers
}

// DecodeMiniMP3 decodes an MP3 file using minimp3.
func DecodeMiniMP3(file *os.File) (beep.StreamSeekCloser, beep.Format, error) {
	// Read entire file into memory
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, beep.Format{}, fmt.Errorf("failed to read file: %w", err)
	}

	// Create decoder from bytes reader
	dec, err := minimp3.NewDecoder(bytes.NewReader(data))
	if err != nil {
		return nil, beep.Format{}, fmt.Errorf("failed to create decoder: %w", err)
	}

	// Decode a bit to get format info
	const testSize = 1024
	testBuf := make([]byte, testSize)

	n, err := dec.Read(testBuf)
	if err != nil && err != io.EOF {
		return nil, beep.Format{}, fmt.Errorf("failed to read test data: %w", err)
	}

	if n == 0 {
		return nil, beep.Format{}, fmt.Errorf("no audio data found")
	}

	// Get decoder info
	sampleRate := dec.SampleRate
	channels := dec.Channels

	// Reset decoder
	dec, err = minimp3.NewDecoder(bytes.NewReader(data))
	if err != nil {
		return nil, beep.Format{}, fmt.Errorf("failed to recreate decoder: %w", err)
	}

	format := beep.Format{
		SampleRate:  beep.SampleRate(sampleRate),
		NumChannels: channels,
		Precision:   2, // 16-bit
	}

	// Start with conservative estimate - will be updated when we reach actual EOF
	// or from ffprobe if available
	totalSamples := sampleRate * 60 * 5 // 5 minutes default - prevents seeking beyond actual audio

	logger.Debug("minimp3: Created decoder for %d Hz, %d channels, initial estimate: %d samples",
		sampleRate, channels, totalSamples)

	// Pre-allocate buffers to reduce GC pressure
	const maxDecodeSize = 4608 * 2 // Max MP3 frame size * 2 for stereo

	return &minimp3Decoder{
		decoder:                dec,
		data:                   data,
		format:                 format,
		position:               0,
		TotalSamples:           totalSamples,
		buffer:                 make([]int16, 0),
		bufferIndex:            0,
		durationUpdateCallback: nil,
		// Pre-allocated buffers
		readBuffer:   make([]byte, 262144), // 256KB read buffer (increased from 64KB)
		decodeBuffer: make([]byte, maxDecodeSize),
		pcmBuffer:    make([]int16, maxDecodeSize),
		samplePool:   make([][2]float64, 8192), // Pool of samples
	}, format, nil
}

// Stream streams audio samples using batch conversion for better performance.
// Instead of converting one sample at a time, this processes all available
// buffered samples in a tight loop, enabling compiler auto-vectorization.
func (d *minimp3Decoder) Stream(samples [][2]float64) (n int, ok bool) {
	for n < len(samples) {
		// Refill internal buffer if exhausted
		if d.bufferIndex >= len(d.buffer) {
			if !d.refillBuffer(samples, n) {
				return n, n > 0
			}
		}

		// Batch convert as many samples as possible from the current buffer
		var filled int
		if d.format.NumChannels == 1 {
			filled = d.fillMonoBatch(samples[n:])
		} else {
			filled = d.fillStereoBatch(samples[n:])
		}
		if filled == 0 {
			break
		}
		n += filled
	}

	return n, n > 0
}

// refillBuffer refills the internal buffer when needed.
func (d *minimp3Decoder) refillBuffer(samples [][2]float64, startIndex int) bool {
	d.detectUnderrun()

	// Read more data - use pre-allocated buffer to reduce GC pressure
	const readSize = 262144 // 256KB buffer for smoother playback (increased from 64KB)
	if d.readBuffer == nil {
		d.readBuffer = make([]byte, readSize)
	}

	buf := d.readBuffer[:readSize]
	bytesRead, err := d.readWithTiming(buf)

	if err == io.EOF || bytesRead == 0 {
		d.handleEOF(samples, startIndex)
		return false
	}

	d.convertBytesToSamples(buf, bytesRead)

	return true
}

// detectUnderrun detects potential buffer underruns.
func (d *minimp3Decoder) detectUnderrun() {
	now := time.Now()
	if !d.lastReadTime.IsZero() {
		elapsed := now.Sub(d.lastReadTime)
		// Only log if reads are happening too frequently (< 50μs is suspicious for actual underrun)
		// Changed from 500μs to 50μs to only detect real underruns
		if elapsed < 50*time.Microsecond {
			d.underrunCount++
			if d.underrunCount%10000 == 0 { // Increased from 1000 to 10000 to reduce log spam
				logger.Debug("Very frequent decoder reads detected: %d occurrences (elapsed: %v)", d.underrunCount, elapsed)
			}
		}
	}

	d.lastReadTime = now
}

// readWithTiming reads data with timing measurement.
func (d *minimp3Decoder) readWithTiming(buf []byte) (int, error) {
	startRead := time.Now()
	bytesRead, err := d.decoder.Read(buf)
	readDuration := time.Since(startRead)

	if readDuration > time.Millisecond*50 {
		logger.Debug("Slow MP3 decode: took %v to read %d bytes", readDuration, bytesRead)
	}

	return bytesRead, err
}

// handleEOF handles end of file condition.
func (d *minimp3Decoder) handleEOF(samples [][2]float64, startIndex int) {
	// We've reached the end - update total samples to actual length
	actualTotalSamples := d.position
	if actualTotalSamples != d.TotalSamples {
		logger.Debug("minimp3: EOF reached, correcting total samples from %d to %d (%.2fs)",
			d.TotalSamples, actualTotalSamples, float64(actualTotalSamples)/float64(d.format.SampleRate))

		d.TotalSamples = actualTotalSamples

		// Notify player of actual duration
		if d.durationUpdateCallback != nil {
			d.durationUpdateCallback(d.TotalSamples)
		}
	}

	// Fill remaining samples with silence for smooth ending
	for j := startIndex; j < len(samples); j++ {
		samples[j][0] = 0
		samples[j][1] = 0
	}
}

// convertBytesToSamples converts bytes to int16 samples using zero-copy reinterpretation.
// This is safe on little-endian architectures (AMD64, ARM64) where the byte layout
// of []byte matches the in-memory representation of []int16.
func (d *minimp3Decoder) convertBytesToSamples(buf []byte, bytesRead int) {
	requiredSize := bytesRead / 2

	// Zero-copy: reinterpret the byte slice as int16 slice directly.
	// Both ARM64 and AMD64 are little-endian, so buf[0]=low, buf[1]=high
	// matches int16 memory layout. This eliminates the conversion loop entirely.
	d.buffer = unsafe.Slice((*int16)(unsafe.Pointer(&buf[0])), requiredSize)

	d.bufferIndex = 0
}

// fillMonoBatch converts available mono int16 samples to [][2]float64 in bulk.
// Returns the number of samples filled. The tight loop with constant scaling
// factor enables compiler auto-vectorization (NEON on ARM64, SSE on AMD64).
func (d *minimp3Decoder) fillMonoBatch(samples [][2]float64) int {
	available := len(d.buffer) - d.bufferIndex
	count := min(available, len(samples))

	const scale = 1.0 / 32768.0
	buf := d.buffer[d.bufferIndex : d.bufferIndex+count]
	for i, s := range buf {
		v := float64(s) * scale
		samples[i][0] = v
		samples[i][1] = v
	}

	d.bufferIndex += count
	d.position += count
	return count
}

// fillStereoBatch converts available stereo int16 samples to [][2]float64 in bulk.
// Processes interleaved L/R pairs. Returns the number of stereo frames filled.
func (d *minimp3Decoder) fillStereoBatch(samples [][2]float64) int {
	availablePairs := (len(d.buffer) - d.bufferIndex) / 2
	count := min(availablePairs, len(samples))

	const scale = 1.0 / 32768.0
	idx := d.bufferIndex
	for i := range count {
		samples[i][0] = float64(d.buffer[idx]) * scale
		samples[i][1] = float64(d.buffer[idx+1]) * scale
		idx += 2
	}

	d.bufferIndex = idx
	d.position += count
	return count
}

// Err returns any error.
func (d *minimp3Decoder) Err() error {
	return nil
}

// Len returns the total number of samples.
func (d *minimp3Decoder) Len() int {
	return d.TotalSamples
}

// Position returns current position in samples.
func (d *minimp3Decoder) Position() int {
	return d.position
}

// Seek seeks to a position in samples - simplified approach.
func (d *minimp3Decoder) Seek(p int) error {
	// Clamp to valid range
	if p < 0 {
		p = 0
	}

	if p >= d.TotalSamples {
		p = d.TotalSamples - 1
	}

	// For seeking near the beginning, just reset and decode forward
	if p < d.TotalSamples/50 { // First 2% of file (about 6 seconds for a 5-minute song)
		return d.seekFromBeginning(p)
	}

	// For later positions, try byte-based approximation then decode forward
	return d.seekApproximate(p)
}

// seekFromBeginning resets decoder and reads forward to target position.
func (d *minimp3Decoder) seekFromBeginning(targetPos int) error {
	// Reset decoder
	dec, err := minimp3.NewDecoder(bytes.NewReader(d.data))
	if err != nil {
		return fmt.Errorf("failed to recreate decoder: %w", err)
	}

	d.decoder = dec
	d.position = 0
	d.buffer = make([]int16, 0)
	d.bufferIndex = 0

	if targetPos <= 0 {
		return nil
	}

	// Skip forward to target position with larger buffer for efficiency
	skipBuffer := make([]byte, 32768) // Increased from 8KB to 32KB for faster skipping
	samplesToSkip := targetPos

	for samplesToSkip > 0 {
		bytesRead, readErr := d.decoder.Read(skipBuffer)
		if readErr == io.EOF || bytesRead == 0 {
			break
		}

		samplesRead := min(bytesRead/(2*d.format.NumChannels), samplesToSkip)

		samplesToSkip -= samplesRead
		d.position += samplesRead
	}

	return nil
}

// seekApproximate uses byte-based estimation for faster seeking to later positions.
func (d *minimp3Decoder) seekApproximate(targetPos int) error {
	// Use simple byte-based estimation
	targetRatio := float64(targetPos) / float64(d.TotalSamples)
	estimatedBytePos := max(
		// Clamp to safe range
		min(

			int(targetRatio*float64(len(d.data))), len(d.data)-1024), 0)

	// Create decoder from estimated position
	dec, err := minimp3.NewDecoder(bytes.NewReader(d.data[estimatedBytePos:]))
	if err != nil {
		// Fallback to beginning if estimation fails
		return d.seekFromBeginning(targetPos)
	}

	d.decoder = dec
	d.position = int(targetRatio * float64(d.TotalSamples))
	d.buffer = make([]int16, 0)
	d.bufferIndex = 0

	return nil
}

// Close closes the decoder.
func (d *minimp3Decoder) Close() error {
	return nil
}
