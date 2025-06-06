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
	tempBuffer := make([][2]float64, 1024)

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
			// Buffer is nearly full, wait
			// Important: cond.Wait() releases the mutex and reacquires it on wake
			bs.cond.Wait()
			// After Wait returns, we still hold the lock
			if bs.closed {
				bs.mu.Unlock()
				return
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
	}
}

// Stream implements beep.Streamer
func (bs *BufferedStreamer) Stream(samples [][2]float64) (n int, ok bool) {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	// Wait for minimum buffer fill on first read
	if bs.readPos == 0 && bs.filled < bs.bufferSize/4 && !bs.closed {
		logger.Debug("Waiting for initial buffer fill: %d/%d samples", bs.filled, bs.bufferSize/4)
		for bs.filled < bs.bufferSize/4 && !bs.closed {
			bs.cond.Wait()
		}
	}

	// Check for underrun
	if bs.filled == 0 {
		if !bs.closed {
			bs.underruns++
			if bs.underruns%10 == 0 {
				logger.Warn("Audio buffer underrun detected: %d occurrences (max fill: %d/%d)",
					bs.underruns, bs.maxFilled, bs.bufferSize)
			}
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
