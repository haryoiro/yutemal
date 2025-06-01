package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/haryoiro/yutemal/internal/structures"
	_ "github.com/mattn/go-sqlite3"
)

// SQLiteDatabase represents the SQLite-based music database
type SQLiteDatabase struct {
	mu       sync.RWMutex
	db       *sql.DB
	path     string
}

// OpenSQLite opens or creates a SQLite database
func OpenSQLite(path string) (*SQLiteDatabase, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable foreign keys and set pragmas for performance
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
			return nil, fmt.Errorf("failed to set pragma: %w", err)
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
		(track_id, title, artists, album, thumbnail, duration, is_available, is_explicit,
		 added_at, file_path, file_size)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = db.db.Exec(query,
		entry.Track.TrackID,
		entry.Track.Title,
		string(artistsJSON),
		entry.Track.Album,
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
		SELECT track_id, title, artists, album, thumbnail, duration, is_available,
		       is_explicit, added_at, file_path, file_size
		FROM tracks
		WHERE track_id = ?
	`

	row := db.db.QueryRow(query, trackID)

	var entry structures.DatabaseEntry
	var artistsJSON string
	var album, thumbnail, filePath sql.NullString
	var fileSize sql.NullInt64

	err := row.Scan(
		&entry.Track.TrackID,
		&entry.Track.Title,
		&artistsJSON,
		&album,
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
	entry.Track.Album = album.String
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
		SELECT track_id, title, artists, album, thumbnail, duration, is_available,
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
		var album, thumbnail, filePath sql.NullString
		var fileSize sql.NullInt64

		err := rows.Scan(
			&entry.Track.TrackID,
			&entry.Track.Title,
			&artistsJSON,
			&album,
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
		entry.Track.Album = album.String
		entry.Track.Thumbnail = thumbnail.String
		entry.FilePath = filePath.String
		entry.FileSize = fileSize.Int64

		entries = append(entries, entry)
	}

	return entries
}

// UpdatePlayStats updates play statistics for a track
func (db *SQLiteDatabase) UpdatePlayStats(trackID string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	tx, err := db.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Update play count and last played
	_, err = tx.Exec(`
		UPDATE tracks
		SET play_count = play_count + 1,
		    last_played = CURRENT_TIMESTAMP
		WHERE track_id = ?
	`, trackID)
	if err != nil {
		return err
	}

	// Add to listening history
	_, err = tx.Exec(`
		INSERT INTO listening_history (track_id)
		VALUES (?)
	`, trackID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// GetMostPlayed returns the most played tracks
func (db *SQLiteDatabase) GetMostPlayed(limit int) []structures.DatabaseEntry {
	db.mu.RLock()
	defer db.mu.RUnlock()

	query := `
		SELECT track_id, title, artists, album, thumbnail, duration, is_available,
		       is_explicit, added_at, file_path, file_size
		FROM tracks
		WHERE play_count > 0
		ORDER BY play_count DESC, last_played DESC
		LIMIT ?
	`

	rows, err := db.db.Query(query, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var entries []structures.DatabaseEntry

	for rows.Next() {
		var entry structures.DatabaseEntry
		var artistsJSON string
		var album, thumbnail, filePath sql.NullString
		var fileSize sql.NullInt64

		err := rows.Scan(
			&entry.Track.TrackID,
			&entry.Track.Title,
			&artistsJSON,
			&album,
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
		entry.Track.Album = album.String
		entry.Track.Thumbnail = thumbnail.String
		entry.FilePath = filePath.String
		entry.FileSize = fileSize.Int64

		entries = append(entries, entry)
	}

	return entries
}

// GetRecentlyPlayed returns recently played tracks
func (db *SQLiteDatabase) GetRecentlyPlayed(limit int) []structures.DatabaseEntry {
	db.mu.RLock()
	defer db.mu.RUnlock()

	query := `
		SELECT DISTINCT t.track_id, t.title, t.artists, t.album, t.thumbnail,
		       t.duration, t.is_available, t.is_explicit, t.added_at,
		       t.file_path, t.file_size
		FROM tracks t
		INNER JOIN listening_history h ON t.track_id = h.track_id
		ORDER BY h.played_at DESC
		LIMIT ?
	`

	rows, err := db.db.Query(query, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var entries []structures.DatabaseEntry

	for rows.Next() {
		var entry structures.DatabaseEntry
		var artistsJSON string
		var album, thumbnail, filePath sql.NullString
		var fileSize sql.NullInt64

		err := rows.Scan(
			&entry.Track.TrackID,
			&entry.Track.Title,
			&artistsJSON,
			&album,
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
		entry.Track.Album = album.String
		entry.Track.Thumbnail = thumbnail.String
		entry.FilePath = filePath.String
		entry.FileSize = fileSize.Int64

		entries = append(entries, entry)
	}

	return entries
}

// SaveAppState saves application state
func (db *SQLiteDatabase) SaveAppState(key, value string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	_, err := db.db.Exec(`
		INSERT OR REPLACE INTO app_state (key, value, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
	`, key, value)
	return err
}

// GetAppState retrieves application state
func (db *SQLiteDatabase) GetAppState(key string) (string, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var value string
	err := db.db.QueryRow("SELECT value FROM app_state WHERE key = ?", key).Scan(&value)
	if err != nil {
		return "", false
	}
	return value, true
}

// Search performs a text search on tracks
func (db *SQLiteDatabase) Search(query string) []structures.DatabaseEntry {
	db.mu.RLock()
	defer db.mu.RUnlock()

	searchPattern := "%" + query + "%"
	sqlQuery := `
		SELECT track_id, title, artists, album, thumbnail, duration, is_available,
		       is_explicit, added_at, file_path, file_size
		FROM tracks
		WHERE title LIKE ? OR artists LIKE ? OR album LIKE ?
		ORDER BY
			CASE
				WHEN title LIKE ? THEN 1
				WHEN artists LIKE ? THEN 2
				ELSE 3
			END,
			play_count DESC
		LIMIT 50
	`

	rows, err := db.db.Query(sqlQuery, searchPattern, searchPattern, searchPattern, searchPattern, searchPattern)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var entries []structures.DatabaseEntry

	for rows.Next() {
		var entry structures.DatabaseEntry
		var artistsJSON string
		var album, thumbnail, filePath sql.NullString
		var fileSize sql.NullInt64

		err := rows.Scan(
			&entry.Track.TrackID,
			&entry.Track.Title,
			&artistsJSON,
			&album,
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
		entry.Track.Album = album.String
		entry.Track.Thumbnail = thumbnail.String
		entry.FilePath = filePath.String
		entry.FileSize = fileSize.Int64

		entries = append(entries, entry)
	}

	return entries
}
