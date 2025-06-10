package player

import (
	"sync"
	"time"

	"github.com/faiface/beep"
	"github.com/haryoiro/yutemal/internal/logger"
)

// BufferedStreamer provides a ring buffer for smooth audio playback
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

	// Stats
	underruns int
	maxFilled int
}

// NewBufferedStreamer creates a new buffered streamer
func NewBufferedStreamer(source beep.Streamer, format beep.Format, bufferSeconds float64) *BufferedStreamer {
	// Calculate buffer size based on sample rate and duration
	bufferSize := int(float64(format.SampleRate) * bufferSeconds)

	bs := &BufferedStreamer{
		source:     source,
		buffer:     make([][2]float64, bufferSize),
		bufferSize: bufferSize,
		format:     format,
	}
	bs.cond = sync.NewCond(&bs.mu)

	// Start background filling
	go bs.fillLoop()

	logger.Debug("Created buffered streamer with %.1f seconds buffer (%d samples)", bufferSeconds, bufferSize)

	return bs
}

// fillLoop continuously fills the buffer in the background
func (bs *BufferedStreamer) fillLoop() {
	// Larger temp buffer for more efficient filling
	tempBuffer := make([][2]float64, 4096)

	defer func() {
		if r := recover(); r != nil {
			logger.Error("Panic in BufferedStreamer fillLoop: %v", r)
		}
	}()

	for {
		bs.mu.Lock()
		if bs.closed {
			bs.mu.Unlock()
			return
		}

		// Calculate available space
		available := bs.bufferSize - bs.filled
		if available < len(tempBuffer) {
			// Buffer is nearly full, wait for some space
			// We want to keep the buffer as full as possible, so only wait if we can't fit a chunk
			if available == 0 {
				// Buffer is completely full, wait for some consumption
				bs.cond.Wait()
				// After Wait returns, we still hold the lock
				if bs.closed {
					bs.mu.Unlock()
					return
				}
				// Recalculate available space after waiting
				available = bs.bufferSize - bs.filled
			}
			// If we still have some space, read only what we can fit
			if available > 0 && available < len(tempBuffer) {
				tempBuffer = tempBuffer[:available]
			}
		}
		bs.mu.Unlock()

		// Read from source (outside of lock to prevent deadlock)
		n, ok := bs.source.Stream(tempBuffer)
		if n == 0 && !ok {
			// Source exhausted
			bs.mu.Lock()
			bs.closed = true
			bs.cond.Broadcast()
			bs.mu.Unlock()
			return
		}

		// Write to ring buffer
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

		// Restore tempBuffer size if it was reduced
		if cap(tempBuffer) > len(tempBuffer) {
			tempBuffer = tempBuffer[:cap(tempBuffer)]
		}

		// Small sleep to prevent busy looping when buffer is full
		// But don't sleep too long to avoid buffer underrun
		if available < len(tempBuffer)/4 {
			time.Sleep(10 * time.Millisecond)
		} else if available < len(tempBuffer)/2 {
			time.Sleep(5 * time.Millisecond)
		}
	}
}

// Stream implements beep.Streamer
func (bs *BufferedStreamer) Stream(samples [][2]float64) (n int, ok bool) {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	// Wait for minimum buffer fill on first read - use 50% for more stability
	if bs.readPos == 0 && bs.filled < bs.bufferSize/2 && !bs.closed {
		logger.Debug("Waiting for initial buffer fill: %d/%d samples", bs.filled, bs.bufferSize/2)
		for bs.filled < bs.bufferSize/2 && !bs.closed {
			bs.cond.Wait()
		}
	}

	// Check for underrun
	if bs.filled == 0 {
		if !bs.closed {
			bs.underruns++
			// Log every underrun for debugging
			logger.Warn("Audio buffer underrun #%d detected at position %d (max fill: %d/%d = %.1f%%)",
				bs.underruns, bs.readPos, bs.maxFilled, bs.bufferSize,
				float64(bs.maxFilled)/float64(bs.bufferSize)*100)
			// Try to give the fill loop more time
			time.Sleep(50 * time.Millisecond)
		} else {
			// Source is closed and buffer is empty
			return 0, false
		}
	}

	// Read from buffer
	for i := range samples {
		if bs.filled == 0 {
			// No more data available right now
			if bs.closed {
				ok = i > 0
			} else {
				ok = true
			}
			n = i
			bs.cond.Broadcast()
			return
		}

		samples[i] = bs.buffer[bs.readPos]
		bs.readPos = (bs.readPos + 1) % bs.bufferSize
		bs.filled--
	}

	bs.cond.Broadcast()
	return len(samples), true
}

// Err implements beep.Streamer
func (bs *BufferedStreamer) Err() error {
	if source, ok := bs.source.(beep.StreamSeeker); ok {
		return source.Err()
	}
	return nil
}

// Close closes the buffered streamer
func (bs *BufferedStreamer) Close() error {
	bs.mu.Lock()
	if bs.closed {
		bs.mu.Unlock()
		return nil
	}
	bs.closed = true
	bs.cond.Broadcast()
	bs.mu.Unlock()

	// Wait a bit for fillLoop to exit cleanly
	time.Sleep(10 * time.Millisecond)

	// Log final stats
	if bs.underruns > 0 {
		logger.Info("BufferedStreamer stats: %d underruns, max buffer fill: %d/%d (%.1f%%)",
			bs.underruns, bs.maxFilled, bs.bufferSize,
			float64(bs.maxFilled)/float64(bs.bufferSize)*100)
	}

	// Don't close the source here - let the player handle that
	return nil
}
