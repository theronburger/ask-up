// Package store persists consultations as small JSON files under the ask-up home
// directory. A consultation is a handle onto a Claude Code session: ask-up keeps
// a friendly id, a label, and the underlying session id so it can be resumed.
// Conversation history lives in Claude Code (via --resume), not here.
package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Consultation is a resumable advisor thread.
type Consultation struct {
	ID        string    `json:"id"`         // friendly handle, e.g. cns_1a2b3c4d
	Label     string    `json:"label"`      // first line of the opening question
	Model     string    `json:"model"`      // model the advisor ran as
	SessionID string    `json:"session_id"` // Claude Code session id to --resume
	CreatedAt time.Time `json:"created_at"`
	LastUsed  time.Time `json:"last_used"`
}

// Store is a directory of consultation files.
type Store struct {
	dir string
}

// New ensures the consultations directory exists under home and returns a Store.
func New(home string) (*Store, error) {
	dir := filepath.Join(home, "consultations")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating %s: %w", dir, err)
	}
	return &Store{dir: dir}, nil
}

// NewID returns a fresh consultation id like "cns_1a2b3c4d".
func NewID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "cns_" + hex.EncodeToString([]byte(time.Now().Format("150405")))[:8]
	}
	return "cns_" + hex.EncodeToString(b)
}

// Label derives a short, single-line label from the opening question.
func Label(question string) string {
	s := strings.Join(strings.Fields(question), " ")
	const maxLen = 60
	if len(s) > maxLen {
		return s[:maxLen-1] + "…"
	}
	return s
}

func (s *Store) path(id string) string {
	return filepath.Join(s.dir, id+".json")
}

// Load reads a consultation by id.
func (s *Store) Load(id string) (*Consultation, error) {
	data, err := os.ReadFile(s.path(id))
	if err != nil {
		return nil, err
	}
	var c Consultation
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", id, err)
	}
	return &c, nil
}

// Save writes a consultation atomically (write-temp-then-rename).
func (s *Store) Save(c *Consultation) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path(c.ID) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path(c.ID))
}

// List returns all consultations, most recently used first.
func (s *Store) List() ([]*Consultation, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	var out []*Consultation
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		c, err := s.Load(strings.TrimSuffix(e.Name(), ".json"))
		if err != nil {
			continue
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastUsed.After(out[j].LastUsed) })
	return out, nil
}
