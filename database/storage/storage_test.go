package storage

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tidwall/buntdb"
)

func TestRedisStorage(t *testing.T) {
	opts := &RedisOptions{
		Addr: "localhost:6379",
		TTL:  5 * time.Second,
	}

	storage, err := NewRedisStorage(opts)
	if err != nil {
		t.Fatalf("Failed to create Redis storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Test session creation
	sid := "test-session"
	err = storage.CreateSession(ctx, sid, "test-phishlet", "http://test.com", "test-agent", "127.0.0.1")
	assert.NoError(t, err)

	// Test session retrieval
	session, err := storage.GetSession(ctx, sid)
	assert.NoError(t, err)
	assert.Equal(t, sid, session.SessionId)
	assert.Equal(t, "test-phishlet", session.Phishlet)

	// Test session updates
	err = storage.UpdateUsername(ctx, sid, "testuser")
	assert.NoError(t, err)
	session, err = storage.GetSession(ctx, sid)
	assert.NoError(t, err)
	assert.Equal(t, "testuser", session.Username)

	// Test session listing
	sessions, err := storage.ListSessions(ctx)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(sessions), 1)

	// Test session deletion
	err = storage.DeleteSession(ctx, sid)
	assert.NoError(t, err)
	_, err = storage.GetSession(ctx, sid)
	assert.Error(t, err)
}

func TestMigration(t *testing.T) {
	// Create temporary BuntDB file
	tmpfile, err := os.CreateTemp("", "buntdb")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	// Initialize BuntDB with test data
	db, err := buntdb.Open(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to open BuntDB: %v", err)
	}

	testSession := &Session{
		Id:         1,
		SessionId:  "test-sid",
		Phishlet:   "test-phishlet",
		LandingURL: "http://test.com",
		CreateTime: time.Now().Unix(),
		UpdateTime: time.Now().Unix(),
	}

	sessionData, _ := json.Marshal(testSession)
	err = db.Update(func(tx *buntdb.Tx) error {
		_, _, err := tx.Set("sessions:1", string(sessionData), nil)
		return err
	})
	assert.NoError(t, err)
	db.Close()

	// Setup migration options
	opts := &MigrationOptions{
		BuntDBPath: tmpfile.Name(),
		Redis: &RedisOptions{
			Addr: "localhost:6379",
			TTL:  24 * time.Hour,
		},
	}

	// Run migration
	err = MigrateToRedis(context.Background(), opts)
	assert.NoError(t, err)

	// Verify Redis storage after migration
	redis, err := NewRedisStorage(opts.Redis)
	assert.NoError(t, err)
	defer redis.Close()

	// List sessions and verify they were migrated correctly
	sessions, err := redis.ListSessions(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 1, len(sessions), "Expected 1 session to be migrated")
	assert.Equal(t, "test-sid", sessions[0].SessionId)
}
