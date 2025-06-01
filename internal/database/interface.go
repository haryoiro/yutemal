package database

import "github.com/haryoiro/yutemal/internal/structures"

// DB is the interface that both Database and SQLiteDatabase implement
type DB interface {
	Add(entry structures.DatabaseEntry) error
	Remove(trackID string) error
	Get(trackID string) (*structures.DatabaseEntry, bool)
	GetAll() []structures.DatabaseEntry
	Close() error
}
