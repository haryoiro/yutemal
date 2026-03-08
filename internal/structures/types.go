package structures

import (
	"time"
)

// MusicDownloadStatus represents the download status of a music file.
type MusicDownloadStatus int

const (
	NotDownloaded MusicDownloadStatus = iota
	Downloading
	Downloaded
	DownloadFailed
)

// Track represents a music track.
// Fields ordered by size (largest first) to minimize padding on ARM64/AMD64.
type Track struct {
	Artists      []string `json:"artists"`                 // 24 bytes (slice header)
	TrackID      string   `json:"track_id"`                // 16 bytes
	Title        string   `json:"title"`                   // 16 bytes
	Thumbnail    string   `json:"thumbnail,omitempty"`     // 16 bytes
	AudioQuality string   `json:"audio_quality,omitempty"` // 16 bytes
	Duration     int      `json:"duration"`                // 8 bytes (in seconds)
	AudioBitrate int      `json:"audio_bitrate,omitempty"` // 8 bytes (kbps)
	IsAvailable  bool     `json:"is_available"`            // 1 byte
	IsExplicit   bool     `json:"is_explicit"`             // 1 byte + 6 padding
}

// Section represents a content section on the home page.
type Section struct {
	ID       string        `json:"id"`
	Title    string        `json:"title"`
	Type     SectionType   `json:"type"`
	Contents []ContentItem `json:"contents"`
}

// SectionType represents the type of section.
type SectionType string

const (
	SectionTypeLibraryPlaylists     SectionType = "library_playlists"
	SectionTypeLikedPlaylists       SectionType = "liked_playlists"
	SectionTypeRecommendedPlaylists SectionType = "recommended_playlists"
	SectionTypeRecentActivity       SectionType = "recent_activity"
	SectionTypeHomeFeed             SectionType = "home_feed"
)

// ContentItem represents an item in a section.
type ContentItem struct {
	Type     string    `json:"type"` // "track", "playlist", "album", etc.
	Track    *Track    `json:"track,omitempty"`
	Playlist *Playlist `json:"playlist,omitempty"`
}

// Playlist represents a playlist with metadata.
type Playlist struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Thumbnail   string `json:"thumbnail"`
	VideoCount  int    `json:"video_count"`
}

// SoundAction represents actions that can be sent to the player.
type SoundAction any

// Player actions.
type PlayPauseAction struct{}
type PlayAction struct{}
type PauseAction struct{}
type RestartPlayerAction struct{}
type VolumeUpAction struct{}
type VolumeDownAction struct{}
type ForwardAction struct{}
type BackwardAction struct{}
type NextAction struct{ Skip int }
type PreviousAction struct{ Skip int }
type CleanupAction struct{}
type AddTracksToQueueAction struct{ Tracks []Track }
type AddTrackAction struct{ Track Track }
type InsertTrackAfterCurrentAction struct{ Track Track }
type DeleteTrackAction struct{}
type DeleteTrackAtIndexAction struct{ Index int }
type ReplaceQueueAction struct{ Tracks []Track }
type TrackStatusUpdateAction struct {
	TrackID string
	Status  MusicDownloadStatus
}
type SeekAction struct {
	Position time.Duration
}
type ShuffleQueueAction struct{}
type JumpToIndexAction struct{ Index int }

// PlayerState represents the current state of the music player.
// Fields ordered to minimize padding and group hot fields together.
type PlayerState struct {
	List         []Track                       // 24 bytes (slice header)
	MusicStatus  map[string]MusicDownloadStatus // 8 bytes (pointer)
	ListSelector *ListSelector                 // 8 bytes (pointer)
	Volume       float64                       // 8 bytes
	CurrentTime  time.Duration                 // 8 bytes
	TotalTime    time.Duration                 // 8 bytes
	Current      int                           // 8 bytes
	IsPlaying    bool                          // 1 byte + 7 padding
}

// ListSelector manages list navigation.
type ListSelector struct {
	Position  int
	ListSize  int
	ViewStart int
	ViewSize  int
}

// AppStatus represents the application status.
type AppStatus struct {
	TotalTasks     int
	CompletedTasks int
	FailedTasks    int
	CurrentTask    string
	IsDownloading  bool
}

// Config represents the application configuration.
type Config struct {
	Theme       Theme       `toml:"theme"`
	KeyBindings KeyBindings `toml:"key_bindings"`

	// Download Configuration
	DownloadDir            string `toml:"download_dir"`
	MaxConcurrentDownloads int    `toml:"max_concurrent_downloads"`
	MaxCacheSize           int64  `toml:"max_cache_size"` // in MB
	AudioQuality           string `toml:"audio_quality"`  // Audio quality: low/medium/high/best

	// Player Configuration
	DefaultVolume float64 `toml:"default_volume"`
	SeekSeconds   int     `toml:"seek_seconds"`

	// Authentication Configuration
	Browser        string `toml:"browser"`         // Browser to read cookies from: "chrome", "chrome-canary", "chromium"
	BrowserProfile string `toml:"browser_profile"` // Browser profile name (e.g., "Default", "Profile 1")

	// UI Configuration
	DisableAltScreen bool `toml:"disable_alt_screen"` // Disable alternate screen for Kitty graphics compatibility
}

// Theme represents the UI theme configuration.
type Theme struct {
	Background       string `toml:"background"`         // Note: Not used to avoid partial background coloring
	Foreground       string `toml:"foreground"`         // Default text color
	Selected         string `toml:"selected"`           // Selected item color
	Playing          string `toml:"playing"`            // Currently playing item color
	Border           string `toml:"border"`             // Border color
	ProgressBar      string `toml:"progress_bar"`       // Progress bar background
	ProgressBarFill  string `toml:"progress_bar_fill"`  // Progress bar fill color
	ProgressBarStyle string `toml:"progress_bar_style"` // Progress bar style: "line", "block", "gradient"
}

// KeyBindings represents configurable keyboard shortcuts.
type KeyBindings struct {
	// Global controls
	PlayPause    string   `toml:"play_pause"`
	Quit         string   `toml:"quit"`
	VolumeUp     []string `toml:"volume_up"`
	VolumeDown   []string `toml:"volume_down"`
	SeekForward  string   `toml:"seek_forward"`
	SeekBackward string   `toml:"seek_backward"`

	// Navigation
	MoveUp      []string `toml:"move_up"`
	MoveDown    []string `toml:"move_down"`
	Select      []string `toml:"select"`
	Back        []string `toml:"back"`
	NextSection string   `toml:"next_section"`
	PrevSection string   `toml:"prev_section"`

	// Actions
	Search      string `toml:"search"`
	Shuffle     string `toml:"shuffle"`
	RemoveTrack string `toml:"remove_track"`
	Home        string `toml:"home"`
}

// Database entry structure.
type DatabaseEntry struct {
	Track    Track
	AddedAt  time.Time
	FilePath string
	FileSize int64
}
