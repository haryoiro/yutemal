package player

import (
	"sync"
	"time"

	"github.com/faiface/beep"

	"github.com/haryoiro/yutemal/internal/logger"
)

// BufferedStreamer provides a ring buffer for smooth audio playback.
type BufferedStreamer struct {
	source     beep.Streamer
	buffer     [][2]float64
	bufferSize int
	readPos    int
	writePos   int
	filled     int
	mu         sync.Mutex
	cond       *sync.Cond
	closed     bool
	format     beep.Format
	position   int // Track actual stream position

	underruns int
	maxFilled int
}

// NewBufferedStreamer creates a new buffered streamer.
func NewBufferedStreamer(source beep.Streamer, format beep.Format, bufferSeconds float64) *BufferedStreamer {
	bufferSize := int(float64(format.SampleRate) * bufferSeconds)

	bs := &BufferedStreamer{
		source:     source,
		buffer:     make([][2]float64, bufferSize),
		bufferSize: bufferSize,
		format:     format,
		position:   0,
	}
	bs.cond = sync.NewCond(&bs.mu)

	logger.Debug("Created buffered streamer with %.1f seconds buffer (%d samples)", bufferSeconds, bufferSize)

	go bs.fillLoop()

	return bs
}

// fillLoop continuously fills the buffer in the background.
func (bs *BufferedStreamer) fillLoop() {
	tempBuffer := make([][2]float64, 8192*2)

	defer func() {
		if r := recover(); r != nil {
			logger.Error("Panic in BufferedStreamer fillLoop: %v", r)
		}
	}()

	for {
		if !bs.processFillIteration(tempBuffer) {
			return
		}
	}
}

// processFillIteration handles one iteration of the fill loop.
func (bs *BufferedStreamer) processFillIteration(tempBuffer [][2]float64) bool {
	bs.mu.Lock()
	if bs.closed {
		bs.mu.Unlock()
		return false
	}

	available := bs.bufferSize - bs.filled
	workBuffer := bs.adjustTempBuffer(tempBuffer, available)
	bs.mu.Unlock()

	n, ok := bs.source.Stream(workBuffer)
	if n == 0 && !ok {
		bs.handleSourceExhausted()
		return false
	}

	bs.writeToBuffer(workBuffer, n)
	bs.handlePostWrite(workBuffer, available)

	return true
}

// adjustTempBuffer adjusts the temp buffer size based on available space.
func (bs *BufferedStreamer) adjustTempBuffer(tempBuffer [][2]float64, available int) [][2]float64 {
	if available >= len(tempBuffer) {
		return tempBuffer
	}

	if available == 0 {
		bs.cond.Wait()

		if bs.closed {
			return tempBuffer
		}

		available = bs.bufferSize - bs.filled
	}

	if available > 0 && available < len(tempBuffer) {
		return tempBuffer[:available]
	}

	return tempBuffer
}

// handleSourceExhausted handles when the source has no more data.
func (bs *BufferedStreamer) handleSourceExhausted() {
	bs.mu.Lock()
	bs.closed = true
	bs.cond.Broadcast()
	bs.mu.Unlock()
	logger.Debug("BufferedStreamer: source exhausted, filled: %d/%d", bs.filled, bs.bufferSize)
}

// writeToBuffer writes samples to the ring buffer.
func (bs *BufferedStreamer) writeToBuffer(tempBuffer [][2]float64, n int) {
	bs.mu.Lock()
	for i := 0; i < n; i++ {
		bs.buffer[bs.writePos] = tempBuffer[i]
		bs.writePos = (bs.writePos + 1) % bs.bufferSize
	}

	bs.filled += n
	if bs.filled > bs.maxFilled {
		bs.maxFilled = bs.filled
	}

	bs.cond.Broadcast()
	bs.mu.Unlock()
}

// handlePostWrite handles actions after writing to buffer.
func (bs *BufferedStreamer) handlePostWrite(tempBuffer [][2]float64, available int) {
	if cap(tempBuffer) > len(tempBuffer) {
		tempBuffer = tempBuffer[:cap(tempBuffer)]
	}

	if available < len(tempBuffer)/2 {
		time.Sleep(2 * time.Millisecond)
	}

	if bs.filled < bs.bufferSize/4 && !bs.closed {
		logger.Debug("BufferedStreamer: low buffer warning - filled: %d/%d (%.1f%%)",
			bs.filled, bs.bufferSize, float64(bs.filled)/float64(bs.bufferSize)*100)
	}
}

// Stream implements beep.Streamer.
func (bs *BufferedStreamer) Stream(samples [][2]float64) (n int, ok bool) {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	// Don't wait for initial fill if we've already started reading
	// This helps with seeking to work immediately
	if bs.position == 0 && bs.filled < bs.bufferSize*3/4 && !bs.closed && bs.underruns == 0 {
		logger.Debug("Waiting for initial buffer fill: %d/%d samples", bs.filled, bs.bufferSize*3/4)

		for bs.filled < bs.bufferSize*3/4 && !bs.closed {
			bs.cond.Wait()
		}
	}

	if bs.filled == 0 {
		if !bs.closed {
			bs.underruns++
			logger.Warn("Audio buffer underrun #%d detected at position %d (max fill: %d/%d = %.1f%%)",
				bs.underruns, bs.position, bs.maxFilled, bs.bufferSize,
				float64(bs.maxFilled)/float64(bs.bufferSize)*100)
			time.Sleep(100 * time.Millisecond)
		} else {
			return 0, false
		}
	}

	for i := range samples {
		if bs.filled == 0 {
			if bs.closed {
				ok = i > 0
			} else {
				ok = true
			}

			n = i

			bs.cond.Broadcast()

			return n, ok
		}

		samples[i] = bs.buffer[bs.readPos]
		bs.readPos = (bs.readPos + 1) % bs.bufferSize
		bs.filled--
		bs.position++
	}

	bs.cond.Broadcast()

	return len(samples), true
}

// Err implements beep.Streamer.
func (bs *BufferedStreamer) Err() error {
	if source, ok := bs.source.(beep.StreamSeeker); ok {
		return source.Err()
	}

	return nil
}

// Close closes the buffered streamer.
func (bs *BufferedStreamer) Close() error {
	bs.mu.Lock()
	if bs.closed {
		bs.mu.Unlock()
		return nil
	}

	bs.closed = true
	bs.cond.Broadcast()
	bs.mu.Unlock()

	time.Sleep(10 * time.Millisecond)

	if bs.underruns > 0 {
		logger.Debug("BufferedStreamer stats: %d underruns, max buffer fill: %d/%d (%.1f%%)",
			bs.underruns, bs.maxFilled, bs.bufferSize,
			float64(bs.maxFilled)/float64(bs.bufferSize)*100)
	}

	return nil
}

// Seek clears the buffer and seeks the underlying source.
func (bs *BufferedStreamer) Seek(position int) error {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	// Clear the buffer and reset state
	bs.readPos = 0
	bs.writePos = 0
	bs.filled = 0
	bs.position = position
	// Don't reset underruns count as it's useful for debugging

	// Seek the underlying source if it supports seeking
	if seeker, ok := bs.source.(beep.StreamSeeker); ok {
		if err := seeker.Seek(position); err != nil {
			return err
		}
	}

	// Wake up the fill loop to start buffering from the new position
	bs.cond.Broadcast()

	logger.Debug("BufferedStreamer: buffer cleared after seek to position %d", position)

	return nil
}

// Position returns the current position.
func (bs *BufferedStreamer) Position() int {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	return bs.position
}

// Len returns the total length if the source implements StreamSeeker.
func (bs *BufferedStreamer) Len() int {
	if seeker, ok := bs.source.(beep.StreamSeeker); ok {
		return seeker.Len()
	}

	return 0
}
