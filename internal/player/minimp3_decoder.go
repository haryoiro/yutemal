package player

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"time"

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
	readBuffer      []byte
	decodeBuffer    []byte       // Reusable decode input buffer
	pcmBuffer       []int16      // Reusable PCM output buffer
	samplePool      [][2]float64 // Pool of sample buffers
	samplePoolIndex int
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
		readBuffer:   make([]byte, 65536), // 64KB read buffer
		decodeBuffer: make([]byte, maxDecodeSize),
		pcmBuffer:    make([]int16, maxDecodeSize),
		samplePool:   make([][2]float64, 8192), // Pool of samples
	}, format, nil
}

// Stream streams audio samples.
func (d *minimp3Decoder) Stream(samples [][2]float64) (n int, ok bool) {
	for i := range samples {
		if !d.fillSample(samples, i) {
			return i, i > 0
		}
	}

	return len(samples), true
}

// fillSample fills a single sample.
func (d *minimp3Decoder) fillSample(samples [][2]float64, index int) bool {
	// Need more data?
	if d.bufferIndex >= len(d.buffer) {
		if !d.refillBuffer(samples, index) {
			return false
		}
	}

	// Convert to float64 samples
	if d.format.NumChannels == 1 {
		d.fillMonoSample(samples, index)
	} else {
		d.fillStereoSample(samples, index)
	}

	return true
}

// refillBuffer refills the internal buffer when needed.
func (d *minimp3Decoder) refillBuffer(samples [][2]float64, startIndex int) bool {
	d.detectUnderrun()

	// Read more data - use pre-allocated buffer to reduce GC pressure
	const readSize = 65536 // 64KB buffer for smoother playback
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
		if elapsed < time.Millisecond*10 {
			d.underrunCount++
			if d.underrunCount%10 == 0 {
				logger.Debug("Potential buffer underrun detected: %d occurrences", d.underrunCount)
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

// convertBytesToSamples converts bytes to int16 samples.
func (d *minimp3Decoder) convertBytesToSamples(buf []byte, bytesRead int) {
	// Convert bytes to int16 samples - reuse pre-allocated buffer
	requiredSize := bytesRead / 2

	// Use pre-allocated pcmBuffer if it's large enough
	if requiredSize <= len(d.pcmBuffer) {
		d.buffer = d.pcmBuffer[:requiredSize]
	} else {
		// Only allocate if we need more than pre-allocated
		if cap(d.buffer) < requiredSize {
			logger.Debug("Allocating larger buffer: %d samples (was %d)", requiredSize, cap(d.buffer))
			d.buffer = make([]int16, requiredSize)
		} else {
			d.buffer = d.buffer[:requiredSize]
		}
	}

	// Use optimized loop for byte-to-int16 conversion
	for j := 0; j < requiredSize; j++ {
		d.buffer[j] = int16(buf[j*2]) | (int16(buf[j*2+1]) << 8)
	}

	d.bufferIndex = 0
}

// fillMonoSample fills a mono sample.
func (d *minimp3Decoder) fillMonoSample(samples [][2]float64, index int) {
	if d.bufferIndex < len(d.buffer) {
		sample := float64(d.buffer[d.bufferIndex]) / 32768.0
		samples[index][0] = sample
		samples[index][1] = sample
		d.bufferIndex++
		d.position++
	} else {
		samples[index][0] = 0
		samples[index][1] = 0
	}
}

// fillStereoSample fills a stereo sample.
func (d *minimp3Decoder) fillStereoSample(samples [][2]float64, index int) {
	if d.bufferIndex+1 < len(d.buffer) {
		left := float64(d.buffer[d.bufferIndex]) / 32768.0
		right := float64(d.buffer[d.bufferIndex+1]) / 32768.0
		samples[index][0] = left
		samples[index][1] = right
		d.bufferIndex += 2
		d.position++
	} else {
		samples[index][0] = 0
		samples[index][1] = 0
	}
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

		samplesRead := bytesRead / (2 * d.format.NumChannels)
		if samplesRead > samplesToSkip {
			samplesRead = samplesToSkip
		}

		samplesToSkip -= samplesRead
		d.position += samplesRead
	}

	return nil
}

// seekApproximate uses byte-based estimation for faster seeking to later positions.
func (d *minimp3Decoder) seekApproximate(targetPos int) error {
	// Use simple byte-based estimation
	targetRatio := float64(targetPos) / float64(d.TotalSamples)
	estimatedBytePos := int(targetRatio * float64(len(d.data)))

	// Clamp to safe range
	if estimatedBytePos >= len(d.data)-1024 {
		estimatedBytePos = len(d.data) - 1024
	}

	if estimatedBytePos < 0 {
		estimatedBytePos = 0
	}

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
