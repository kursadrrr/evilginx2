package core

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSessionManager(t *testing.T) {
	sm := NewSessionManager()
	defer sm.Stop()

	// Test session creation and retrieval
	s1, err := NewSession("test1")
	assert.NoError(t, err)
	sm.Add(s1)

	s2 := sm.Get(s1.Id)
	assert.Equal(t, s1, s2)

	// Test session expiration
	s3, err := NewSession("test2")
	assert.NoError(t, err)
	s3.ExpiresAt = time.Now().UTC().Add(-1 * time.Hour) // Expired 1 hour ago
	sm.Add(s3)

	time.Sleep(defaultCleanupInterval + time.Second)
	s4 := sm.Get(s3.Id)
	assert.Nil(t, s4, "Session should have been cleaned up")

	// Test concurrent access
	s5, err := NewSession("test3")
	assert.NoError(t, err)
	sm.Add(s5)

	done := make(chan bool)
	go func() {
		s5.SetUsername("user1")
		done <- true
	}()
	go func() {
		s5.SetPassword("pass1")
		done <- true
	}()

	<-done
	<-done

	s6 := sm.Get(s5.Id)
	assert.Equal(t, "user1", s6.Username)
	assert.Equal(t, "pass1", s6.Password)
}

func TestSession(t *testing.T) {
	s, err := NewSession("test")
	assert.NoError(t, err)

	// Test basic operations
	s.SetUsername("testuser")
	s.SetPassword("testpass")
	s.SetCustom("key1", "value1")

	assert.Equal(t, "testuser", s.Username)
	assert.Equal(t, "testpass", s.Password)
	assert.Equal(t, "value1", s.Custom["key1"])

	// Test cookie token operations
	s.AddCookieAuthToken("example.com", "session", "abc123", "/", true, time.Now().Add(time.Hour))

	tokens := make(map[string][]*CookieAuthToken)
	tokens["example.com"] = []*CookieAuthToken{
		{name: "session", optional: false},
	}

	assert.True(t, s.AllCookieAuthTokensCaptured(tokens))

	// Test session finish
	s.Finish(true)
	assert.True(t, s.IsDone)
	assert.True(t, s.IsAuthUrl)
}
