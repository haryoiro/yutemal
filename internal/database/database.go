package database

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/haryoiro/yutemal/internal/structures"
)

// Database represents the music database
type Database struct {
	mu       sync.RWMutex
	path     string
	entries  []structures.DatabaseEntry
	index    map[string]int // trackID -> index mapping
	file     *os.File
	modified bool
}

// Open opens or creates a database file
func Open(path string) (*Database, error) {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db := &Database{
		path:  path,
		file:  file,
		index: make(map[string]int),
	}

	if err := db.load(); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to load database: %w", err)
	}

	return db, nil
}

// Close closes the database
func (db *Database) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.modified {
		if err := db.save(); err != nil {
			return err
		}
	}

	return db.file.Close()
}

// Add adds a new entry to the database
func (db *Database) Add(entry structures.DatabaseEntry) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	// Check if already exists
	if _, exists := db.index[entry.Track.TrackID]; exists {
		return nil
	}

	entry.AddedAt = time.Now()
	db.entries = append(db.entries, entry)
	db.index[entry.Track.TrackID] = len(db.entries) - 1
	db.modified = true

	// Append to file immediately
	return db.appendEntry(entry)
}

// Remove removes an entry from the database
func (db *Database) Remove(trackID string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	idx, exists := db.index[trackID]
	if !exists {
		return nil
	}

	// Remove from slice
	db.entries = append(db.entries[:idx], db.entries[idx+1:]...)
	delete(db.index, trackID)

	// Rebuild index
	for i := idx; i < len(db.entries); i++ {
		db.index[db.entries[i].Track.TrackID] = i
	}

	db.modified = true
	return nil
}

// Get retrieves an entry by track ID
func (db *Database) Get(trackID string) (*structures.DatabaseEntry, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	idx, exists := db.index[trackID]
	if !exists {
		return nil, false
	}

	entry := db.entries[idx]
	return &entry, true
}

// GetAll returns all entries
func (db *Database) GetAll() []structures.DatabaseEntry {
	db.mu.RLock()
	defer db.mu.RUnlock()

	result := make([]structures.DatabaseEntry, len(db.entries))
	copy(result, db.entries)
	return result
}

// load loads the database from disk
func (db *Database) load() error {
	// Reset to beginning
	if _, err := db.file.Seek(0, 0); err != nil {
		return err
	}

	for {
		var size uint32
		if err := binary.Read(db.file, binary.LittleEndian, &size); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		data := make([]byte, size)
		if _, err := io.ReadFull(db.file, data); err != nil {
			return err
		}

		var entry structures.DatabaseEntry
		if err := json.Unmarshal(data, &entry); err != nil {
			return err
		}

		db.entries = append(db.entries, entry)
		db.index[entry.Track.TrackID] = len(db.entries) - 1
	}

	return nil
}

// save saves the entire database to disk
func (db *Database) save() error {
	// Create temporary file
	tmpPath := db.path + ".tmp"
	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer tmpFile.Close()

	// Write all entries
	for _, entry := range db.entries {
		data, err := json.Marshal(entry)
		if err != nil {
			return err
		}

		size := uint32(len(data))
		if err := binary.Write(tmpFile, binary.LittleEndian, size); err != nil {
			return err
		}

		if _, err := tmpFile.Write(data); err != nil {
			return err
		}
	}

	// Sync to disk
	if err := tmpFile.Sync(); err != nil {
		return err
	}

	// Close original file
	db.file.Close()

	// Replace with temporary file
	if err := os.Rename(tmpPath, db.path); err != nil {
		return err
	}

	// Reopen file
	db.file, err = os.OpenFile(db.path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}

	db.modified = false
	return nil
}

// appendEntry appends a single entry to the file
func (db *Database) appendEntry(entry structures.DatabaseEntry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	// Seek to end
	if _, err := db.file.Seek(0, 2); err != nil {
		return err
	}

	size := uint32(len(data))
	if err := binary.Write(db.file, binary.LittleEndian, size); err != nil {
		return err
	}

	if _, err := db.file.Write(data); err != nil {
		return err
	}

	return db.file.Sync()
}

// Fix attempts to fix database corruption
func Fix(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	var validEntries []structures.DatabaseEntry

	for {
		var size uint32
		if err := binary.Read(file, binary.LittleEndian, &size); err != nil {
			if err == io.EOF {
				break
			}
			// Skip corrupted entry
			continue
		}

		// Sanity check size
		if size > 1024*1024 { // 1MB max per entry
			continue
		}

		data := make([]byte, size)
		if _, err := io.ReadFull(file, data); err != nil {
			continue
		}

		var entry structures.DatabaseEntry
		if err := json.Unmarshal(data, &entry); err != nil {
			continue
		}

		validEntries = append(validEntries, entry)
	}

	// Write back valid entries
	tmpPath := path + ".tmp"
	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer tmpFile.Close()

	for _, entry := range validEntries {
		data, err := json.Marshal(entry)
		if err != nil {
			continue
		}

		size := uint32(len(data))
		binary.Write(tmpFile, binary.LittleEndian, size)
		tmpFile.Write(data)
	}

	tmpFile.Sync()
	tmpFile.Close()

	return os.Rename(tmpPath, path)
}
