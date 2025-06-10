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

	underruns int
	maxFilled int
}

// NewBufferedStreamer creates a new buffered streamer
func NewBufferedStreamer(source beep.Streamer, format beep.Format, bufferSeconds float64) *BufferedStreamer {
	bufferSize := int(float64(format.SampleRate) * bufferSeconds)

	bs := &BufferedStreamer{
		source:     source,
		buffer:     make([][2]float64, bufferSize),
		bufferSize: bufferSize,
		format:     format,
	}
	bs.cond = sync.NewCond(&bs.mu)

	logger.Debug("Created buffered streamer with %.1f seconds buffer (%d samples)", bufferSeconds, bufferSize)

	go bs.fillLoop()

	return bs
}

// fillLoop continuously fills the buffer in the background
func (bs *BufferedStreamer) fillLoop() {
	tempBuffer := make([][2]float64, 8192)

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

		available := bs.bufferSize - bs.filled
		if available < len(tempBuffer) {
			if available == 0 {
				bs.cond.Wait()
				if bs.closed {
					bs.mu.Unlock()
					return
				}
				available = bs.bufferSize - bs.filled
			}
			if available > 0 && available < len(tempBuffer) {
				tempBuffer = tempBuffer[:available]
			}
		}
		bs.mu.Unlock()

		n, ok := bs.source.Stream(tempBuffer)
		if n == 0 && !ok {
			bs.mu.Lock()
			bs.closed = true
			bs.cond.Broadcast()
			bs.mu.Unlock()
			logger.Debug("BufferedStreamer: source exhausted, filled: %d/%d", bs.filled, bs.bufferSize)
			return
		}

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
}

// Stream implements beep.Streamer
func (bs *BufferedStreamer) Stream(samples [][2]float64) (n int, ok bool) {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	if bs.readPos == 0 && bs.filled < bs.bufferSize*3/4 && !bs.closed {
		logger.Debug("Waiting for initial buffer fill: %d/%d samples", bs.filled, bs.bufferSize*3/4)
		for bs.filled < bs.bufferSize*3/4 && !bs.closed {
			bs.cond.Wait()
		}
	}

	if bs.filled == 0 {
		if !bs.closed {
			bs.underruns++
			logger.Warn("Audio buffer underrun #%d detected at position %d (max fill: %d/%d = %.1f%%)",
				bs.underruns, bs.readPos, bs.maxFilled, bs.bufferSize,
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

	time.Sleep(10 * time.Millisecond)

	if bs.underruns > 0 {
		logger.Debug("BufferedStreamer stats: %d underruns, max buffer fill: %d/%d (%.1f%%)",
			bs.underruns, bs.maxFilled, bs.bufferSize,
			float64(bs.maxFilled)/float64(bs.bufferSize)*100)
	}

	return nil
}
