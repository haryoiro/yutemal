package player

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/haryoiro/yutemal/internal/logger"
)

// Player represents the audio player
type Player struct {
	mu                 sync.RWMutex
	streamer           beep.StreamSeekCloser
	ctrl               *beep.Ctrl
	volume             *effects.Volume
	format             beep.Format
	isPlaying          bool
	currentFile        string
	position           time.Duration
	duration           time.Duration
	ctx                context.Context
	cancel             context.CancelFunc
	speakerInitialized bool
	currentSampleRate  beep.SampleRate
	lastSeekTime       time.Time
	seekCooldown       time.Duration
}

// New creates a new audio player
func New() (*Player, error) {
	ctx, cancel := context.WithCancel(context.Background())

	player := &Player{
		ctx:          ctx,
		cancel:       cancel,
		seekCooldown: 500 * time.Millisecond, // Prevent immediate end-of-song detection after seek
	}

	// Don't initialize speaker here - do it when we load the first file
	logger.Debug("Audio player created (speaker will be initialized on first file load)")

	return player, nil
}

// LoadFile loads an audio file for playback
func (p *Player) LoadFile(filepath string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Close previous file if any
	if p.streamer != nil {
		p.streamer.Close()
	}

	// Open file
	file, err := os.Open(filepath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}

	// Decode MP3 (you can extend this for other formats)
	streamer, format, err := mp3.Decode(file)
	if err != nil {
		file.Close()
		return fmt.Errorf("failed to decode MP3: %w", err)
	}

	// Create volume control
	volume := &effects.Volume{
		Streamer: streamer,
		Base:     2,
		Volume:   0, // Start at normal volume (0 dB)
		Silent:   false,
	}

	// Create playback control
	ctrl := &beep.Ctrl{
		Streamer: volume,
		Paused:   true,
	}

	p.streamer = streamer
	p.ctrl = ctrl
	p.volume = volume
	p.format = format
	p.currentFile = filepath
	p.isPlaying = false

	// Calculate duration
	length := p.streamer.Len()
	p.duration = p.format.SampleRate.D(length)

	// Initialize or reinitialize speaker if needed
	if !p.speakerInitialized || p.currentSampleRate != format.SampleRate {
		if p.speakerInitialized {
			speaker.Close()
			time.Sleep(100 * time.Millisecond) // Give it time to close
		}

		err = speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))
		if err != nil {
			return fmt.Errorf("failed to initialize speaker for sample rate %d: %w", format.SampleRate, err)
		}

		p.speakerInitialized = true
		p.currentSampleRate = format.SampleRate
		logger.Debug("Speaker initialized with sample rate: %d Hz", format.SampleRate)
	}

	// Clear any existing audio and start the new one
	speaker.Clear()
	speaker.Play(ctrl)

	logger.Debug("Loaded file: %s, duration: %v, format: %v", filepath, p.duration, format)

	return nil
}

// Play starts playback
func (p *Player) Play() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.ctrl == nil {
		return fmt.Errorf("no file loaded")
	}

	speaker.Lock()
	p.ctrl.Paused = false
	p.isPlaying = true
	speaker.Unlock()

	logger.Debug("Playback started")

	return nil
}

// Pause pauses playback
func (p *Player) Pause() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.ctrl == nil {
		return fmt.Errorf("no file loaded")
	}

	speaker.Lock()
	p.ctrl.Paused = true
	p.isPlaying = false
	speaker.Unlock()

	return nil
}

// Toggle toggles play/pause
func (p *Player) Toggle() error {
	p.mu.RLock()
	isPlaying := p.isPlaying
	p.mu.RUnlock()

	if isPlaying {
		return p.Pause()
	}
	return p.Play()
}

// Stop stops playback
func (p *Player) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.ctrl == nil || !p.speakerInitialized {
		return nil
	}

	speaker.Clear()

	if p.streamer != nil {
		if err := p.streamer.Seek(0); err != nil {
			logger.Error("Error seeking to start: %v", err)
		}
	}

	p.isPlaying = false
	p.position = 0

	return nil
}

// SetVolume sets the volume (0.0 to 1.0)
func (p *Player) SetVolume(volume float64) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.volume == nil || !p.speakerInitialized {
		return fmt.Errorf("no file loaded")
	}

	// Convert to dB scale: 0.0 -> -∞ dB, 0.5 -> -6 dB, 1.0 -> 0 dB
	var dbVolume float64
	if volume <= 0 {
		p.volume.Silent = true
		return nil
	} else {
		p.volume.Silent = false
		// Use logarithmic scale for more natural volume control
		// volume: 0.0 to 1.0
		// dB: -∞ to 0
		if volume < 0.01 {
			dbVolume = -4.0 // Very quiet but not silent
		} else {
			// Logarithmic scale: 20 * log10(volume)
			dbVolume = 20 * (volume - 1) // Simplified approximation
		}
	}

	speaker.Lock()
	p.volume.Volume = dbVolume
	speaker.Unlock()

	return nil
}

// GetVolume returns the current volume (0.0 to 1.0)
func (p *Player) GetVolume() float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.volume == nil {
		return 0.5
	}

	if p.volume.Silent {
		return 0.0
	}

	// Convert from dB back to linear scale
	// dbVolume = 20 * (volume - 1)
	// volume = (dbVolume / 20) + 1
	return (p.volume.Volume / 20) + 1
}

// Seek seeks to a specific position
func (p *Player) Seek(pos time.Duration) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.streamer == nil {
		return fmt.Errorf("no file loaded")
	}

	// Clamp position to valid range
	if pos < 0 {
		pos = 0
	}
	if pos > p.duration {
		pos = p.duration
	}

	samplePos := p.format.SampleRate.N(pos)
	if err := p.streamer.Seek(samplePos); err != nil {
		return fmt.Errorf("failed to seek: %w", err)
	}

	// Record seek operation to avoid immediate end-of-song detection
	p.lastSeekTime = time.Now()
	
	// Update position based on actual streamer position after seek
	actualSample := p.streamer.Position()
	p.position = p.format.SampleRate.D(actualSample)
	
	logger.Debug("Seeked to %v (requested: %v)", p.position, pos)
	return nil
}

// SeekForward seeks forward by the specified duration
func (p *Player) SeekForward(duration time.Duration) error {
	p.mu.RLock()
	currentPos := p.GetPositionUnsafe() // Get real-time position
	newPos := currentPos + duration
	
	// Handle boundary: if seeking beyond end, go to 95% to avoid immediate song end
	if newPos > p.duration {
		if p.duration > time.Second {
			newPos = p.duration - (p.duration / 20) // 95% of duration
		} else {
			newPos = p.duration
		}
	}
	p.mu.RUnlock()

	return p.Seek(newPos)
}

// SeekBackward seeks backward by the specified duration
func (p *Player) SeekBackward(duration time.Duration) error {
	p.mu.RLock()
	currentPos := p.GetPositionUnsafe() // Get real-time position
	newPos := currentPos - duration
	if newPos < 0 {
		newPos = 0
	}
	p.mu.RUnlock()

	return p.Seek(newPos)
}

// GetPosition returns the current playback position
func (p *Player) GetPosition() time.Duration {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.GetPositionUnsafe()
}

// GetPositionUnsafe returns the current playback position without locking
// Should only be called when already holding the lock
func (p *Player) GetPositionUnsafe() time.Duration {
	if p.streamer == nil {
		return 0
	}

	// Calculate position based on current stream position
	currentSample := p.streamer.Position()
	p.position = p.format.SampleRate.D(currentSample)
	return p.position
}

// GetDuration returns the total duration
func (p *Player) GetDuration() time.Duration {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.duration
}

// IsPlaying returns whether the player is currently playing
func (p *Player) IsPlaying() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.isPlaying && p.ctrl != nil && !p.ctrl.Paused
}

// GetCurrentFile returns the currently loaded file
func (p *Player) GetCurrentFile() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.currentFile
}

// IsRecentSeek returns true if a seek operation was performed recently
func (p *Player) IsRecentSeek() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return time.Since(p.lastSeekTime) < p.seekCooldown
}

// Close closes the player and releases resources
func (p *Player) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.cancel()

	if p.streamer != nil {
		p.streamer.Close()
	}

	if p.speakerInitialized {
		speaker.Close()
		p.speakerInitialized = false
	}

	return nil
}

// VolumeUp increases volume by 5%
func (p *Player) VolumeUp() error {
	currentVol := p.GetVolume()
	newVol := currentVol + 0.05
	if newVol > 1.0 {
		newVol = 1.0
	}
	return p.SetVolume(newVol)
}

// VolumeDown decreases volume by 5%
func (p *Player) VolumeDown() error {
	currentVol := p.GetVolume()
	newVol := currentVol - 0.05
	if newVol < 0.0 {
		newVol = 0.0
	}
	return p.SetVolume(newVol)
}
