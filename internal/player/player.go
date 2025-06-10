package player

import (
	"context"
	"fmt"
	"math"
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
	iseeking           bool
	savedVolume        float64
	savedVolumeSet     bool
}

// New creates a new audio player
func New() (*Player, error) {
	ctx, cancel := context.WithCancel(context.Background())

	player := &Player{
		ctx:            ctx,
		cancel:         cancel,
		seekCooldown:   500 * time.Millisecond,
		iseeking:       false,
		savedVolume:    0.7,
		savedVolumeSet: false,
	}

	logger.Debug("Audio player created (speaker will be initialized on first file load)")

	return player, nil
}

// LoadFile loads an audio file for playback
func (p *Player) LoadFile(filepath string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.streamer != nil {
		p.streamer.Close()
		p.streamer = nil
	}

	if p.speakerInitialized {
		speaker.Clear()
	}

	p.duration = 0
	p.iseeking = false
	p.ctrl = nil
	p.volume = nil

	file, err := os.Open(filepath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}

	var streamer beep.StreamSeekCloser
	var format beep.Format

	if strings.HasSuffix(strings.ToLower(filepath), ".mp3") {
		streamer, format, err = DecodeMiniMP3(file)
		if err != nil {
			file.Close()
			return fmt.Errorf("failed to decode MP3: %w", err)
		}

		if minimp3Dec, ok := streamer.(*minimp3Decoder); ok {
			minimp3Dec.durationUpdateCallback = p.UpdateActualDuration
		}
	} else if strings.HasSuffix(strings.ToLower(filepath), ".wav") {
		streamer, format, err = wav.Decode(file)
		if err != nil {
			file.Close()
			return fmt.Errorf("failed to decode WAV: %w", err)
		}
	} else {
		file.Close()
		return fmt.Errorf("unsupported file format: %s", filepath)
	}

	p.streamer = streamer

	bufferedStreamer := NewBufferedStreamer(streamer, format, 4.0)

	var volumeToApply float64 = 0.7
	if p.savedVolumeSet {
		volumeToApply = p.savedVolume
	} else if p.volume != nil {
		volumeToApply = p.GetVolume()
	}

	var dbVolume float64
	var isSilent bool = false
	if volumeToApply <= 0 {
		isSilent = true
		dbVolume = -60.0
	} else if volumeToApply < 0.001 {
		dbVolume = -60.0
	} else {
		adjustedVolume := volumeToApply * volumeToApply
		dbVolume = 20.0 * math.Log10(adjustedVolume)
		if dbVolume < -60.0 {
			dbVolume = -60.0
		}
	}

	volume := &effects.Volume{
		Streamer: bufferedStreamer,
		Base:     2,
		Volume:   dbVolume,
		Silent:   isSilent,
	}

	ctrl := &beep.Ctrl{
		Streamer: volume,
		Paused:   true,
	}

	p.ctrl = ctrl
	p.volume = volume
	p.format = format
	p.currentFile = filepath
	p.isPlaying = false

	actualDuration := p.getActualDuration(filepath)
	if actualDuration > 0 {
		p.duration = actualDuration

		if minimp3Dec, ok := streamer.(*minimp3Decoder); ok {
			actualSamples := p.format.SampleRate.N(actualDuration)
			minimp3Dec.TotalSamples = actualSamples
		}
	} else {
		length := p.streamer.Len()
		p.duration = p.format.SampleRate.D(length)
	}

	if !p.speakerInitialized || p.currentSampleRate != format.SampleRate {
		if p.speakerInitialized {
			speaker.Close()
			time.Sleep(100 * time.Millisecond)
		}

		err = speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/2))
		if err != nil {
			return fmt.Errorf("failed to initialize speaker for sample rate %d: %w", format.SampleRate, err)
		}

		p.speakerInitialized = true
		p.currentSampleRate = format.SampleRate
	}

	speaker.Play(ctrl)

	fileInfo, err := os.Stat(filepath)
	if err == nil {
		fileSizeMB := float64(fileInfo.Size()) / (1024 * 1024)
		fileExt := strings.ToLower(filepath[strings.LastIndex(filepath, ".")+1:])
		logger.Debug("Loaded %s file: %s (%.2f MB), duration: %v, sample rate: %d Hz, channels: %d",
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
	p.iseeking = false

	return nil
}

// SetVolume sets the volume (0.0 to 1.0)
func (p *Player) SetVolume(volume float64) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.savedVolume = volume
	p.savedVolumeSet = true

	if p.volume == nil || !p.speakerInitialized {
		return nil
	}

	var dbVolume float64
	if volume <= 0 {
		p.volume.Silent = true
		return nil
	} else {
		p.volume.Silent = false
		if volume < 0.001 {
			dbVolume = -60.0
		} else {
			adjustedVolume := volume * volume
			dbVolume = 20.0 * math.Log10(adjustedVolume)
			if dbVolume < -60.0 {
				dbVolume = -60.0
			}
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
		if p.savedVolumeSet {
			return p.savedVolume
		}
		return 0.7
	}

	if p.volume.Silent {
		return 0.0
	}

	return math.Pow(10, p.volume.Volume/40.0)
}

func (p *Player) Seek(pos time.Duration) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.streamer == nil {
		return fmt.Errorf("no file loaded")
	}

	if p.iseeking {
		return fmt.Errorf("seek already in progress")
	}
	p.iseeking = true
	defer func() { p.iseeking = false }()

	if pos < 0 {
		pos = 0
	}
	if pos > p.duration {
		pos = p.duration
	}

	samplePos := p.format.SampleRate.N(pos)

	wasPlaying := p.isPlaying
	if wasPlaying && p.ctrl != nil {
		speaker.Lock()
		p.ctrl.Paused = true
		speaker.Unlock()
		speaker.Clear()
	}

	if err := p.streamer.Seek(samplePos); err != nil {
		if err := p.streamer.Seek(0); err != nil {
			return fmt.Errorf("failed to reset position: %w", err)
		}
		pos = 0
	}

	if wasPlaying && p.ctrl != nil {
		speaker.Play(p.ctrl)
		speaker.Lock()
		p.ctrl.Paused = false
		speaker.Unlock()
	}

	p.lastSeekTime = time.Now()

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

func (p *Player) GetPosition() time.Duration {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.streamer == nil {
		return 0
	}

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

	currentPos := p.streamer.Position()
	totalLen := p.streamer.Len()

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

		logger.Debug("Duration corrected from %v to %v (difference: %v)",
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

func (p *Player) getActualDuration(filepath string) time.Duration {
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
