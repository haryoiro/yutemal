package player

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/speaker"
	"github.com/faiface/beep/wav"
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
	duration           time.Duration
	ctx                context.Context
	cancel             context.CancelFunc
	speakerInitialized bool
	currentSampleRate  beep.SampleRate
	lastSeekTime       time.Time
	seekCooldown       time.Duration
	iseeking           bool // Prevent concurrent seeks
}

// New creates a new audio player
func New() (*Player, error) {
	ctx, cancel := context.WithCancel(context.Background())

	player := &Player{
		ctx:          ctx,
		cancel:       cancel,
		seekCooldown: 500 * time.Millisecond, // Reduced cooldown for more responsive seeking
		iseeking:     false,
	}

	logger.Info("Audio player created (speaker will be initialized on first file load)")

	return player, nil
}

// LoadFile loads an audio file for playback
func (p *Player) LoadFile(filepath string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Close previous file if any
	if p.streamer != nil {
		p.streamer.Close()
		p.streamer = nil
	}

	// Clear speaker to ensure clean state
	if p.speakerInitialized {
		speaker.Clear()
	}

	// Reset all player state for new file
	p.duration = 0
	p.iseeking = false
	p.ctrl = nil
	p.volume = nil

	// Open file
	file, err := os.Open(filepath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}

	// Decode audio file based on extension
	var streamer beep.StreamSeekCloser
	var format beep.Format

	if strings.HasSuffix(strings.ToLower(filepath), ".mp3") {
		// Decode MP3 using minimp3 decoder
		logger.Debug("Loading MP3 file: %s", filepath)
		streamer, format, err = DecodeMiniMP3(file)
		if err != nil {
			file.Close()
			return fmt.Errorf("failed to decode MP3: %w", err)
		}

		// Set duration update callback for dynamic duration correction
		if minimp3Dec, ok := streamer.(*minimp3Decoder); ok {
			minimp3Dec.durationUpdateCallback = p.UpdateActualDuration
		}

		logger.Debug("MP3 decode successful")
	} else if strings.HasSuffix(strings.ToLower(filepath), ".wav") {
		// Decode WAV using beep's wav decoder
		streamer, format, err = wav.Decode(file)
		if err != nil {
			file.Close()
			return fmt.Errorf("failed to decode WAV: %w", err)
		}
	} else {
		file.Close()
		return fmt.Errorf("unsupported file format: %s", filepath)
	}

	// Store original streamer reference before wrapping
	p.streamer = streamer
	
	// Wrap streamer in buffered streamer for smoother playback
	bufferedStreamer := NewBufferedStreamer(streamer, format, 1.0) // 1 second buffer
	
	// Create volume control
	volume := &effects.Volume{
		Streamer: bufferedStreamer,
		Base:     2,
		Volume:   0, // Start at normal volume (0 dB)
		Silent:   false,
	}

	// Create playback control
	ctrl := &beep.Ctrl{
		Streamer: volume,
		Paused:   true,
	}

	// p.streamer already set above
	p.ctrl = ctrl
	p.volume = volume
	p.format = format
	p.currentFile = filepath
	p.isPlaying = false

	// Calculate duration - prefer ffprobe for accuracy
	actualDuration := p.getActualDuration(filepath)
	if actualDuration > 0 {
		p.duration = actualDuration
		logger.Info("Using ffprobe duration: %v", actualDuration)

		// Update decoder with accurate sample count
		if minimp3Dec, ok := streamer.(*minimp3Decoder); ok {
			actualSamples := p.format.SampleRate.N(actualDuration)
			minimp3Dec.TotalSamples = actualSamples
			logger.Info("Updated minimp3 decoder with ffprobe sample count: %d", actualSamples)
		}
	} else {
		// Fallback to decoder's estimate
		length := p.streamer.Len()
		p.duration = p.format.SampleRate.D(length)
		logger.Debug("Using decoder estimated duration: %v (%d samples)", p.duration, length)
	}

	// Initialize or reinitialize speaker if needed
	if !p.speakerInitialized || p.currentSampleRate != format.SampleRate {
		if p.speakerInitialized {
			speaker.Close()
			time.Sleep(100 * time.Millisecond) // Give it time to close
		}

		err = speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/4))
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

	// Log file information
	fileInfo, err := os.Stat(filepath)
	if err == nil {
		fileSizeMB := float64(fileInfo.Size()) / (1024 * 1024)
		fileExt := strings.ToLower(filepath[strings.LastIndex(filepath, ".")+1:])
		logger.Info("Loaded %s file: %s (%.2f MB), duration: %v, sample rate: %d Hz, channels: %d",
			fileExt, filepath, fileSizeMB, p.duration, format.SampleRate, format.NumChannels)
	}

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

	// Clear speaker completely
	speaker.Clear()

	// Reset streamer position safely
	if p.streamer != nil {
		if err := p.streamer.Seek(0); err != nil {
			logger.Error("Error seeking to start: %v", err)
		}
	}

	// Reset playback state
	p.isPlaying = false
	p.iseeking = false

	return nil
}

// SetVolume sets the volume (0.0 to 1.0)
func (p *Player) SetVolume(volume float64) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.volume == nil || !p.speakerInitialized {
		return fmt.Errorf("no file loaded")
	}

	// Convert to dB scale
	var dbVolume float64
	if volume <= 0 {
		p.volume.Silent = true
		return nil
	} else {
		p.volume.Silent = false
		// Logarithmic scale: 20 * log10(volume)
		if volume < 0.01 {
			dbVolume = -4.0 // Very quiet but not silent
		} else {
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
	return (p.volume.Volume / 20) + 1
}

// Seek seeks to a specific position - simplified version
func (p *Player) Seek(pos time.Duration) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.streamer == nil {
		return fmt.Errorf("no file loaded")
	}

	// Prevent concurrent seeks
	if p.iseeking {
		return fmt.Errorf("seek already in progress")
	}
	p.iseeking = true
	defer func() { p.iseeking = false }()

	// Clamp position to valid range
	if pos < 0 {
		pos = 0
	}
	if pos > p.duration {
		pos = p.duration
	}

	samplePos := p.format.SampleRate.N(pos)

	// Pause during seek for cleaner operation
	wasPlaying := p.isPlaying
	if wasPlaying && p.ctrl != nil {
		speaker.Lock()
		p.ctrl.Paused = true
		speaker.Unlock()
		speaker.Clear()
	}

	// Perform the seek
	if err := p.streamer.Seek(samplePos); err != nil {
		logger.Debug("Seek failed at sample %d, resetting to start: %v", samplePos, err)
		// Reset on failure
		if err := p.streamer.Seek(0); err != nil {
			return fmt.Errorf("failed to reset position: %w", err)
		}
		pos = 0
	}

	// Resume playback if it was playing
	if wasPlaying && p.ctrl != nil {
		speaker.Play(p.ctrl)
		speaker.Lock()
		p.ctrl.Paused = false
		speaker.Unlock()
	}

	// Record seek operation
	p.lastSeekTime = time.Now()

	logger.Debug("Seek completed to position: %v", pos)
	return nil
}

// SeekForward seeks forward by the specified duration
func (p *Player) SeekForward(duration time.Duration) error {
	currentPos := p.GetPosition()
	newPos := currentPos + duration

	if newPos > p.duration {
		newPos = p.duration
	}

	return p.Seek(newPos)
}

// SeekBackward seeks backward by the specified duration
func (p *Player) SeekBackward(duration time.Duration) error {
	currentPos := p.GetPosition()
	newPos := currentPos - duration
	if newPos < 0 {
		newPos = 0
	}

	return p.Seek(newPos)
}

// GetPosition returns the current playback position using stream position only
func (p *Player) GetPosition() time.Duration {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.streamer == nil {
		return 0
	}

	// Use stream position as single source of truth
	streamPos := p.streamer.Position()
	return p.format.SampleRate.D(streamPos)
}

// HasEnded checks if playback has reached the end
func (p *Player) HasEnded() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.streamer == nil || !p.isPlaying {
		return false
	}

	// Simple end detection using stream position
	currentPos := p.streamer.Position()
	totalLen := p.streamer.Len()

	// Consider ended if we're very close to the end (within 100 samples)
	threshold := 100
	hasEnded := currentPos >= totalLen-threshold

	return hasEnded
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

// GetRawPosition returns the current sample position
func (p *Player) GetRawPosition() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.streamer == nil {
		return 0
	}
	return p.streamer.Position()
}

// GetRawLength returns the total samples
func (p *Player) GetRawLength() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.streamer == nil {
		return 0
	}
	return p.streamer.Len()
}

// GetSampleRate returns the current sample rate
func (p *Player) GetSampleRate() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return int(p.format.SampleRate)
}

// UpdateActualDuration updates the duration based on actual EOF detection
func (p *Player) UpdateActualDuration(actualSamples int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.format.SampleRate == 0 || actualSamples <= 0 {
		return
	}

	actualDuration := p.format.SampleRate.D(actualSamples)
	if actualDuration != p.duration {
		oldDuration := p.duration
		p.duration = actualDuration

		logger.Info("Duration corrected from %v to %v (difference: %v)",
			oldDuration, actualDuration, oldDuration-actualDuration)
	}
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

// getActualDuration uses ffprobe to get the exact duration of the file
func (p *Player) getActualDuration(filepath string) time.Duration {
	// Use ffprobe to get accurate duration
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		filepath,
	)

	output, err := cmd.Output()
	if err != nil {
		return 0
	}

	// Parse duration
	durationStr := strings.TrimSpace(string(output))
	if durationStr == "" || durationStr == "N/A" {
		return 0
	}

	var seconds float64
	fmt.Sscanf(durationStr, "%f", &seconds)

	if seconds > 0 {
		return time.Duration(seconds * float64(time.Second))
	}

	return 0
}
