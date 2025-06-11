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

	// Dynamic buffer management
	minBufferSize        int
	maxBufferSize        int
	targetBufferSize     int
	resizeInProgress     bool
	consecutiveUnderruns int
	lastUnderrunTime     time.Time
}

// NewBufferedStreamer creates a new buffered streamer.
func NewBufferedStreamer(source beep.Streamer, format beep.Format, bufferSeconds float64) *BufferedStreamer {
	initialBufferSize := int(float64(format.SampleRate) * bufferSeconds)
	minBufferSize := int(float64(format.SampleRate) * 2.0) // 2 seconds minimum
	maxBufferSize := int(float64(format.SampleRate) * 8.0) // 8 seconds maximum

	bs := &BufferedStreamer{
		source:           source,
		buffer:           make([][2]float64, maxBufferSize), // Allocate max size upfront
		bufferSize:       initialBufferSize,
		targetBufferSize: initialBufferSize,
		minBufferSize:    minBufferSize,
		maxBufferSize:    maxBufferSize,
		format:           format,
		position:         0,
		lastUnderrunTime: time.Now(), // Initialize to prevent immediate reduction
	}
	bs.cond = sync.NewCond(&bs.mu)

	logger.Debug("Created buffered streamer with %.1f seconds buffer (%d samples), min: %.1f sec, max: %.1f sec",
		bufferSeconds, initialBufferSize,
		float64(minBufferSize)/float64(format.SampleRate),
		float64(maxBufferSize)/float64(format.SampleRate))

	go bs.fillLoop()

	return bs
}

// fillLoop continuously fills the buffer in the background.
func (bs *BufferedStreamer) fillLoop() {
	tempBuffer := make([][2]float64, 8192*2)
	healthCheckTicker := time.NewTicker(5 * time.Second)
	defer healthCheckTicker.Stop()

	defer func() {
		if r := recover(); r != nil {
			logger.Error("Panic in BufferedStreamer fillLoop: %v", r)
		}
	}()

	for {
		select {
		case <-healthCheckTicker.C:
			bs.checkBufferHealth()
		default:
			if !bs.processFillIteration(tempBuffer) {
				return
			}
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
			bs.handleUnderrun()
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

	// Log performance statistics
	logger.Info("BufferedStreamer performance stats:")
	logger.Info("  - Underruns: %d (consecutive max: %d)", bs.underruns, bs.consecutiveUnderruns)
	logger.Info("  - Max buffer fill: %d/%d (%.1f%%)", 
		bs.maxFilled, bs.bufferSize,
		float64(bs.maxFilled)/float64(bs.bufferSize)*100)
	logger.Info("  - Final buffer size: %.1f seconds (%d samples)",
		float64(bs.bufferSize)/float64(bs.format.SampleRate), bs.bufferSize)
	logger.Info("  - Target buffer size: %.1f seconds",
		float64(bs.targetBufferSize)/float64(bs.format.SampleRate))

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

// handleUnderrun handles buffer underrun with dynamic resizing and progressive retry
func (bs *BufferedStreamer) handleUnderrun() {
	bs.underruns++
	now := time.Now()

	// Check if this is a consecutive underrun
	if now.Sub(bs.lastUnderrunTime) < 2*time.Second {
		bs.consecutiveUnderruns++
	} else {
		bs.consecutiveUnderruns = 1
	}
	bs.lastUnderrunTime = now

	logger.Warn("Audio buffer underrun #%d detected at position %d (consecutive: %d, max fill: %d/%d = %.1f%%)",
		bs.underruns, bs.position, bs.consecutiveUnderruns,
		bs.maxFilled, bs.bufferSize,
		float64(bs.maxFilled)/float64(bs.bufferSize)*100)

	// Dynamic buffer resizing based on consecutive underruns
	if bs.consecutiveUnderruns >= 2 && bs.targetBufferSize < bs.maxBufferSize {
		newSize := bs.targetBufferSize + int(float64(bs.format.SampleRate)*0.5) // Add 0.5 seconds
		if newSize > bs.maxBufferSize {
			newSize = bs.maxBufferSize
		}

		oldSizeSeconds := float64(bs.targetBufferSize) / float64(bs.format.SampleRate)
		newSizeSeconds := float64(newSize) / float64(bs.format.SampleRate)

		logger.Info("Increasing buffer size from %.1f to %.1f seconds due to repeated underruns",
			oldSizeSeconds, newSizeSeconds)

		bs.targetBufferSize = newSize
		go bs.resizeBuffer()
	}

	// Progressive retry with decreasing wait times
	waitTime := bs.calculateWaitTime()
	time.Sleep(waitTime)
}

// calculateWaitTime returns progressively shorter wait times for retries
func (bs *BufferedStreamer) calculateWaitTime() time.Duration {
	switch bs.consecutiveUnderruns {
	case 1:
		return 100 * time.Millisecond
	case 2:
		return 50 * time.Millisecond
	case 3:
		return 25 * time.Millisecond
	default:
		return 10 * time.Millisecond
	}
}

// resizeBuffer gradually adjusts the active buffer size
func (bs *BufferedStreamer) resizeBuffer() {
	bs.mu.Lock()
	if bs.resizeInProgress {
		bs.mu.Unlock()
		return
	}
	bs.resizeInProgress = true
	targetSize := bs.targetBufferSize
	bs.mu.Unlock()

	// Gradually increase buffer size to avoid sudden changes
	for {
		bs.mu.Lock()
		if bs.closed || bs.bufferSize >= targetSize {
			bs.resizeInProgress = false
			bs.mu.Unlock()
			break
		}

		// Increase by 10% or 0.1 second worth of samples, whichever is larger
		increment := bs.bufferSize / 10
		minIncrement := int(float64(bs.format.SampleRate) * 0.1)
		if increment < minIncrement {
			increment = minIncrement
		}

		newSize := bs.bufferSize + increment
		if newSize > targetSize {
			newSize = targetSize
		}

		bs.bufferSize = newSize
		logger.Debug("Buffer size adjusted to %.2f seconds (%d samples)",
			float64(newSize)/float64(bs.format.SampleRate), newSize)

		bs.mu.Unlock()

		// Wait a bit before next adjustment
		time.Sleep(100 * time.Millisecond)
	}
}

// checkBufferHealth monitors buffer health and adjusts size downward if stable
func (bs *BufferedStreamer) checkBufferHealth() {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	// Only consider reducing buffer size if:
	// 1. No underruns at all OR haven't had an underrun for a long time
	// 2. Buffer is consistently full
	// 3. We're above minimum buffer size
	stableTime := time.Since(bs.lastUnderrunTime)
	
	if (bs.underruns == 0 && bs.position > int(bs.format.SampleRate)*30) || // 30 seconds of playback
	   (bs.underruns > 0 && stableTime > 60*time.Second) { // 1 minute since last underrun
		
		// Only reduce if buffer is consistently very full
		fillRatio := float64(bs.filled) / float64(bs.bufferSize)
		if bs.targetBufferSize > bs.minBufferSize && fillRatio > 0.9 {
			// Reduce target by 0.25 seconds
			newTarget := bs.targetBufferSize - int(float64(bs.format.SampleRate)*0.25)
			if newTarget < bs.minBufferSize {
				newTarget = bs.minBufferSize
			}

			if newTarget < bs.targetBufferSize {
				bs.targetBufferSize = newTarget
				logger.Debug("Reducing target buffer size to %.2f seconds (fill ratio: %.1f%%, stable for: %v)",
					float64(newTarget)/float64(bs.format.SampleRate),
					fillRatio*100, stableTime)
			}
		}

		// Reset consecutive underrun counter only after significant stable time
		if stableTime > 30*time.Second {
			bs.consecutiveUnderruns = 0
		}
	}
	
	// Log current buffer status periodically
	if bs.position%(int(bs.format.SampleRate)*10) == 0 && bs.position > 0 { // Every 10 seconds
		fillRatio := float64(bs.filled) / float64(bs.bufferSize)
		logger.Debug("Buffer health: size=%.1fs, filled=%.1f%%, underruns=%d",
			float64(bs.bufferSize)/float64(bs.format.SampleRate),
			fillRatio*100, bs.underruns)
	}
}
