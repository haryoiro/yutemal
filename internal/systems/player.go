package systems

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

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
	skipUpdate       bool // Flag to skip position updates during critical operations
}

// NewPlayerSystem creates a new player system
func NewPlayerSystem(cfg *structures.Config, db database.DB, cacheDir string) *PlayerSystem {
	audioPlayer, err := player.New()
	if err != nil {
		logger.Error("Failed to create audio player: %v", err)
		audioPlayer = nil
	}

	return &PlayerSystem{
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
}

// Start starts the player system
func (ps *PlayerSystem) Start() error {
	if ps.player != nil {
		ps.player.SetVolume(ps.config.DefaultVolume)
	}
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

// updateLoop periodically updates the player state
func (ps *PlayerSystem) updateLoop() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ps.mu.Lock()
			// Skip update if we're in the middle of a critical operation
			if ps.skipUpdate {
				ps.mu.Unlock()
				continue
			}
			if ps.player != nil {
				ps.state.CurrentTime = ps.player.GetPosition()
				ps.state.IsPlaying = ps.player.IsPlaying()

				// Check if we've reached the end of the current song
				if ps.state.IsPlaying && 
				   ps.state.CurrentTime >= ps.state.TotalTime-time.Millisecond*200 &&
				   ps.state.TotalTime > 0 {
					// Check if this is a recent seek to near the end
					if ps.player.IsRecentSeek() {
						// If we seeked to near the end, still advance after a short delay
						// This prevents getting stuck at the end after manual seek
						if ps.state.CurrentTime >= ps.state.TotalTime-time.Millisecond*100 {
							logger.Debug("Seeked to end of song, advancing to next (current: %v, total: %v)", 
								ps.state.CurrentTime, ps.state.TotalTime)
							ps.nextSong()
						}
					} else {
						// Natural end of song
						logger.Debug("Song ended naturally, advancing to next song (current: %v, total: %v)", 
							ps.state.CurrentTime, ps.state.TotalTime)
						ps.nextSong()
					}
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

// handleAction processes player actions
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
				// Try to reload the current song
				ps.loadCurrentSong()
			} else {
				ps.state.IsPlaying = false
				logger.Debug("Playback paused")
			}
		} else {
			if err := ps.player.Play(); err != nil {
				logger.Error("Failed to start playback: %v", err)
				// Try to reload the current song
				ps.loadCurrentSong()
			} else {
				ps.state.IsPlaying = true
				ps.state.CurrentTime = ps.player.GetPosition()
				ps.state.TotalTime = ps.player.GetDuration()
				logger.Debug("Playback started, current time: %v, total time: %v",
					ps.state.CurrentTime, ps.state.TotalTime)
			}
		}
	case structures.PlayAction:
		if !ps.validatePlayerState() {
			logger.Warn("Cannot play: invalid player state")
			ps.loadCurrentSong()
			return
		}
		
		logger.Debug("PlayAction: current=%d, list size=%d, isPlaying=%v", 
			ps.state.Current, len(ps.state.List), ps.state.IsPlaying)
			
		if !ps.state.IsPlaying {
			ps.loadCurrentSong()
			if err := ps.player.Play(); err != nil {
				logger.Error("Failed to start playback: %v", err)
				ps.handleLoadFailure()
			} else {
				ps.state.IsPlaying = true
				ps.state.CurrentTime = ps.player.GetPosition()
				ps.state.TotalTime = ps.player.GetDuration()
				logger.Debug("Playback started, current time: %v, total time: %v",
					ps.state.CurrentTime, ps.state.TotalTime)
			}
		} else {
			// 一旦停止してclearしてから再生（曲の再開始）
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
				logger.Debug("Playback restarted, current time: %v, total time: %v",
					ps.state.CurrentTime, ps.state.TotalTime)
			}
		}
	case structures.PauseAction:
		if !ps.validatePlayerState() {
			logger.Warn("Cannot pause: invalid player state")
			return
		}
		
		if err := ps.player.Pause(); err != nil {
			logger.Error("Failed to pause playback: %v", err)
			// Try to reload the current song
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
			ps.skipUpdate = true
			if err := ps.player.SeekForward(time.Duration(ps.config.SeekSeconds) * time.Second); err != nil {
				logger.Error("Failed to seek forward: %v", err)
			} else {
				logger.Debug("Seeked forward by %d seconds", ps.config.SeekSeconds)
			}
			// Re-enable updates after a short delay
			go func() {
				time.Sleep(200 * time.Millisecond)
				ps.mu.Lock()
				ps.skipUpdate = false
				ps.mu.Unlock()
			}()
		}

	case structures.BackwardAction:
		if ps.validatePlayerState() && ps.state.TotalTime > 0 {
			// Temporarily disable position updates during seek
			ps.skipUpdate = true
			if err := ps.player.SeekBackward(time.Duration(ps.config.SeekSeconds) * time.Second); err != nil {
				logger.Error("Failed to seek backward: %v", err)
			} else {
				logger.Debug("Seeked backward by %d seconds", ps.config.SeekSeconds)
			}
			// Re-enable updates after a short delay
			go func() {
				time.Sleep(200 * time.Millisecond)
				ps.mu.Lock()
				ps.skipUpdate = false
				ps.mu.Unlock()
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
	}
}

// nextSong advances to the next song
func (ps *PlayerSystem) nextSong() {
	// Disable updates during song transition
	ps.skipUpdate = true
	defer func() {
		// Re-enable updates after transition
		go func() {
			time.Sleep(300 * time.Millisecond)
			ps.mu.Lock()
			ps.skipUpdate = false
			ps.mu.Unlock()
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

// loadCurrentSong loads the current song for playback
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
			// Update status to failed
			ps.state.MusicStatus[currentTrack.TrackID] = structures.DownloadFailed
			return
		}

		ps.state.TotalTime = ps.player.GetDuration()
		ps.state.CurrentTime = 0
		logger.Debug("Song loaded successfully, duration: %v", ps.state.TotalTime)

		// Auto-play if we were playing before
		if ps.state.IsPlaying {
			if err := ps.player.Play(); err != nil {
				logger.Error("Failed to start playback: %v", err)
				ps.state.IsPlaying = false
			}
		}
	} else {
		// Try to find the file in cache directory
		cachePath := filepath.Join(ps.cacheDir, "downloads", currentTrack.TrackID+".mp3")
		logger.Debug("Trying to load from cache: %s", cachePath)

		if _, err := os.Stat(cachePath); err == nil {
			if err := ps.player.LoadFile(cachePath); err == nil {
				ps.state.TotalTime = ps.player.GetDuration()
				ps.state.CurrentTime = 0
				logger.Debug("Song loaded from cache, duration: %v", ps.state.TotalTime)

				// Add to database for future reference
				fileInfo, _ := os.Stat(cachePath)
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
