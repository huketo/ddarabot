package store

import (
	"path/filepath"
	"testing"
)

func TestStore_MarkAndIsProcessed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := New(path)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer s.Close()

	uri := "at://did:plc:test/app.bsky.feed.post/abc123"

	if s.IsProcessed(uri) {
		t.Error("IsProcessed() = true before marking")
	}

	if err := s.MarkProcessed(uri, []string{"en", "ja"}); err != nil {
		t.Fatalf("MarkProcessed() error = %v", err)
	}

	if !s.IsProcessed(uri) {
		t.Error("IsProcessed() = false after marking")
	}
}

func TestStore_Cursor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := New(path)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer s.Close()

	cursor, ok := s.GetCursor()
	if ok {
		t.Error("GetCursor() ok = true before saving")
	}

	if err := s.SaveCursor(1725516665333808); err != nil {
		t.Fatalf("SaveCursor() error = %v", err)
	}

	cursor, ok = s.GetCursor()
	if !ok {
		t.Fatal("GetCursor() ok = false after saving")
	}
	if cursor != 1725516665333808 {
		t.Errorf("GetCursor() = %d, want 1725516665333808", cursor)
	}

	if err := s.SaveCursor(1725516665444000); err != nil {
		t.Fatalf("SaveCursor() error = %v", err)
	}
	cursor, _ = s.GetCursor()
	if cursor != 1725516665444000 {
		t.Errorf("GetCursor() = %d, want 1725516665444000", cursor)
	}
}

func TestStore_ProcessedLanguages(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := New(path)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer s.Close()

	uri := "at://did:plc:test/app.bsky.feed.post/lang123"

	// Unprocessed URI should return an empty map
	langs := s.ProcessedLanguages(uri)
	if len(langs) != 0 {
		t.Errorf("ProcessedLanguages() on unprocessed URI: got %v, want empty map", langs)
	}

	// Mark with two languages
	if err := s.MarkProcessed(uri, []string{"en", "ja"}); err != nil {
		t.Fatalf("MarkProcessed() error = %v", err)
	}

	langs = s.ProcessedLanguages(uri)
	if len(langs) != 2 {
		t.Fatalf("ProcessedLanguages() got %d entries, want 2", len(langs))
	}
	for _, l := range []string{"en", "ja"} {
		if !langs[l] {
			t.Errorf("ProcessedLanguages() missing language %q", l)
		}
	}

	// Mark again with an additional language
	if err := s.MarkProcessed(uri, []string{"en", "ja", "zh"}); err != nil {
		t.Fatalf("MarkProcessed() error = %v", err)
	}

	langs = s.ProcessedLanguages(uri)
	if len(langs) != 3 {
		t.Fatalf("ProcessedLanguages() got %d entries, want 3", len(langs))
	}
	for _, l := range []string{"en", "ja", "zh"} {
		if !langs[l] {
			t.Errorf("ProcessedLanguages() missing language %q after update", l)
		}
	}
}

func TestStore_CloseAndReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	s, err := New(path)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	uri := "at://did:plc:test/app.bsky.feed.post/xyz"
	s.MarkProcessed(uri, []string{"en"})
	s.SaveCursor(12345)
	s.Close()

	s2, err := New(path)
	if err != nil {
		t.Fatalf("New() reopen error = %v", err)
	}
	defer s2.Close()

	if !s2.IsProcessed(uri) {
		t.Error("data lost after reopen: IsProcessed = false")
	}
	cursor, ok := s2.GetCursor()
	if !ok || cursor != 12345 {
		t.Errorf("data lost after reopen: cursor = %d, ok = %v", cursor, ok)
	}
}
