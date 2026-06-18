// Package store persists consultations as JSON files under the ask-up home
// directory. Each consultation is one Opus thread that the calling agent can
// continue, with enough metadata to judge cache warmth before reviving it.
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

// Message is one turn in a consultation.
type Message struct {
	Role string `json:"role"` // "user" or "assistant"
	Text string `json:"text"`
}

// Consultation is a persisted Opus thread.
type Consultation struct {
	ID           string    `json:"id"`
	Label        string    `json:"label"`
	Model        string    `json:"model"`
	CreatedAt    time.Time `json:"created_at"`
	LastUsed     time.Time `json:"last_used"`
	PrefixTokens int64     `json:"prefix_tokens"` // total prompt tokens of the last request
	Messages     []Message `json:"messages"`
}

// Warmth is the cache assessment of a consultation at a point in time.
type Warmth struct {
	Cacheable bool          // prefix reached the model's cache floor
	Warm      bool          // within the warmth window (only meaningful if Cacheable)
	Age       time.Duration // time since last use
}

// Assess reports whether reviving this consultation would hit a warm cache.
// Below the cache floor there is nothing cached, so Warm is reported false but
// callers should treat reuse as free (see Store callers / the warmth guard).
func (c *Consultation) Assess(now time.Time, floor int64, window time.Duration) Warmth {
	age := now.Sub(c.LastUsed)
	cacheable := c.PrefixTokens >= floor
	return Warmth{Cacheable: cacheable, Warm: cacheable && age <= window, Age: age}
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
		// rand.Read only fails on a broken platform RNG; fall back to time.
		return "cns_" + hex.EncodeToString([]byte(time.Now().Format("150405")))[:8]
	}
	return "cns_" + hex.EncodeToString(b)
}

// Label derives a short, single-line label from the first question.
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
			continue // skip unreadable/partial files rather than fail the whole list
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastUsed.After(out[j].LastUsed) })
	return out, nil
}
