package player

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/faiface/beep"
	"github.com/tosone/minimp3"
)

// minimp3Decoder wraps minimp3 decoder to implement beep.StreamSeekCloser
type minimp3Decoder struct {
	decoder      *minimp3.Decoder
	data         []byte
	format       beep.Format
	position     int // Current position in samples
	totalSamples int
	buffer       []int16
	bufferIndex  int
}

// DecodeMiniMP3 decodes an MP3 file using minimp3
func DecodeMiniMP3(file *os.File) (beep.StreamSeekCloser, beep.Format, error) {
	// Get file info for size
	stat, err := file.Stat()
	if err != nil {
		return nil, beep.Format{}, fmt.Errorf("failed to stat file: %w", err)
	}
	fileSize := stat.Size()

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

	// Estimate duration based on file size and bitrate
	// YouTube Music typically uses 128-256 kbps
	// Average bitrate calculation: file_size * 8 / duration = bitrate
	// So: duration = file_size * 8 / bitrate

	// Use a conservative bitrate estimate
	estimatedBitrate := 160 // kbps, conservative estimate for YouTube Music
	estimatedDuration := float64(fileSize) * 8.0 / float64(estimatedBitrate) / 1000.0 // seconds
	totalSamples := int(estimatedDuration * float64(sampleRate))


	return &minimp3Decoder{
		decoder:      dec,
		data:         data,
		format:       format,
		position:     0,
		totalSamples: totalSamples,
		buffer:       make([]int16, 0),
		bufferIndex:  0,
	}, format, nil
}

// Stream streams audio samples
func (d *minimp3Decoder) Stream(samples [][2]float64) (n int, ok bool) {
	for i := range samples {
		// Need more data?
		if d.bufferIndex >= len(d.buffer) {
			// Read more data
			const readSize = 4096
			buf := make([]byte, readSize)
			n, err := d.decoder.Read(buf)
			if err == io.EOF || n == 0 {
				return i, false
			}

			// Convert bytes to int16 samples
			d.buffer = make([]int16, n/2)
			for j := 0; j < n/2; j++ {
				d.buffer[j] = int16(buf[j*2]) | (int16(buf[j*2+1]) << 8)
			}
			d.bufferIndex = 0
		}

		// Convert to float64 samples
		if d.format.NumChannels == 1 {
			// Mono
			if d.bufferIndex < len(d.buffer) {
				sample := float64(d.buffer[d.bufferIndex]) / 32768.0
				samples[i][0] = sample
				samples[i][1] = sample
				d.bufferIndex++
				d.position++
			}
		} else {
			// Stereo
			if d.bufferIndex+1 < len(d.buffer) {
				left := float64(d.buffer[d.bufferIndex]) / 32768.0
				right := float64(d.buffer[d.bufferIndex+1]) / 32768.0
				samples[i][0] = left
				samples[i][1] = right
				d.bufferIndex += 2
				d.position += 1
			}
		}
	}
	return len(samples), true
}

// Err returns any error
func (d *minimp3Decoder) Err() error {
	return nil
}

// Len returns the total number of samples
func (d *minimp3Decoder) Len() int {
	return d.totalSamples
}

// Position returns current position in samples
func (d *minimp3Decoder) Position() int {
	return d.position
}

// Seek seeks to a position in samples
func (d *minimp3Decoder) Seek(p int) error {
	// Since minimp3 doesn't support direct seeking,
	// we'll use a more efficient approach by calculating byte offset

	// Reset decoder to beginning
	dec, err := minimp3.NewDecoder(bytes.NewReader(d.data))
	if err != nil {
		return fmt.Errorf("failed to recreate decoder for seek: %w", err)
	}

	d.decoder = dec
	d.position = 0
	d.buffer = make([]int16, 0)
	d.bufferIndex = 0

	if p <= 0 {
		return nil
	}

	// Try to estimate byte position based on average bitrate
	// This is approximate but avoids decoding all audio
	bytesPerSecond := 128000 / 8 // Assume 128kbps
	secondsToSkip := float64(p) / float64(d.format.SampleRate)
	estimatedBytes := int(secondsToSkip * float64(bytesPerSecond))

	// Make sure we don't exceed file size
	if estimatedBytes >= len(d.data) {
		estimatedBytes = len(d.data) - 1024 // Leave some buffer
	}

	// Create decoder starting from estimated position
	// This won't be exact but will be much faster
	if estimatedBytes > 0 && estimatedBytes < len(d.data) {
		// Try to find MP3 frame sync
		for i := estimatedBytes; i < len(d.data)-1; i++ {
			// MP3 frame sync is 0xFF 0xFx
			if d.data[i] == 0xFF && (d.data[i+1]&0xF0) == 0xF0 {
				// Found potential frame start
				newDec, err := minimp3.NewDecoder(bytes.NewReader(d.data[i:]))
				if err == nil {
					d.decoder = newDec
					d.position = p // Approximate position
					return nil
				}
			}
		}
	}

	// Fallback: decode from start but don't output audio
	// This is still faster than the previous approach
	samplesToSkip := p
	skipBuffer := make([]byte, 4096)

	for samplesToSkip > 0 {
		n, err := d.decoder.Read(skipBuffer)
		if err == io.EOF || n == 0 {
			break
		}

		// Calculate samples read
		samplesRead := n / (2 * d.format.NumChannels) // 16-bit samples
		if samplesRead > samplesToSkip {
			samplesRead = samplesToSkip
		}
		samplesToSkip -= samplesRead
		d.position += samplesRead
	}

	return nil
}

// Close closes the decoder
func (d *minimp3Decoder) Close() error {
	// minimp3 decoder doesn't need explicit close
	return nil
}
