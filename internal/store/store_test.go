package store

import (
	"strings"
	"testing"
	"time"
)

func TestNewIDFormat(t *testing.T) {
	id := NewID()
	if !strings.HasPrefix(id, "cns_") {
		t.Errorf("id %q missing cns_ prefix", id)
	}
	if len(id) != len("cns_")+8 {
		t.Errorf("id %q wrong length", id)
	}
	if a, b := NewID(), NewID(); a == b {
		t.Error("NewID returned duplicate ids")
	}
}

func TestLabel(t *testing.T) {
	if got := Label("  hello   world  "); got != "hello world" {
		t.Errorf("Label collapse = %q", got)
	}
	long := strings.Repeat("a", 100)
	got := Label(long)
	if len([]rune(got)) > 60 {
		t.Errorf("Label not truncated: %d runes", len([]rune(got)))
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("truncated label should end with ellipsis: %q", got)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	st, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	want := &Consultation{
		ID:           "cns_abc12345",
		Label:        "lock ordering",
		Model:        "claude-opus-4-8",
		CreatedAt:    time.Now().Truncate(time.Second),
		LastUsed:     time.Now().Truncate(time.Second),
		PrefixTokens: 5300,
		Messages: []Message{
			{Role: "user", Text: "q1"},
			{Role: "assistant", Text: "a1"},
		},
	}
	if err := st.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := st.Load("cns_abc12345")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != want.ID || got.PrefixTokens != want.PrefixTokens || len(got.Messages) != 2 {
		t.Errorf("round trip mismatch: %+v", got)
	}
}

func TestListOrdering(t *testing.T) {
	st, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	old := &Consultation{ID: "cns_old00000", LastUsed: time.Now().Add(-time.Hour)}
	recent := &Consultation{ID: "cns_new00000", LastUsed: time.Now()}
	if err := st.Save(old); err != nil {
		t.Fatal(err)
	}
	if err := st.Save(recent); err != nil {
		t.Fatal(err)
	}
	list, err := st.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].ID != "cns_new00000" {
		t.Errorf("expected most-recent first, got %v", list)
	}
}

func TestAssess(t *testing.T) {
	now := time.Now()
	window := 290 * time.Second
	const floor = 4096

	tests := []struct {
		name          string
		prefix        int64
		lastUsed      time.Time
		wantCacheable bool
		wantWarm      bool
	}{
		{"large and recent", 5000, now.Add(-1 * time.Minute), true, true},
		{"large and stale", 5000, now.Add(-30 * time.Minute), true, false},
		{"below floor", 1000, now.Add(-1 * time.Second), false, false},
		{"below floor and stale", 1000, now.Add(-time.Hour), false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := &Consultation{PrefixTokens: tc.prefix, LastUsed: tc.lastUsed}
			w := c.Assess(now, floor, window)
			if w.Cacheable != tc.wantCacheable {
				t.Errorf("Cacheable = %v, want %v", w.Cacheable, tc.wantCacheable)
			}
			if w.Warm != tc.wantWarm {
				t.Errorf("Warm = %v, want %v", w.Warm, tc.wantWarm)
			}
		})
	}
}
