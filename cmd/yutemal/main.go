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
	"github.com/haryoiro/yutemal/internal/systems"
	"github.com/haryoiro/yutemal/internal/ui"
)

const (
	version = "0.1.0"
	banner  = `
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
		showHelp     = flag.Bool("help", false, "Show help message")
		showFiles    = flag.Bool("files", false, "Show file locations")
		fixDB        = flag.Bool("fix-db", false, "Fix database issues")
		clearCache   = flag.Bool("clear-cache", false, "Clear cache directory")
		showVersion  = flag.Bool("version", false, "Show version")
		debugMode    = flag.Bool("debug", false, "Enable debug logging")
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
		fmt.Println("    Ctrl+D      - Quit application")
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
		fmt.Printf("yutemal (Go) v%s\n", version)
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
		fmt.Println("Clearing cache...")
		// Initialize minimal logging for cache clearing operation
		_, _, dataDir := getDirectories()
		logFile := filepath.Join(dataDir, "yutemal.log")
		if err := logger.InitLogger(logFile, logger.INFO, false); err != nil {
			fmt.Printf("Warning: Failed to initialize logger: %v\n", err)
		}
		defer logger.CloseLogger()

		// Only clear the downloads directory, not the entire cache
		downloadsDir := filepath.Join(cacheDir, "downloads")
		if err := os.RemoveAll(downloadsDir); err != nil {
			logger.Fatal("Failed to clear downloads cache: %v", err)
		}
		// Also clear the SQLite database
		dbPath := filepath.Join(dataDir, "yutemal.db")
		if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
			logger.Warn("Failed to remove database: %v", err)
		}
		fmt.Println("Cache cleared successfully")
		return
	}

	// Check if yt-dlp is installed
	if err := checkYtDlp(); err != nil {
		fmt.Println(banner)
		fmt.Println("\n❌ yt-dlp is not installed!")
		fmt.Println("\nyt-dlp is required to download music from YouTube.")
		fmt.Println("\nInstallation instructions:")
		fmt.Println("  macOS:    brew install yt-dlp")
		fmt.Println("  Linux:    sudo apt install yt-dlp  # or use pip")
		fmt.Println("  Windows:  winget install yt-dlp")
		fmt.Println("  Python:   pip install yt-dlp")
		fmt.Println("\nFor more information, visit: https://github.com/yt-dlp/yt-dlp")
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

	// Initialize SQLite database
	db, err := database.OpenSQLite(filepath.Join(dataDir, "yutemal.db"))
	if err != nil {
		logger.Fatal("Failed to open SQLite database: %v", err)
	}
	defer func() {
		logger.Debug("Closing database connection")
		db.Close()
	}()
	logger.Debug("SQLite database opened successfully")

	// Check for authentication - try both header.txt and headers.txt
	headerFile := filepath.Join(configDir, "headers.txt")
	if !fileExists(headerFile) {
		// Try alternative name
		altHeaderFile := "header.txt"
		if fileExists(altHeaderFile) {
			headerFile = altHeaderFile
		} else {
			fmt.Println(banner)
			fmt.Println("\nNo authentication found!")
			fmt.Println("Please create a header.txt or headers.txt file with your YouTube Music cookies.")
			fmt.Printf("Locations: %s or %s\n", headerFile, altHeaderFile)
			fmt.Println("\nSee README for instructions on obtaining cookies.")
			return
		}
	}

	// Initialize systems
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
	defer func() {
		logger.Debug("Stopping all application systems...")
		appSystems.Stop()
	}()
	logger.Info("All systems started successfully")

	// Start the application
	fmt.Println(banner)
	fmt.Println("\nStarting yutemal...")
	logger.Info("yutemal application starting...")

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
