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
	got := Label(strings.Repeat("a", 100))
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
		ID:        "cns_abc12345",
		Label:     "lock ordering",
		Model:     "opus",
		SessionID: "8b6b15e2-9207-48cc-a3d1-73d8044b1765",
		CreatedAt: time.Now().Truncate(time.Second),
		LastUsed:  time.Now().Truncate(time.Second),
	}
	if err := st.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := st.Load("cns_abc12345")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != want.ID || got.SessionID != want.SessionID || got.Model != want.Model {
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
