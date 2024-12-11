package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/tidwall/buntdb"
)

// MigrationOptions contains configuration for the migration process
type MigrationOptions struct {
	BuntDBPath string
	Redis      *RedisOptions
}

// MigrateToRedis migrates data from BuntDB to Redis
func MigrateToRedis(ctx context.Context, opts *MigrationOptions) error {
	// Open BuntDB
	buntDB, err := buntdb.Open(opts.BuntDBPath)
	if err != nil {
		return fmt.Errorf("failed to open BuntDB: %v", err)
	}
	defer buntDB.Close()

	// Create Redis storage
	redisStorage, err := NewRedisStorage(opts.Redis)
	if err != nil {
		return fmt.Errorf("failed to create Redis storage: %v", err)
	}
	defer redisStorage.Close()

	// Create indexes if they don't exist
	err = buntDB.CreateIndex("sessions_id", "sessions:*", buntdb.IndexJSON("id"))
	if err != nil && err != buntdb.ErrIndexExists {
		return fmt.Errorf("failed to create index: %v", err)
	}

	migratedCount := 0
	// Migrate sessions
	err = buntDB.View(func(tx *buntdb.Tx) error {
		err := tx.Ascend("sessions_id", func(key, value string) bool {
			var session Session
			if err := json.Unmarshal([]byte(value), &session); err != nil {
				fmt.Printf("Warning: Error unmarshaling session: %v\n", err)
				return true
			}

			// Add new fields for Redis
			session.ExpiresAt = time.Now().Add(24 * time.Hour)
			session.LastAccessed = time.Now()

			// Save to Redis
			if err := redisStorage.saveSession(ctx, &session); err != nil {
				fmt.Printf("Warning: Error migrating session %s: %v\n", session.SessionId, err)
				return true
			}
			migratedCount++
			return true
		})
		return err
	})

	if err != nil {
		return fmt.Errorf("migration failed: %v", err)
	}

	// Success even if no sessions were migrated
	return nil
}
