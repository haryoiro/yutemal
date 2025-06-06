package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/haryoiro/yutemal/internal/config"
	"github.com/haryoiro/yutemal/internal/database"
	"github.com/haryoiro/yutemal/internal/logger"
	"github.com/haryoiro/yutemal/internal/structures"
	"github.com/haryoiro/yutemal/internal/systems"
	"github.com/haryoiro/yutemal/internal/ui"
	"github.com/haryoiro/yutemal/internal/version"
)

const (
	banner = `
██╗   ██╗██╗   ██╗████████╗███████╗███╗   ███╗ █████╗ ██╗
╚██╗ ██╔╝██║   ██║╚══██╔══╝██╔════╝████╗ ████║██╔══██╗██║
 ╚████╔╝ ██║   ██║   ██║   █████╗  ██╔████╔██║███████║██║
  ╚██╔╝  ██║   ██║   ██║   ██╔══╝  ██║╚██╔╝██║██╔══██║██║
   ██║   ╚██████╔╝   ██║   ███████╗██║ ╚═╝ ██║██║  ██║███████╗
   ╚═╝    ╚═════╝    ╚═╝   ╚══════╝╚═╝     ╚═╝╚═╝  ╚═╝╚══════╝
                       YouTube Music AT Terminal`
)

func main() {
	// Parse command line flags
	var (
		showHelp    = flag.Bool("help", false, "Show help message")
		showFiles   = flag.Bool("files", false, "Show file locations")
		fixDB       = flag.Bool("fix-db", false, "Fix database issues")
		clearCache  = flag.Bool("clear-cache", false, "Clear all cache data (downloads, database, logs)")
		showVersion = flag.Bool("version", false, "Show version")
		debugMode   = flag.Bool("debug", false, "Enable debug logging")
	)

	flag.Parse()

	// Handle command line options
	if *showHelp {
		fmt.Println(banner)
		fmt.Println("\nUsage: yutemal [OPTIONS]")
		fmt.Println("\nOptions:")
		flag.PrintDefaults()
		fmt.Println("\nKeyboard shortcuts:")
		fmt.Println("  Global controls:")
		fmt.Println("    Space       - Play/Pause")
		fmt.Println("    ←           - Seek backward")
		fmt.Println("    →           - Seek forward")
		fmt.Println("    + or =      - Volume up")
		fmt.Println("    - or _      - Volume down")
		fmt.Println("    Ctrl+C/D    - Quit application")
		fmt.Println("")
		fmt.Println("  Navigation:")
		fmt.Println("    ↑ or k      - Move selection up")
		fmt.Println("    ↓ or j      - Move selection down")
		fmt.Println("    Enter or l  - Select/play item")
		fmt.Println("    Esc/Backspace - Go back")
		fmt.Println("")
		fmt.Println("  Home view:")
		fmt.Println("    Tab         - Next section")
		fmt.Println("    Shift+Tab   - Previous section")
		fmt.Println("    f           - Open search")
		fmt.Println("")
		fmt.Println("  Playlist view:")
		fmt.Println("    r           - Remove track from playlist")
		fmt.Println("    h           - Return to home")
		fmt.Println("\nDebug options:")
		fmt.Println("  --debug     - Enable debug logging to file")
		return
	}

	if *showVersion {
		fmt.Println(version.Info())
		return
	}

	// Get configuration directories
	configDir, cacheDir, dataDir := getDirectories()

	if *showFiles {
		fmt.Println("# yutemal file locations:")
		fmt.Printf("  Config: %s\n", configDir)
		fmt.Printf("  Cache:  %s\n", cacheDir)
		fmt.Printf("  Data:   %s\n", dataDir)
		fmt.Printf("  Logs:   %s\n", filepath.Join(dataDir, "yutemal.log"))
		return
	}

	if *fixDB {
		fmt.Println("Fixing database...")
		// For SQLite, we don't need a fix function as it handles its own integrity
		fmt.Println("SQLite database self-manages integrity")
		return
	}

	if *clearCache {
		// Ask for confirmation
		fmt.Println("⚠️  WARNING: This will delete all cached data including:")
		fmt.Println("  - Downloaded audio files")
		fmt.Println("  - Database (playlists, tracks)")
		fmt.Println("  - Logs")
		fmt.Println("  - Cookies")
		fmt.Println("\nAre you sure you want to continue? (y/N): ")

		var confirm string
		fmt.Scanln(&confirm)
		if confirm != "y" && confirm != "Y" {
			fmt.Println("Cache clearing cancelled.")
			return
		}

		fmt.Println("Clearing all cache data...")

		// Clear cache directory (includes downloads)
		if err := os.RemoveAll(cacheDir); err != nil {
			fmt.Printf("Failed to clear cache directory: %v\n", err)
		} else {
			fmt.Printf("✓ Cleared cache directory: %s\n", cacheDir)
		}

		// Clear data directory (includes database and logs)
		if err := os.RemoveAll(dataDir); err != nil {
			fmt.Printf("Failed to clear data directory: %v\n", err)
		} else {
			fmt.Printf("✓ Cleared data directory: %s\n", dataDir)
		}

		// Note: We don't clear the config directory as it contains user preferences
		fmt.Println("\n✅ All cache data cleared successfully")
		fmt.Printf("Note: Configuration files in %s were preserved\n", configDir)
		return
	}

	// Check if yt-dlp is installed
	if err := checkYtDlp(); err != nil {
		showYtDlpError()
		return
	}

	// Check if ffprobe is installed
	if err := checkFfprobe(); err != nil {
		showFfprobeError()
		return
	}

	// Initialize logging
	logFile := filepath.Join(dataDir, "yutemal.log")
	if err := initLogging(logFile, *debugMode); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logging: %v\n", err)
		os.Exit(1)
	}
	defer logger.CloseLogger()

	// Load configuration
	configPath := filepath.Join(configDir, "config.toml")
	cfg := loadConfiguration(configPath)

	// Initialize SQLite database
	db := initializeDatabase(dataDir)
	defer func() {
		logger.Debug("Closing database connection")
		db.Close()
	}()

	// Check for authentication
	headerFile := findHeaderFile(configDir)
	if headerFile == "" {
		showAuthenticationError(configDir)
		return
	}

	// Initialize and start systems
	appSystems := initializeSystems(cfg, db, cacheDir, headerFile)
	defer func() {
		logger.Debug("Stopping all application systems...")
		appSystems.Stop()
	}()

	// Run the UI
	logger.Debug("Starting UI")
	if err := ui.RunSimple(appSystems, cfg); err != nil {
		logger.Fatal("Application error: %v", err)
	}

	logger.Info("yutemal application shutdown complete")
}

func getDirectories() (config, cache, data string) {
	// Use XDG Base Directory specification
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		config = filepath.Join(xdgConfig, "yutemal")
	} else if home, err := os.UserHomeDir(); err == nil {
		config = filepath.Join(home, ".config", "yutemal")
	}

	if xdgCache := os.Getenv("XDG_CACHE_HOME"); xdgCache != "" {
		cache = filepath.Join(xdgCache, "yutemal")
	} else if home, err := os.UserHomeDir(); err == nil {
		cache = filepath.Join(home, ".cache", "yutemal")
	}

	if xdgData := os.Getenv("XDG_DATA_HOME"); xdgData != "" {
		data = filepath.Join(xdgData, "yutemal")
	} else if home, err := os.UserHomeDir(); err == nil {
		data = filepath.Join(home, ".local", "share", "yutemal")
	}

	// Create directories if they don't exist
	os.MkdirAll(config, 0755)
	os.MkdirAll(cache, 0755)
	os.MkdirAll(data, 0755)

	return
}

func initLogging(logFile string, debugMode bool) error {
	// Determine log level based on debug mode
	logLevel := logger.INFO
	if debugMode {
		logLevel = logger.DEBUG
	}

	// Initialize the global logger
	if err := logger.InitLogger(logFile, logLevel, debugMode); err != nil {
		return err
	}

	logger.Info("Logger initialized with debug mode: %v", debugMode)
	if debugMode {
		logger.Debug("Debug logging is enabled")
	}

	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func checkYtDlp() error {
	// Try to find yt-dlp in PATH
	path, err := exec.LookPath("yt-dlp")
	if err != nil {
		return fmt.Errorf("yt-dlp not found in PATH")
	}

	// Verify it's executable
	cmd := exec.Command(path, "--version")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to run yt-dlp: %w", err)
	}

	version := strings.TrimSpace(string(output))
	logger.Info("Found yt-dlp version: %s", version)
	logger.Debug("yt-dlp path: %s", path)
	return nil
}

func checkFfprobe() error {
	// Try to find ffprobe in PATH
	path, err := exec.LookPath("ffprobe")
	if err != nil {
		return fmt.Errorf("ffprobe not found in PATH")
	}

	// Verify it's executable
	cmd := exec.Command(path, "-version")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to run ffprobe: %w", err)
	}

	// Extract version info from first line
	lines := strings.Split(string(output), "\n")
	if len(lines) > 0 {
		version := strings.TrimSpace(lines[0])
		logger.Info("Found ffprobe: %s", version)
		logger.Debug("ffprobe path: %s", path)
	}
	return nil
}

// Helper functions for main

func showYtDlpError() {
	fmt.Println(banner)
	fmt.Println("\n❌ yt-dlp is not installed!")
	fmt.Println("\nyt-dlp is required to download music from YouTube.")
	fmt.Println("\nInstallation instructions:")
	fmt.Println("  macOS:    brew install yt-dlp")
	fmt.Println("  Linux:    sudo apt install yt-dlp  # or use pip")
	fmt.Println("  Windows:  winget install yt-dlp")
	fmt.Println("  Python:   pip install yt-dlp")
	fmt.Println("\nFor more information, visit: https://github.com/yt-dlp/yt-dlp")
}

func showFfprobeError() {
	fmt.Println(banner)
	fmt.Println("\n❌ ffprobe is not installed!")
	fmt.Println("\nffprobe (part of FFmpeg) is required for audio file analysis.")
	fmt.Println("\nInstallation instructions:")
	fmt.Println("  macOS:    brew install ffmpeg")
	fmt.Println("  Linux:    sudo apt install ffmpeg")
	fmt.Println("  Windows:  winget install ffmpeg")
	fmt.Println("\nFor more information, visit: https://ffmpeg.org/download.html")
}

func loadConfiguration(configPath string) *structures.Config {
	cfg, err := config.Load(configPath)
	if err != nil {
		logger.Warn("Failed to load config, using defaults: %v", err)
		cfg = config.Default()

		// Save default config for future use
		if err := config.Save(cfg, configPath); err != nil {
			logger.Warn("Failed to save default config: %v", err)
		} else {
			logger.Info("Created default config at: %s", configPath)
		}
	} else {
		logger.Debug("Configuration loaded successfully from: %s", configPath)
	}
	return cfg
}

func initializeDatabase(dataDir string) database.DB {
	db, err := database.OpenSQLite(filepath.Join(dataDir, "yutemal.db"))
	if err != nil {
		logger.Fatal("Failed to open SQLite database: %v", err)
	}
	logger.Debug("SQLite database opened successfully")
	return db
}

func findHeaderFile(configDir string) string {
	headerFile := filepath.Join(configDir, "headers.txt")
	if fileExists(headerFile) {
		return headerFile
	}

	// Try alternative name
	altHeaderFile := "header.txt"
	if fileExists(altHeaderFile) {
		return altHeaderFile
	}

	return ""
}

func showAuthenticationError(configDir string) {
	fmt.Println(banner)
	fmt.Println("\nNo authentication found!")
	fmt.Println("Please create a header.txt or headers.txt file with your YouTube Music cookies.")
	fmt.Printf("Locations: %s/headers.txt or ./header.txt\n", configDir)
	fmt.Println("\nSee README for instructions on obtaining cookies.")
}

func initializeSystems(cfg *structures.Config, db database.DB, cacheDir, headerFile string) *systems.Systems {
	logger.Debug("Initializing application systems...")
	appSystems := systems.New(cfg, db, cacheDir)

	// Initialize API client with header file
	logger.Debug("Initializing YouTube API with header file: %s", headerFile)
	if err := appSystems.API.InitializeFromHeaderFile(headerFile); err != nil {
		logger.Warn("Failed to initialize YouTube API: %v", err)
		fmt.Printf("Warning: YouTube API not available. Some features will be limited.\n")
	} else {
		logger.Debug("YouTube API initialized successfully")
	}

	// Set header file for download system (for cookie authentication)
	logger.Debug("Setting header file for download system")
	if err := appSystems.Download.SetHeaderFile(headerFile); err != nil {
		logger.Warn("Failed to set header file for downloads: %v", err)
		fmt.Printf("Warning: Downloads may fail without proper authentication.\n")
	} else {
		logger.Debug("Header file set successfully for downloads")
	}

	// Start all systems
	logger.Debug("Starting all application systems...")
	if err := appSystems.Start(); err != nil {
		logger.Fatal("Failed to start systems: %v", err)
	}
	logger.Info("All systems started successfully")

	return appSystems
}
