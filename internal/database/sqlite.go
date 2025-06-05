package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/haryoiro/yutemal/internal/structures"
	_ "github.com/mattn/go-sqlite3"
)

// SQLiteDatabase represents the SQLite-based music database
type SQLiteDatabase struct {
	mu   sync.RWMutex
	db   *sql.DB
	path string
}

// OpenSQLite opens or creates a SQLite database
func OpenSQLite(path string) (*SQLiteDatabase, error) {
	// Ensure the directory exists with proper permissions
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Set directory permissions explicitly
	if err := os.Chmod(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to set directory permissions: %w", err)
	}

	// Create the file if it doesn't exist with proper permissions
	if _, err := os.Stat(path); os.IsNotExist(err) {
		file, err := os.Create(path)
		if err != nil {
			return nil, fmt.Errorf("failed to create database file: %w", err)
		}
		file.Close()

		// Set file permissions explicitly
		if err := os.Chmod(path, 0644); err != nil {
			return nil, fmt.Errorf("failed to set file permissions: %w", err)
		}
	}

	// Open without WAL mode initially to avoid journal file issues
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test basic connectivity first
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Set pragmas one by one with error handling
	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA cache_size = 10000",
		"PRAGMA temp_store = MEMORY",
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to set pragma '%s': %w", pragma, err)
		}
	}

	sqliteDB := &SQLiteDatabase{
		db:   db,
		path: path,
	}

	if err := sqliteDB.createTables(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return sqliteDB, nil
}

// createTables creates the necessary database tables
func (db *SQLiteDatabase) createTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS tracks (
			track_id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			artists TEXT NOT NULL, -- JSON array
			album TEXT,
			thumbnail TEXT,
			duration INTEGER NOT NULL,
			is_available INTEGER NOT NULL DEFAULT 1,
			is_explicit INTEGER NOT NULL DEFAULT 0,
			added_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			file_path TEXT,
			file_size INTEGER DEFAULT 0,
			thumbnail_path TEXT,
			play_count INTEGER DEFAULT 0,
			last_played DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_tracks_title ON tracks(title)`,
		`CREATE INDEX IF NOT EXISTS idx_tracks_added_at ON tracks(added_at)`,
		`CREATE INDEX IF NOT EXISTS idx_tracks_play_count ON tracks(play_count)`,

		`CREATE TABLE IF NOT EXISTS playlists (
			playlist_id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT,
			thumbnail TEXT,
			is_local INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS playlist_tracks (
			playlist_id TEXT NOT NULL,
			track_id TEXT NOT NULL,
			position INTEGER NOT NULL,
			added_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (playlist_id, track_id),
			FOREIGN KEY (playlist_id) REFERENCES playlists(playlist_id) ON DELETE CASCADE,
			FOREIGN KEY (track_id) REFERENCES tracks(track_id) ON DELETE CASCADE
		)`,

		`CREATE TABLE IF NOT EXISTS listening_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			track_id TEXT NOT NULL,
			played_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			duration_played INTEGER NOT NULL DEFAULT 0,
			FOREIGN KEY (track_id) REFERENCES tracks(track_id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_history_played_at ON listening_history(played_at)`,

		`CREATE TABLE IF NOT EXISTS app_state (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,

		// API cache table
		`CREATE TABLE IF NOT EXISTS api_cache (
			cache_key TEXT PRIMARY KEY,
			cache_type TEXT NOT NULL,
			response_data TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME NOT NULL,
			etag TEXT,
			request_params TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_cache_type ON api_cache(cache_type)`,
		`CREATE INDEX IF NOT EXISTS idx_cache_expires ON api_cache(expires_at)`,

		// Trigger to update updated_at timestamp
		`CREATE TRIGGER IF NOT EXISTS update_tracks_timestamp
		AFTER UPDATE ON tracks
		BEGIN
			UPDATE tracks SET updated_at = CURRENT_TIMESTAMP WHERE track_id = NEW.track_id;
		END`,

		`CREATE TRIGGER IF NOT EXISTS update_playlists_timestamp
		AFTER UPDATE ON playlists
		BEGIN
			UPDATE playlists SET updated_at = CURRENT_TIMESTAMP WHERE playlist_id = NEW.playlist_id;
		END`,
	}

	for _, query := range queries {
		if _, err := db.db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query: %w", err)
		}
	}

	// Run migrations for existing databases
	if err := db.runMigrations(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// runMigrations applies schema updates to existing databases
func (db *SQLiteDatabase) runMigrations() error {
	// Check if playlists table has sync columns
	var columnExists bool
	err := db.db.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('playlists')
		WHERE name = 'last_synced'
	`).Scan(&columnExists)

	if err != nil {
		return fmt.Errorf("failed to check column existence: %w", err)
	}

	// Add sync columns if they don't exist
	if !columnExists {
		migrations := []string{
			`ALTER TABLE playlists ADD COLUMN last_synced DATETIME`,
			`ALTER TABLE playlists ADD COLUMN sync_etag TEXT`,
		}

		for _, migration := range migrations {
			if _, err := db.db.Exec(migration); err != nil {
				// Ignore error if column already exists
				// SQLite doesn't support IF NOT EXISTS for ALTER TABLE
			}
		}
	}

	// Check if tracks table has thumbnail_path column
	var thumbnailPathExists bool
	err = db.db.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('tracks')
		WHERE name = 'thumbnail_path'
	`).Scan(&thumbnailPathExists)

	if err != nil {
		return fmt.Errorf("failed to check thumbnail_path column existence: %w", err)
	}

	// Add thumbnail_path column if it doesn't exist
	if !thumbnailPathExists {
		if _, err := db.db.Exec(`ALTER TABLE tracks ADD COLUMN thumbnail_path TEXT`); err != nil {
			// Ignore error if column already exists
		}
	}

	return nil
}

// Close closes the database
func (db *SQLiteDatabase) Close() error {
	return db.db.Close()
}

// Add adds a new track to the database
func (db *SQLiteDatabase) Add(entry structures.DatabaseEntry) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	artistsJSON, err := json.Marshal(entry.Track.Artists)
	if err != nil {
		return fmt.Errorf("failed to marshal artists: %w", err)
	}

	query := `
		INSERT OR REPLACE INTO tracks
		(track_id, title, artists, thumbnail, duration, is_available, is_explicit,
		 added_at, file_path, file_size)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = db.db.Exec(query,
		entry.Track.TrackID,
		entry.Track.Title,
		string(artistsJSON),
		entry.Track.Thumbnail,
		entry.Track.Duration,
		entry.Track.IsAvailable,
		entry.Track.IsExplicit,
		entry.AddedAt,
		entry.FilePath,
		entry.FileSize,
	)

	return err
}

// Remove removes a track from the database
func (db *SQLiteDatabase) Remove(trackID string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	_, err := db.db.Exec("DELETE FROM tracks WHERE track_id = ?", trackID)
	return err
}

// Get retrieves a track by ID
func (db *SQLiteDatabase) Get(trackID string) (*structures.DatabaseEntry, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	query := `
		SELECT track_id, title, artists, thumbnail, duration, is_available,
		       is_explicit, added_at, file_path, file_size
		FROM tracks
		WHERE track_id = ?
	`

	row := db.db.QueryRow(query, trackID)

	var entry structures.DatabaseEntry
	var artistsJSON string
	var thumbnail, filePath sql.NullString
	var fileSize sql.NullInt64

	err := row.Scan(
		&entry.Track.TrackID,
		&entry.Track.Title,
		&artistsJSON,
		&thumbnail,
		&entry.Track.Duration,
		&entry.Track.IsAvailable,
		&entry.Track.IsExplicit,
		&entry.AddedAt,
		&filePath,
		&fileSize,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false
		}
		return nil, false
	}

	// Parse artists JSON
	if err := json.Unmarshal([]byte(artistsJSON), &entry.Track.Artists); err != nil {
		return nil, false
	}

	// Handle nullable fields
	entry.Track.Thumbnail = thumbnail.String
	entry.FilePath = filePath.String
	entry.FileSize = fileSize.Int64
	return &entry, true
}

// GetAll returns all tracks
func (db *SQLiteDatabase) GetAll() []structures.DatabaseEntry {
	db.mu.RLock()
	defer db.mu.RUnlock()

	query := `
		SELECT track_id, title, artists, thumbnail, duration, is_available,
		       is_explicit, added_at, file_path, file_size
		FROM tracks
		ORDER BY added_at DESC
	`

	rows, err := db.db.Query(query)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var entries []structures.DatabaseEntry

	for rows.Next() {
		var entry structures.DatabaseEntry
		var artistsJSON string
		var thumbnail, filePath sql.NullString
		var fileSize sql.NullInt64

		err := rows.Scan(
			&entry.Track.TrackID,
			&entry.Track.Title,
			&artistsJSON,
			&thumbnail,
			&entry.Track.Duration,
			&entry.Track.IsAvailable,
			&entry.Track.IsExplicit,
			&entry.AddedAt,
			&filePath,
			&fileSize,
		)

		if err != nil {
			continue
		}

		// Parse artists JSON
		if err := json.Unmarshal([]byte(artistsJSON), &entry.Track.Artists); err != nil {
			continue
		}

		// Handle nullable fields
		entry.Track.Thumbnail = thumbnail.String
		entry.FilePath = filePath.String
		entry.FileSize = fileSize.Int64

		entries = append(entries, entry)
	}

	return entries
}

// GetCache retrieves cached data by key
func (db *SQLiteDatabase) GetCache(cacheKey string) (string, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var responseData string
	err := db.db.QueryRow(`
		SELECT response_data FROM api_cache
		WHERE cache_key = ? AND expires_at > CURRENT_TIMESTAMP
	`, cacheKey).Scan(&responseData)

	if err != nil {
		return "", false
	}

	return responseData, true
}

// SetCache stores data in the cache
func (db *SQLiteDatabase) SetCache(cacheKey, cacheType, responseData string, ttlSeconds int) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	query := `
		INSERT OR REPLACE INTO api_cache
		(cache_key, cache_type, response_data, created_at, expires_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP, datetime('now', '+' || ? || ' seconds'))
	`

	_, err := db.db.Exec(query, cacheKey, cacheType, responseData, ttlSeconds)
	return err
}

// InvalidateCache removes a specific cache entry
func (db *SQLiteDatabase) InvalidateCache(cacheKey string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	_, err := db.db.Exec("DELETE FROM api_cache WHERE cache_key = ?", cacheKey)
	return err
}

// InvalidateCacheByType removes all cache entries of a specific type
func (db *SQLiteDatabase) InvalidateCacheByType(cacheType string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	_, err := db.db.Exec("DELETE FROM api_cache WHERE cache_type = ?", cacheType)
	return err
}

// CleanExpiredCache removes expired cache entries
func (db *SQLiteDatabase) CleanExpiredCache() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	_, err := db.db.Exec("DELETE FROM api_cache WHERE expires_at <= CURRENT_TIMESTAMP")
	return err
}
