package constants

import "time"

// Queue and worker pool sizes
const (
	DefaultQueueSize    = 1000
	DefaultWorkerCount  = 10
	DefaultMaxBatchSize = 100
)

// Timing constants
const (
	DefaultSleepMs       = 100 * time.Millisecond
	MarqueeTickInterval  = 150 * time.Millisecond
	PlayerUpdateInterval = 50 * time.Millisecond
	DownloadRetryDelay   = 2 * time.Second
	CleanupCheckInterval = 24 * time.Hour
)

// UI constants
const (
	DefaultPlayerHeight = 5
	DefaultMaxWidth     = 80
	MinVisibleItems     = 3
	ScrollPadding       = 2
)

// Audio player constants
const (
	SecondsPerMinute  = 60
	DefaultSampleRate = 44100
	DefaultBufferSize = 4096
	VolumeStep        = 0.05 // 5% volume change per step
	SeekSeconds       = 10   // Seconds to seek forward/backward
)

// Download constants
const (
	MaxDownloadRetries = 3
	AudioQuality       = "0" // Best quality for yt-dlp
)

// File size constants
const (
	KB = 1024
	MB = 1024 * KB
	GB = 1024 * MB
)

// Database constants
const (
	DefaultCacheSize = 10000
	MaxSearchResults = 50
)
