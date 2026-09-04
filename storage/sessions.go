// Package storage provides persistent session/fixture storage for recorded
// requests, allowing developers to save and reload interactions.
package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"routa/recorder"
)

// SessionInfo describes a saved session.
type SessionInfo struct {
	Name       string    `json:"name"`
	EntryCount int       `json:"entry_count"`
	CreatedAt  time.Time `json:"created_at"`
	SizeBytes  int64     `json:"size_bytes"`
}

// Session is the on-disk format for a saved session.
type Session struct {
	Name      string            `json:"name"`
	CreatedAt time.Time         `json:"created_at"`
	Entries   []*recorder.Entry `json:"entries"`
}

// Store manages session persistence to the filesystem.
type Store struct {
	dir string
}

// NewStore creates a Store that saves sessions to the given directory.
func NewStore(dir string) *Store {
	return &Store{dir: dir}
}

// SaveSession persists the given entries as a named session.
func (s *Store) SaveSession(name string, entries []*recorder.Entry) error {
	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return fmt.Errorf("create sessions dir: %w", err)
	}

	session := Session{
		Name:      name,
		CreatedAt: time.Now(),
		Entries:   entries,
	}

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	path := filepath.Join(s.dir, sanitizeName(name)+".json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write session file: %w", err)
	}
	return nil
}

// LoadSession reads a named session from disk.
func (s *Store) LoadSession(name string) (*Session, error) {
	path := filepath.Join(s.dir, sanitizeName(name)+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read session file: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}
	return &session, nil
}

// ListSessions returns info about all saved sessions.
func (s *Store) ListSessions() ([]SessionInfo, error) {
	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("read sessions dir: %w", err)
	}

	var sessions []SessionInfo
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}

		info, err := e.Info()
		if err != nil {
			continue
		}

		// Quick-read just the metadata.
		path := filepath.Join(s.dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var session Session
		if err := json.Unmarshal(data, &session); err != nil {
			continue
		}

		sessions = append(sessions, SessionInfo{
			Name:       session.Name,
			EntryCount: len(session.Entries),
			CreatedAt:  session.CreatedAt,
			SizeBytes:  info.Size(),
		})
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].CreatedAt.After(sessions[j].CreatedAt)
	})

	return sessions, nil
}

// DeleteSession removes a saved session.
func (s *Store) DeleteSession(name string) error {
	path := filepath.Join(s.dir, sanitizeName(name)+".json")
	return os.Remove(path)
}

// sanitizeName replaces unsafe characters in a session name for use as a filename.
func sanitizeName(name string) string {
	result := make([]byte, 0, len(name))
	for _, c := range []byte(name) {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '_' {
			result = append(result, c)
		} else {
			result = append(result, '_')
		}
	}
	if len(result) == 0 {
		return "unnamed"
	}
	return string(result)
}
