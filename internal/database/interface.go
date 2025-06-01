package database

import "github.com/haryoiro/yutemal/internal/structures"

// DB is the interface that both Database and SQLiteDatabase implement
type DB interface {
	Add(entry structures.DatabaseEntry) error
	Remove(trackID string) error
	Get(trackID string) (*structures.DatabaseEntry, bool)
	GetAll() []structures.DatabaseEntry
	Close() error

	// Cache methods
	GetCache(cacheKey string) (string, bool)
	SetCache(cacheKey, cacheType, responseData string, ttlSeconds int) error
	InvalidateCache(cacheKey string) error
	InvalidateCacheByType(cacheType string) error
	CleanExpiredCache() error
}
