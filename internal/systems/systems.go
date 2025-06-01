package systems

import (
	"github.com/haryoiro/yutemal/internal/database"
	"github.com/haryoiro/yutemal/internal/structures"
)

// Systems contains all the core systems of the application
type Systems struct {
	Config   *structures.Config
	Database database.DB
	CacheDir string
	Player   *PlayerSystem
	Download *DownloadSystem
	API      *APISystem
}

// New creates a new Systems instance
func New(cfg *structures.Config, db database.DB, cacheDir string) *Systems {
	s := &Systems{
		Config:   cfg,
		Database: db,
		CacheDir: cacheDir,
	}

	// Initialize subsystems
	s.Player = NewPlayerSystem(cfg, db, cacheDir)
	s.Download = NewDownloadSystem(cfg, db, cacheDir)
	s.API = NewAPISystem(cfg)

	return s
}

// Start starts all systems
func (s *Systems) Start() error {
	// Connect download status updates to player
	s.Download.SetStatusCallback(func(trackID string, status structures.MusicDownloadStatus) {
		s.Player.SendAction(structures.TrackStatusUpdateAction{
			TrackID: trackID,
			Status:  status,
		})
	})

	// Connect player download requests to download system
	s.Player.SetDownloadCallback(func(video structures.Track) {
		s.QueueVideoForDownload(video)
	})

	// Start player system
	if err := s.Player.Start(); err != nil {
		return err
	}

	// Start download system
	if err := s.Download.Start(); err != nil {
		return err
	}

	return nil
}

// Stop stops all systems
func (s *Systems) Stop() error {
	s.Player.Stop()
	s.Download.Stop()
	return nil
}

// QueueVideoForDownload checks if a video needs downloading and queues it
func (s *Systems) QueueVideoForDownload(video structures.Track) {
	// Check if already downloaded
	if _, exists := s.Database.Get(video.TrackID); exists {
		return
	}

	// Queue for download
	s.Download.QueueDownload(video)
}
