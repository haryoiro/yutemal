package systems

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/haryoiro/yutemal/internal/api"
	"github.com/haryoiro/yutemal/internal/database"
	"github.com/haryoiro/yutemal/internal/logger"
	"github.com/haryoiro/yutemal/internal/player"
	"github.com/haryoiro/yutemal/internal/structures"
)

// PlayerSystem manages audio playback
type PlayerSystem struct {
	mu               sync.RWMutex
	config           *structures.Config
	database         database.DB
	state            *structures.PlayerState
	actionChan       chan structures.SoundAction
	stopChan         chan struct{}
	player           *player.Player
	cacheDir         string
	downloadCallback func(track structures.Track)
	skipUpdate       int32       // Atomic flag to skip position updates during critical operations
	apiClient        interface{} // API client for fetching bitrate info (optional)
}

// NewPlayerSystem creates a new player system
func NewPlayerSystem(cfg *structures.Config, db database.DB, cacheDir string) *PlayerSystem {
	audioPlayer, err := player.New()
	if err != nil {
		logger.Error("Failed to create audio player: %v", err)
		audioPlayer = nil
	}

	ps := &PlayerSystem{
		config:     cfg,
		database:   db,
		actionChan: make(chan structures.SoundAction, 100),
		stopChan:   make(chan struct{}),
		player:     audioPlayer,
		cacheDir:   cacheDir,
		state: &structures.PlayerState{
			MusicStatus:  make(map[string]structures.MusicDownloadStatus),
			Volume:       cfg.DefaultVolume,
			ListSelector: &structures.ListSelector{},
		},
	}

	// Set initial volume once
	if ps.player != nil {
		logger.Debug("Setting initial volume to: %.2f", cfg.DefaultVolume)
		ps.player.SetVolume(cfg.DefaultVolume)
	}

	return ps
}

// Start starts the player system
func (ps *PlayerSystem) Start() error {
	// Don't reset volume here - it's already set in NewPlayerSystem
	go ps.run()
	go ps.updateLoop()
	return nil
}

// Stop stops the player system
func (ps *PlayerSystem) Stop() {
	close(ps.stopChan)
	if ps.player != nil {
		ps.player.Close()
	}
}

// SetDownloadCallback sets the callback for automatic download queueing
func (ps *PlayerSystem) SetDownloadCallback(callback func(track structures.Track)) {
	ps.downloadCallback = callback
}

// SetAPIClient sets the API client for fetching additional track information
func (ps *PlayerSystem) SetAPIClient(client interface{}) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.apiClient = client
}

// SendAction sends an action to the player
func (ps *PlayerSystem) SendAction(action structures.SoundAction) {
	select {
	case ps.actionChan <- action:
	default:
		// Channel full, drop action
	}
}

// GetState returns a copy of the current player state
func (ps *PlayerSystem) GetState() structures.PlayerState {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	// Update state from audio player
	if ps.player != nil {
		ps.state.Volume = ps.player.GetVolume()
		ps.state.IsPlaying = ps.player.IsPlaying()
		ps.state.CurrentTime = ps.player.GetPosition()
		ps.state.TotalTime = ps.player.GetDuration()
	}

	// Create a deep copy
	stateCopy := *ps.state
	stateCopy.MusicStatus = make(map[string]structures.MusicDownloadStatus)
	for k, v := range ps.state.MusicStatus {
		stateCopy.MusicStatus[k] = v
	}

	return stateCopy
}

// run is the main loop of the player system
func (ps *PlayerSystem) run() {
	for {
		select {
		case action := <-ps.actionChan:
			ps.handleAction(action)

		case <-ps.stopChan:
			return
		}
	}
}

// updateLoop periodically updates the player state - simplified version
func (ps *PlayerSystem) updateLoop() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Skip update if we're in the middle of a critical operation
			if atomic.LoadInt32(&ps.skipUpdate) != 0 {
				continue
			}

			ps.mu.Lock()
			if ps.player != nil {
				ps.state.CurrentTime = ps.player.GetPosition()
				ps.state.IsPlaying = ps.player.IsPlaying()

				// Simplified debug logging every 5 seconds
				if int(ps.state.CurrentTime.Seconds())%5 == 0 && ps.state.CurrentTime.Milliseconds()%5000 < 100 {
					// logger.Debug("Position: %v/%v, Playing=%v",
					// ps.state.CurrentTime, ps.state.TotalTime, ps.state.IsPlaying)
				}

				// Check if we've reached the end of the current song
				if ps.state.IsPlaying && ps.player.HasEnded() && !ps.player.IsRecentSeek() {
					logger.Debug("Song ended, advancing to next song")
					ps.nextSong()
				}
			}
			ps.mu.Unlock()

		case <-ps.stopChan:
			return
		}
	}
}

// refreshDownloadStatus updates the download status for all tracks in the list
func (ps *PlayerSystem) refreshDownloadStatus() {
	for _, track := range ps.state.List {
		// Check if file exists in database
		if _, exists := ps.database.Get(track.TrackID); exists {
			ps.state.MusicStatus[track.TrackID] = structures.Downloaded
		} else {
			// Check if file exists in cache
			cachePath := filepath.Join(ps.cacheDir, "downloads", track.TrackID+".mp3")
			if _, err := os.Stat(cachePath); err == nil {
				ps.state.MusicStatus[track.TrackID] = structures.Downloaded
			} else {
				// Check if it's currently downloading
				if status, ok := ps.state.MusicStatus[track.TrackID]; ok && status == structures.Downloading {
					// Keep downloading status
				} else {
					ps.state.MusicStatus[track.TrackID] = structures.NotDownloaded
				}
			}
		}
	}
}

// handleAction processes player actions - simplified version
func (ps *PlayerSystem) handleAction(action structures.SoundAction) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	switch a := action.(type) {
	case structures.PlayPauseAction:
		if !ps.validatePlayerState() {
			logger.Warn("Cannot toggle playback: invalid player state")
			ps.loadCurrentSong()
			return
		}

		if ps.state.IsPlaying {
			if err := ps.player.Pause(); err != nil {
				logger.Error("Failed to pause playback: %v", err)
				ps.loadCurrentSong()
			} else {
				ps.state.IsPlaying = false
				logger.Debug("Playback paused")
			}
		} else {
			if err := ps.player.Play(); err != nil {
				logger.Error("Failed to start playback: %v", err)
				ps.loadCurrentSong()
			} else {
				ps.state.IsPlaying = true
				ps.state.CurrentTime = ps.player.GetPosition()
				ps.state.TotalTime = ps.player.GetDuration()
				logger.Debug("Playback started")
			}
		}

	case structures.PlayAction:
		if !ps.validatePlayerState() {
			logger.Warn("Cannot play: invalid player state")
			ps.loadCurrentSong()
			return
		}

		if !ps.state.IsPlaying {
			ps.loadCurrentSong()
			if err := ps.player.Play(); err != nil {
				logger.Error("Failed to start playback: %v", err)
				ps.handleLoadFailure()
			} else {
				ps.state.IsPlaying = true
				ps.state.CurrentTime = ps.player.GetPosition()
				ps.state.TotalTime = ps.player.GetDuration()
				logger.Debug("Playback started")
			}
		} else {
			// Restart current song
			if err := ps.player.Stop(); err != nil {
				logger.Error("Failed to stop playback: %v", err)
			}
			ps.loadCurrentSong()
			if err := ps.player.Play(); err != nil {
				logger.Error("Failed to restart playback: %v", err)
				ps.handleLoadFailure()
			} else {
				ps.state.IsPlaying = true
				ps.state.CurrentTime = ps.player.GetPosition()
				ps.state.TotalTime = ps.player.GetDuration()
				logger.Debug("Playback restarted")
			}
		}

	case structures.PauseAction:
		if !ps.validatePlayerState() {
			logger.Warn("Cannot pause: invalid player state")
			return
		}

		if err := ps.player.Pause(); err != nil {
			logger.Error("Failed to pause playback: %v", err)
			ps.loadCurrentSong()
		} else {
			ps.state.IsPlaying = false
			logger.Debug("Playback paused")
		}

	case structures.VolumeUpAction:
		if ps.player != nil {
			if err := ps.player.VolumeUp(); err != nil {
				logger.Error("Failed to increase volume: %v", err)
			}
			ps.state.Volume = ps.player.GetVolume()
		}

	case structures.VolumeDownAction:
		if ps.player != nil {
			if err := ps.player.VolumeDown(); err != nil {
				logger.Error("Failed to decrease volume: %v", err)
			}
			ps.state.Volume = ps.player.GetVolume()
		}

	case structures.ForwardAction:
		if ps.validatePlayerState() && ps.state.TotalTime > 0 {
			// Temporarily disable position updates during seek
			atomic.StoreInt32(&ps.skipUpdate, 1)
			if err := ps.player.SeekForward(time.Duration(ps.config.SeekSeconds) * time.Second); err != nil {
				logger.Error("Failed to seek forward: %v", err)
			} else {
				logger.Debug("Seeked forward by %d seconds", ps.config.SeekSeconds)
			}
			// Re-enable updates after a short delay
			go func() {
				time.Sleep(200 * time.Millisecond)
				atomic.StoreInt32(&ps.skipUpdate, 0)
			}()
		}

	case structures.BackwardAction:
		if ps.validatePlayerState() && ps.state.TotalTime > 0 {
			// Temporarily disable position updates during seek
			atomic.StoreInt32(&ps.skipUpdate, 1)
			if err := ps.player.SeekBackward(time.Duration(ps.config.SeekSeconds) * time.Second); err != nil {
				logger.Error("Failed to seek backward: %v", err)
			} else {
				logger.Debug("Seeked backward by %d seconds", ps.config.SeekSeconds)
			}
			// Re-enable updates after a short delay
			go func() {
				time.Sleep(200 * time.Millisecond)
				atomic.StoreInt32(&ps.skipUpdate, 0)
			}()
		}

	case structures.NextAction:
		ps.nextSong()

	case structures.PreviousAction:
		ps.previousSong()

	case structures.AddTracksToQueueAction:
		for _, track := range a.Tracks {
			ps.state.List = append(ps.state.List, track)
			// Queue for download
			if ps.downloadCallback != nil {
				ps.downloadCallback(track)
			}
		}
		// Refresh status for all tracks
		ps.refreshDownloadStatus()

	case structures.AddTrackAction:
		if len(ps.state.List) == 0 {
			ps.state.List = append(ps.state.List, a.Track)
		} else {
			// Insert after current
			ps.state.List = append(ps.state.List[:ps.state.Current+1],
				append([]structures.Track{a.Track}, ps.state.List[ps.state.Current+1:]...)...)
		}
		ps.state.MusicStatus[a.Track.TrackID] = structures.NotDownloaded
		// Queue for download
		if ps.downloadCallback != nil {
			ps.downloadCallback(a.Track)
		}

	case structures.ReplaceQueueAction:
		// Keep only up to current position
		if ps.state.Current+1 < len(ps.state.List) {
			ps.state.List = ps.state.List[:ps.state.Current+1]
		}
		// Add new tracks
		for _, track := range a.Tracks {
			ps.state.List = append(ps.state.List, track)
			// Queue for download
			if ps.downloadCallback != nil {
				ps.downloadCallback(track)
			}
		}
		// Refresh status for all tracks
		ps.refreshDownloadStatus()
		// Move to next
		ps.nextSong()

	case structures.TrackStatusUpdateAction:
		ps.state.MusicStatus[a.TrackID] = a.Status
		// If the track just finished downloading and it's the current track, reload it
		if a.Status == structures.Downloaded &&
			ps.state.Current < len(ps.state.List) &&
			ps.state.List[ps.state.Current].TrackID == a.TrackID {
			logger.Debug("Current track download completed, reloading...")
			// Give the database a moment to sync
			go func() {
				time.Sleep(200 * time.Millisecond)
				ps.mu.Lock()
				defer ps.mu.Unlock()
				ps.loadCurrentSong()
				// If we were trying to play, start playback now
				if ps.state.IsPlaying && ps.player != nil {
					if err := ps.player.Play(); err != nil {
						logger.Error("Failed to start playback after download: %v", err)
						ps.state.IsPlaying = false
					} else {
						logger.Info("Started playback after download completed")
					}
				}
			}()
		}

	case structures.DeleteTrackAction:
		ps.deleteCurrentTrack()

	case structures.CleanupAction:
		ps.state.List = nil
		ps.state.Current = 0
		ps.state.MusicStatus = make(map[string]structures.MusicDownloadStatus)
		if ps.player != nil {
			ps.player.Stop()
		}

	case structures.RestartPlayerAction:
		// Restart the current song
		ps.loadCurrentSong()

	case structures.SeekAction:
		// Seek to specific position
		if ps.player != nil && ps.validatePlayerState() {
			if err := ps.player.Seek(a.Position); err != nil {
				logger.Error("Failed to seek: %v", err)
			}
		}
	}
}

// nextSong advances to the next song - simplified version
func (ps *PlayerSystem) nextSong() {
	// Disable updates during song transition
	atomic.StoreInt32(&ps.skipUpdate, 1)
	defer func() {
		// Re-enable updates after transition
		go func() {
			time.Sleep(300 * time.Millisecond)
			atomic.StoreInt32(&ps.skipUpdate, 0)
		}()
	}()

	if ps.state.Current+1 < len(ps.state.List) {
		wasPlaying := ps.state.IsPlaying
		ps.state.Current++
		ps.loadCurrentSong()
		// Maintain playing state
		if wasPlaying && ps.player != nil {
			if err := ps.player.Play(); err != nil {
				logger.Error("Failed to start playback after advancing to next song: %v", err)
				ps.state.IsPlaying = false
			} else {
				ps.state.IsPlaying = true
			}
		}
	} else {
		// Reached end of playlist, stop playing
		ps.state.IsPlaying = false
		if ps.player != nil {
			ps.player.Stop()
		}
		logger.Debug("Reached end of playlist")
	}
}

// previousSong goes back to the previous song
func (ps *PlayerSystem) previousSong() {
	if ps.state.Current > 0 {
		wasPlaying := ps.state.IsPlaying
		ps.state.Current--
		ps.loadCurrentSong()
		// Maintain playing state
		if wasPlaying && ps.player != nil {
			if err := ps.player.Play(); err != nil {
				logger.Error("Failed to start playback after going to previous song: %v", err)
				ps.state.IsPlaying = false
			} else {
				ps.state.IsPlaying = true
			}
		}
	}
}

// loadCurrentSong loads the current song for playback - simplified version
func (ps *PlayerSystem) loadCurrentSong() {
	if !ps.validatePlayerState() {
		return
	}

	currentTrack := ps.state.List[ps.state.Current]
	logger.Info("Loading song: %s by %s", currentTrack.Title, strings.Join(currentTrack.Artists, ", "))

	// Check if the file is downloaded
	if entry, exists := ps.database.Get(currentTrack.TrackID); exists {
		logger.Debug("Loading from database: %s", entry.FilePath)
		if err := ps.player.LoadFile(entry.FilePath); err != nil {
			logger.Error("Failed to load file %s: %v", entry.FilePath, err)
			ps.state.MusicStatus[currentTrack.TrackID] = structures.DownloadFailed
			return
		}

		ps.state.TotalTime = ps.player.GetDuration()

		// Update track duration with actual file duration if different
		actualDurationSeconds := int(ps.state.TotalTime.Seconds() + 0.999)
		if actualDurationSeconds != currentTrack.Duration && actualDurationSeconds > 0 {
			logger.Debug("Updating track duration from %d to %d seconds", currentTrack.Duration, actualDurationSeconds)
			currentTrack.Duration = actualDurationSeconds
			ps.state.List[ps.state.Current].Duration = actualDurationSeconds
			entry.Track.Duration = actualDurationSeconds
			if err := ps.database.Add(*entry); err != nil {
				logger.Error("Failed to update duration in database: %v", err)
			}
		}

		// Log bitrate information if available
		if entry.Track.AudioBitrate > 0 {
			logger.Info("Song loaded: %s by %s (%d kbps, %s quality)",
				currentTrack.Title,
				strings.Join(currentTrack.Artists, ", "),
				entry.Track.AudioBitrate,
				entry.Track.AudioQuality)
		} else {
			logger.Debug("Song loaded successfully, duration: %v", ps.state.TotalTime)
			// Fetch bitrate info from API in background if not available
			go ps.fetchAndUpdateBitrate(currentTrack)
		}

		// Auto-play if we were playing before
		if ps.state.IsPlaying {
			if err := ps.player.Play(); err != nil {
				logger.Error("Failed to start playback: %v", err)
				ps.state.IsPlaying = false
			}
		}
	} else {
		// Check if it's currently downloading
		if status, ok := ps.state.MusicStatus[currentTrack.TrackID]; ok && status == structures.Downloading {
			logger.Info("Track is currently downloading, waiting for completion...")
			return
		}

		// Try to find the file in cache directory
		cachePath := filepath.Join(ps.cacheDir, "downloads", currentTrack.TrackID+".mp3")
		logger.Debug("Trying to load from cache: %s", cachePath)

		if _, err := os.Stat(cachePath); err == nil {
			if err := ps.player.LoadFile(cachePath); err == nil {
				ps.state.TotalTime = ps.player.GetDuration()
				logger.Debug("Song loaded from cache, duration: %v", ps.state.TotalTime)

				// Update track duration with actual file duration
				actualDurationSeconds := int(ps.state.TotalTime.Seconds() + 0.999)
				if actualDurationSeconds != currentTrack.Duration && actualDurationSeconds > 0 {
					logger.Debug("Updating track duration from %d to %d seconds", currentTrack.Duration, actualDurationSeconds)
					currentTrack.Duration = actualDurationSeconds
					ps.state.List[ps.state.Current].Duration = actualDurationSeconds
				}

				// Add to database for future reference
				fileInfo, _ := os.Stat(cachePath)
				currentTrack.Duration = actualDurationSeconds
				entry := structures.DatabaseEntry{
					Track:    currentTrack,
					FilePath: cachePath,
					AddedAt:  time.Now(),
					FileSize: fileInfo.Size(),
				}
				if err := ps.database.Add(entry); err != nil {
					logger.Error("Failed to add to database: %v", err)
				}

				if ps.state.IsPlaying {
					if err := ps.player.Play(); err != nil {
						logger.Error("Failed to start playback: %v", err)
						ps.state.IsPlaying = false
					}
				}
			} else {
				logger.Error("Failed to load file from cache: %v", err)
				ps.state.MusicStatus[currentTrack.TrackID] = structures.NotDownloaded
			}
		} else {
			logger.Debug("File not found in cache: %s", cachePath)
			ps.state.MusicStatus[currentTrack.TrackID] = structures.NotDownloaded
			// Queue for download if callback is set
			if ps.downloadCallback != nil {
				logger.Info("Queueing for download: %s", currentTrack.TrackID)
				ps.downloadCallback(currentTrack)
			}
		}
	}
}

// handleLoadFailure handles the case when current song fails to load
func (ps *PlayerSystem) handleLoadFailure() {
	currentTrack := ps.state.List[ps.state.Current]
	logger.Warn("Failed to load track: %s, attempting to skip", currentTrack.Title)

	// Mark as failed
	ps.state.MusicStatus[currentTrack.TrackID] = structures.DownloadFailed

	// Try to advance to next song if available
	if ps.state.Current+1 < len(ps.state.List) {
		logger.Debug("Advancing to next song due to load failure")
		ps.nextSong()
	} else {
		// No more songs, stop playback
		logger.Debug("No more songs available, stopping playback")
		ps.state.IsPlaying = false
		if ps.player != nil {
			ps.player.Stop()
		}
	}
}

// validatePlayerState ensures the player state is consistent
func (ps *PlayerSystem) validatePlayerState() bool {
	if ps.player == nil {
		logger.Error("Player is nil")
		return false
	}

	if ps.state.Current < 0 || ps.state.Current >= len(ps.state.List) {
		logger.Error("Invalid current track index: %d (list size: %d)", ps.state.Current, len(ps.state.List))
		return false
	}

	return true
}

// deleteCurrentTrack removes the current track from the playlist and deletes its files
func (ps *PlayerSystem) deleteCurrentTrack() {
	if ps.state.Current < 0 || ps.state.Current >= len(ps.state.List) {
		return
	}

	currentTrack := ps.state.List[ps.state.Current]

	// Stop playback
	if ps.player != nil {
		ps.player.Stop()
	}

	// Remove from database and delete files
	if entry, exists := ps.database.Get(currentTrack.TrackID); exists {
		// Remove from database
		if err := ps.database.Remove(currentTrack.TrackID); err != nil {
			logger.Error("Failed to remove track from database: %v", err)
		}

		// Delete the file
		if err := os.Remove(entry.FilePath); err != nil {
			logger.Error("Failed to delete file %s: %v", entry.FilePath, err)
		}

		// Also try to delete JSON metadata if it exists
		jsonPath := strings.TrimSuffix(entry.FilePath, filepath.Ext(entry.FilePath)) + ".json"
		if err := os.Remove(jsonPath); err != nil && !os.IsNotExist(err) {
			logger.Error("Failed to delete JSON file %s: %v", jsonPath, err)
		}
	}

	// Remove from music status
	delete(ps.state.MusicStatus, currentTrack.TrackID)

	// Remove from playlist
	ps.state.List = append(ps.state.List[:ps.state.Current], ps.state.List[ps.state.Current+1:]...)

	// Update list selector
	if ps.state.ListSelector != nil && ps.state.ListSelector.ListSize > 0 {
		ps.state.ListSelector.ListSize--
	}

	// Adjust current position
	if ps.state.Current >= len(ps.state.List) && ps.state.Current > 0 {
		ps.state.Current--
	}

	// If there are still songs, play the next one
	if len(ps.state.List) > 0 {
		ps.loadCurrentSong()
	}
}

// fetchAndUpdateBitrate fetches bitrate information from API and updates the database
func (ps *PlayerSystem) fetchAndUpdateBitrate(track structures.Track) {
	// Check if API client is available
	if ps.apiClient == nil {
		return
	}

	// Type assertion for the API client
	type StreamingDataFetcher interface {
		GetStreamingData(videoID string) (*api.StreamingData, error)
	}

	fetcher, ok := ps.apiClient.(StreamingDataFetcher)
	if !ok {
		return
	}

	// Check if we already have bitrate info
	if entry, exists := ps.database.Get(track.TrackID); exists && entry.Track.AudioBitrate > 0 {
		return
	}

	// Fetch streaming data
	streamingData, err := fetcher.GetStreamingData(track.TrackID)
	if err != nil {
		logger.Debug("Failed to fetch streaming data for %s: %v", track.TrackID, err)
		return
	}

	// Find the best audio format
	var bestBitrate int
	for _, format := range streamingData.AdaptiveFormats {
		// Check if it's an audio format
		if strings.HasPrefix(format.MimeType, "audio/") && format.Bitrate > bestBitrate {
			bestBitrate = format.Bitrate
		}
	}

	if bestBitrate > 0 {
		// Convert to kbps
		bitrateKbps := bestBitrate / 1000
		logger.Info("Fetched bitrate info from API for %s: %d kbps", track.TrackID, bitrateKbps)

		// Update database if entry exists
		if entry, exists := ps.database.Get(track.TrackID); exists {
			entry.Track.AudioBitrate = bitrateKbps
			if err := ps.database.Add(*entry); err != nil {
				logger.Error("Failed to update bitrate in database: %v", err)
			}
		}
	}
}
