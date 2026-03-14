# DDaraBot MVP Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go-based Bluesky bot that detects `#ddara` posts via Jetstream and auto-replies with translations using Genkit LLM.

**Architecture:** Event-driven daemon — Jetstream WebSocket listener feeds events through a channel to a filter → translator → poster pipeline. Genkit handles LLM calls; XRPC is implemented directly via `net/http`. BoltDB tracks processed posts and cursor state.

**Tech Stack:** Go 1.24, Genkit (multi-provider LLM), BoltDB, Jetstream official client, XRPC (`net/http`), TOML config, `log/slog`

**Spec:** `docs/superpowers/specs/2026-03-15-ddarabot-design.md`

**Reference docs:**
- Bluesky API: https://docs.bsky.app/docs/get-started
- Bluesky HTTP Reference: https://docs.bsky.app/docs/category/http-reference
- Genkit Go: https://genkit.dev/docs/go/get-started
- Jetstream: https://github.com/bluesky-social/jetstream

---

## File Structure

```
ddarabot/
├── cmd/
│   └── ddarabot/
│       └── main.go              # Entrypoint, Genkit init, signal handling, CLI
├── internal/
│   ├── config/
│   │   ├── config.go            # TOML config structs + loading + env overrides
│   │   └── config_test.go
│   ├── bluesky/
│   │   ├── auth.go              # Session create/refresh (XRPC direct)
│   │   ├── auth_test.go
│   │   ├── poster.go            # Reply posting (XRPC createRecord)
│   │   ├── poster_test.go
│   │   └── types.go             # XRPC request/response structs
│   ├── jetstream/
│   │   ├── listener.go          # Official Jetstream client wrapper
│   │   └── listener_test.go
│   ├── filter/
│   │   ├── filter.go            # #ddara detection, tag removal, preprocessing
│   │   └── filter_test.go
│   ├── translator/
│   │   ├── translator.go        # Genkit-based translation (Generate calls)
│   │   ├── translator_test.go
│   │   └── prompt.go            # Translation/summarization prompt templates
│   ├── store/
│   │   ├── store.go             # BoltDB dedup + cursor storage
│   │   └── store_test.go
│   └── bot/
│       ├── bot.go               # Main orchestration (pipeline wiring)
│       └── bot_test.go
├── config.example.toml
├── Dockerfile
├── .gitignore
├── Makefile
├── go.mod
└── go.sum
```

---

## Chunk 1: Project Foundation

### Task 1: Project Scaffold

**Files:**
- Create: `go.mod`
- Create: `Makefile`
- Create: `cmd/ddarabot/main.go` (minimal placeholder)

- [ ] **Step 1: Initialize Go module**

Run: `go mod init github.com/huketo/ddarabot`

- [ ] **Step 2: Create directory structure**

Run:
```bash
mkdir -p cmd/ddarabot internal/{config,bluesky,jetstream,filter,translator,store,bot}
```

- [ ] **Step 3: Create minimal main.go placeholder**

```go
// cmd/ddarabot/main.go
package main

import (
	"fmt"
	"os"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Printf("ddarabot %s\n", version)
		return
	}
	fmt.Println("ddarabot: not yet implemented")
}
```

- [ ] **Step 4: Create Makefile**

```makefile
APP_NAME := ddarabot
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

.PHONY: build run test clean

build:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/$(APP_NAME) ./cmd/ddarabot/

run:
	go run ./cmd/ddarabot/ --config config.toml

test:
	go test ./... -v

clean:
	rm -rf bin/
```

- [ ] **Step 5: Create .gitignore**

```
bin/
*.db
config.toml
```

- [ ] **Step 6: Verify build**

Run: `make build && ./bin/ddarabot version`
Expected: `ddarabot dev` (or a git hash)

- [ ] **Step 7: Commit**

```bash
git add go.mod Makefile cmd/ internal/ .gitignore
git commit -m "feat: initialize project scaffold"
```

---

### Task 2: Config Module

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

- [ ] **Step 1: Write failing test for TOML config loading**

```go
// internal/config/config_test.go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_ValidConfig(t *testing.T) {
	content := `
[bluesky]
handle = "test.bsky.social"
app_password = "test-password"
pds_host = "https://bsky.social"

[jetstream]
url = "wss://jetstream2.us-east.bsky.network/subscribe"
watched_dids = ["did:plc:test123"]

[translation]
source_language = "ko"
target_languages = ["en", "ja"]
trigger_hashtag = "ddara"
summarize_on_overflow = true
footer = "\n\n🌐 Translated by #DDaraBot"

[llm]
model = "googleai/gemini-2.5-flash"

[llm.googleai]
api_key = "test-key"

[store]
path = "./test.db"

[log]
level = "info"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Bluesky.Handle != "test.bsky.social" {
		t.Errorf("Handle = %q, want %q", cfg.Bluesky.Handle, "test.bsky.social")
	}
	if cfg.LLM.Model != "googleai/gemini-2.5-flash" {
		t.Errorf("Model = %q, want %q", cfg.LLM.Model, "googleai/gemini-2.5-flash")
	}
	if len(cfg.Translation.TargetLanguages) != 2 {
		t.Errorf("TargetLanguages len = %d, want 2", len(cfg.Translation.TargetLanguages))
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	content := `
[bluesky]
handle = "test.bsky.social"
app_password = "file-password"
pds_host = "https://bsky.social"

[jetstream]
url = "wss://jetstream2.us-east.bsky.network/subscribe"
watched_dids = ["did:plc:test123"]

[translation]
source_language = "ko"
target_languages = ["en"]
trigger_hashtag = "ddara"
summarize_on_overflow = true
footer = "\n\n🌐 Translated by #DDaraBot"

[llm]
model = "openai/gpt-4o-mini"

[llm.openai]
api_key = "file-key"

[store]
path = "./test.db"

[log]
level = "info"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	os.WriteFile(path, []byte(content), 0644)

	t.Setenv("DDARA_BLUESKY_APP_PASSWORD", "env-password")
	t.Setenv("OPENAI_API_KEY", "env-openai-key")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Bluesky.AppPassword != "env-password" {
		t.Errorf("AppPassword = %q, want %q", cfg.Bluesky.AppPassword, "env-password")
	}
	if cfg.LLM.OpenAI.APIKey != "env-openai-key" {
		t.Errorf("OpenAI.APIKey = %q, want %q", cfg.LLM.OpenAI.APIKey, "env-openai-key")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := Load("/nonexistent/config.toml")
	if err == nil {
		t.Fatal("Load() expected error for missing file")
	}
}

func TestLoad_InvalidTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.toml")
	os.WriteFile(path, []byte("not valid [[ toml"), 0644)

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() expected error for invalid TOML")
	}
}

func TestConfig_Provider(t *testing.T) {
	cfg := &Config{LLM: LLM{Model: "openai/gpt-4o-mini"}}
	if got := cfg.LLM.Provider(); got != "openai" {
		t.Errorf("Provider() = %q, want %q", got, "openai")
	}

	cfg.LLM.Model = "ollama/llama3"
	if got := cfg.LLM.Provider(); got != "ollama" {
		t.Errorf("Provider() = %q, want %q", got, "ollama")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -v`
Expected: FAIL (package not found / functions not defined)

- [ ] **Step 3: Install TOML dependency and write implementation**

Run: `go get github.com/BurntSushi/toml`

```go
// internal/config/config.go
package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Bluesky     Bluesky     `toml:"bluesky"`
	Jetstream   Jetstream   `toml:"jetstream"`
	Translation Translation `toml:"translation"`
	LLM         LLM         `toml:"llm"`
	Store       Store       `toml:"store"`
	Log         Log         `toml:"log"`
}

type Bluesky struct {
	Handle      string `toml:"handle"`
	AppPassword string `toml:"app_password"`
	PDSHost     string `toml:"pds_host"`
}

type Jetstream struct {
	URL        string   `toml:"url"`
	WatchedDIDs []string `toml:"watched_dids"`
}

type Translation struct {
	SourceLanguage      string   `toml:"source_language"`
	TargetLanguages     []string `toml:"target_languages"`
	TriggerHashtag      string   `toml:"trigger_hashtag"`
	SummarizeOnOverflow bool     `toml:"summarize_on_overflow"`
	Footer              string   `toml:"footer"`
}

type LLM struct {
	Model     string         `toml:"model"`
	OpenAI    OpenAIConfig   `toml:"openai"`
	Anthropic AnthropicConfig `toml:"anthropic"`
	GoogleAI  GoogleAIConfig `toml:"googleai"`
	Ollama    OllamaConfig   `toml:"ollama"`
	VertexAI  VertexAIConfig `toml:"vertexai"`
}

func (l *LLM) Provider() string {
	parts := strings.SplitN(l.Model, "/", 2)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

type OpenAIConfig struct {
	APIKey string `toml:"api_key"`
}

type AnthropicConfig struct {
	APIKey string `toml:"api_key"`
}

type GoogleAIConfig struct {
	APIKey string `toml:"api_key"`
}

type OllamaConfig struct {
	ServerAddress string `toml:"server_address"`
	Timeout       int    `toml:"timeout"`
}

type VertexAIConfig struct {
	ProjectID string `toml:"project_id"`
	Location  string `toml:"location"`
}

type Store struct {
	Path string `toml:"path"`
}

type Log struct {
	Level string `toml:"level"`
}

func Load(path string) (*Config, error) {
	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("load config %s: %w", path, err)
	}
	applyEnvOverrides(&cfg)
	return &cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("DDARA_BLUESKY_APP_PASSWORD"); v != "" {
		cfg.Bluesky.AppPassword = v
	}
	if v := os.Getenv("OPENAI_API_KEY"); v != "" {
		cfg.LLM.OpenAI.APIKey = v
	}
	if v := os.Getenv("ANTHROPIC_API_KEY"); v != "" {
		cfg.LLM.Anthropic.APIKey = v
	}
	if v := os.Getenv("GOOGLE_API_KEY"); v != "" {
		cfg.LLM.GoogleAI.APIKey = v
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/ -v`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config/ go.mod go.sum
git commit -m "feat: add TOML config loading with env overrides"
```

---

### Task 3: Store Module (BoltDB)

**Files:**
- Create: `internal/store/store.go`
- Create: `internal/store/store_test.go`

- [ ] **Step 1: Install BoltDB dependency**

Run: `go get go.etcd.io/bbolt`

- [ ] **Step 2: Write failing tests**

```go
// internal/store/store_test.go
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

	// No cursor saved yet
	cursor, ok := s.GetCursor()
	if ok {
		t.Error("GetCursor() ok = true before saving")
	}

	// Save cursor
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

	// Update cursor
	if err := s.SaveCursor(1725516665444000); err != nil {
		t.Fatalf("SaveCursor() error = %v", err)
	}
	cursor, _ = s.GetCursor()
	if cursor != 1725516665444000 {
		t.Errorf("GetCursor() = %d, want 1725516665444000", cursor)
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

	// Reopen
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
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/store/ -v`
Expected: FAIL

- [ ] **Step 4: Write implementation**

```go
// internal/store/store.go
package store

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	bolt "go.etcd.io/bbolt"
)

var (
	bucketProcessed = []byte("processed_posts")
	bucketCursor    = []byte("cursor")
	keyCursor       = []byte("jetstream_cursor")
)

type processedRecord struct {
	Timestamp int64    `json:"timestamp"`
	Languages []string `json:"languages"`
}

type Store struct {
	db *bolt.DB
}

func New(path string) (*Store, error) {
	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open bolt db: %w", err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(bucketProcessed); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(bucketCursor); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("create buckets: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) IsProcessed(uri string) bool {
	var found bool
	s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketProcessed)
		found = b.Get([]byte(uri)) != nil
		return nil
	})
	return found
}

func (s *Store) MarkProcessed(uri string, languages []string) error {
	rec := processedRecord{
		Timestamp: time.Now().Unix(),
		Languages: languages,
	}
	val, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketProcessed).Put([]byte(uri), val)
	})
}

func (s *Store) SaveCursor(cursor int64) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketCursor).Put(keyCursor, []byte(strconv.FormatInt(cursor, 10)))
	})
}

func (s *Store) GetCursor() (int64, bool) {
	var cursor int64
	var found bool
	s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketCursor)
		v := b.Get(keyCursor)
		if v != nil {
			c, err := strconv.ParseInt(string(v), 10, 64)
			if err == nil {
				cursor = c
				found = true
			}
		}
		return nil
	})
	return cursor, found
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/store/ -v`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git add internal/store/ go.mod go.sum
git commit -m "feat: add BoltDB store for dedup and cursor"
```

---

## Chunk 2: Filter Engine

### Task 4: Filter Module

**Files:**
- Create: `internal/filter/filter.go`
- Create: `internal/filter/filter_test.go`

This is pure logic with no external dependencies — highly testable.

- [ ] **Step 1: Write failing tests**

```go
// internal/filter/filter_test.go
package filter

import (
	"encoding/json"
	"testing"
)

func TestHasTriggerTag(t *testing.T) {
	tests := []struct {
		name    string
		facets  json.RawMessage
		tag     string
		want    bool
	}{
		{
			name: "has ddara tag",
			facets: json.RawMessage(`[{
				"index": {"byteStart": 20, "byteEnd": 26},
				"features": [{"$type": "app.bsky.richtext.facet#tag", "tag": "ddara"}]
			}]`),
			tag:  "ddara",
			want: true,
		},
		{
			name: "case insensitive",
			facets: json.RawMessage(`[{
				"index": {"byteStart": 0, "byteEnd": 6},
				"features": [{"$type": "app.bsky.richtext.facet#tag", "tag": "DDara"}]
			}]`),
			tag:  "ddara",
			want: true,
		},
		{
			name: "no matching tag",
			facets: json.RawMessage(`[{
				"index": {"byteStart": 0, "byteEnd": 5},
				"features": [{"$type": "app.bsky.richtext.facet#tag", "tag": "other"}]
			}]`),
			tag:  "ddara",
			want: false,
		},
		{
			name: "no facets",
			facets: nil,
			tag:    "ddara",
			want:   false,
		},
		{
			name: "empty facets array",
			facets: json.RawMessage(`[]`),
			tag:    "ddara",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasTriggerTag(tt.facets, tt.tag)
			if got != tt.want {
				t.Errorf("HasTriggerTag() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRemoveTriggerTag(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		facets json.RawMessage
		tag    string
		want   string
	}{
		{
			// "오늘 날씨가 좋네요! " = 28 bytes (9 Korean chars × 3 bytes + "! " 2 bytes)
			// "#ddara" = 6 bytes → byteStart=29, byteEnd=35
			name: "remove trailing #ddara",
			text: "오늘 날씨가 좋네요! #ddara",
			facets: json.RawMessage(`[{
				"index": {"byteStart": 29, "byteEnd": 35},
				"features": [{"$type": "app.bsky.richtext.facet#tag", "tag": "ddara"}]
			}]`),
			tag:  "ddara",
			want: "오늘 날씨가 좋네요!",
		},
		{
			name: "remove #ddara with leading space",
			text: "Hello world #ddara",
			facets: json.RawMessage(`[{
				"index": {"byteStart": 12, "byteEnd": 18},
				"features": [{"$type": "app.bsky.richtext.facet#tag", "tag": "ddara"}]
			}]`),
			tag:  "ddara",
			want: "Hello world",
		},
		{
			name: "no ddara tag to remove",
			text: "Hello world",
			facets: nil,
			tag:    "ddara",
			want:   "Hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RemoveTriggerTag(tt.text, tt.facets, tt.tag)
			if got != tt.want {
				t.Errorf("RemoveTriggerTag() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsReply(t *testing.T) {
	tests := []struct {
		name   string
		record json.RawMessage
		want   bool
	}{
		{
			name:   "is a reply",
			record: json.RawMessage(`{"$type":"app.bsky.feed.post","text":"reply","reply":{"root":{},"parent":{}}}`),
			want:   true,
		},
		{
			name:   "not a reply",
			record: json.RawMessage(`{"$type":"app.bsky.feed.post","text":"original post"}`),
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsReply(tt.record)
			if got != tt.want {
				t.Errorf("IsReply() = %v, want %v", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/filter/ -v`
Expected: FAIL

- [ ] **Step 3: Write implementation**

```go
// internal/filter/filter.go
package filter

import (
	"encoding/json"
	"strings"
)

type facet struct {
	Index    facetIndex    `json:"index"`
	Features []facetFeature `json:"features"`
}

type facetIndex struct {
	ByteStart int `json:"byteStart"`
	ByteEnd   int `json:"byteEnd"`
}

type facetFeature struct {
	Type string `json:"$type"`
	Tag  string `json:"tag"`
}

type replyCheck struct {
	Reply *json.RawMessage `json:"reply"`
}

// HasTriggerTag checks if facets contain the trigger hashtag.
func HasTriggerTag(facetsJSON json.RawMessage, triggerTag string) bool {
	if len(facetsJSON) == 0 {
		return false
	}
	var facets []facet
	if err := json.Unmarshal(facetsJSON, &facets); err != nil {
		return false
	}
	for _, f := range facets {
		for _, feat := range f.Features {
			if feat.Type == "app.bsky.richtext.facet#tag" &&
				strings.EqualFold(feat.Tag, triggerTag) {
				return true
			}
		}
	}
	return false
}

// RemoveTriggerTag removes the trigger hashtag text from the post using facet byte indices.
func RemoveTriggerTag(text string, facetsJSON json.RawMessage, triggerTag string) string {
	if len(facetsJSON) == 0 {
		return text
	}
	var facets []facet
	if err := json.Unmarshal(facetsJSON, &facets); err != nil {
		return text
	}

	textBytes := []byte(text)
	for _, f := range facets {
		for _, feat := range f.Features {
			if feat.Type == "app.bsky.richtext.facet#tag" &&
				strings.EqualFold(feat.Tag, triggerTag) {
				start := f.Index.ByteStart
				end := f.Index.ByteEnd
				if start > len(textBytes) || end > len(textBytes) {
					continue
				}
				// Remove leading space before tag if present
				if start > 0 && textBytes[start-1] == ' ' {
					start--
				}
				result := make([]byte, 0, len(textBytes))
				result = append(result, textBytes[:start]...)
				result = append(result, textBytes[end:]...)
				return strings.TrimSpace(string(result))
			}
		}
	}
	return text
}

// IsReply checks if a post record is a reply.
func IsReply(record json.RawMessage) bool {
	var rc replyCheck
	if err := json.Unmarshal(record, &rc); err != nil {
		return false
	}
	return rc.Reply != nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/filter/ -v`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/filter/
git commit -m "feat: add filter engine for #ddara tag detection"
```

---

## Chunk 3: Translator (Genkit)

### Task 5: Prompt Templates

**Files:**
- Create: `internal/translator/prompt.go`

- [ ] **Step 1: Write prompt template functions**

```go
// internal/translator/prompt.go
package translator

import "fmt"

func buildTranslateSystemPrompt(sourceLang, targetLang string) string {
	return fmt.Sprintf(`You are a professional translator. Translate the following social media post from %s to %s.

Rules:
- Keep the tone and nuance of the original
- Preserve @mentions, URLs, and emoji as-is
- Do not add explanations or notes
- Output only the translated text
- Keep hashtags in their original language (do not translate hashtags)`, sourceLang, targetLang)
}

func buildSummarizeSystemPrompt(sourceLang, targetLang string, maxGraphemes int) string {
	return fmt.Sprintf(`You are a professional translator. The following social media post needs to be translated from %s to %s and condensed to fit within %d graphemes.

Rules:
- Preserve the core meaning
- Keep the tone natural for social media
- Output only the translated and condensed text`, sourceLang, targetLang, maxGraphemes)
}
```

- [ ] **Step 2: Write prompt tests**

```go
// internal/translator/prompt_test.go
package translator

import (
	"strings"
	"testing"
)

func TestBuildTranslateSystemPrompt(t *testing.T) {
	prompt := buildTranslateSystemPrompt("ko", "en")
	if !strings.Contains(prompt, "ko") || !strings.Contains(prompt, "en") {
		t.Errorf("prompt missing language codes: %s", prompt)
	}
	if !strings.Contains(prompt, "do not translate hashtags") {
		t.Error("prompt missing hashtag rule")
	}
}

func TestBuildSummarizeSystemPrompt(t *testing.T) {
	prompt := buildSummarizeSystemPrompt("ko", "en", 250)
	if !strings.Contains(prompt, "250") {
		t.Errorf("prompt missing max graphemes: %s", prompt)
	}
}
```

- [ ] **Step 3: Run tests to verify they pass**

Run: `go test ./internal/translator/ -v -run TestBuild`
Expected: All PASS

- [ ] **Step 4: Commit**

```bash
git add internal/translator/prompt.go internal/translator/prompt_test.go
git commit -m "feat: add translation prompt templates"
```

---

### Task 6: Translator with Genkit

**Files:**
- Create: `internal/translator/translator.go`
- Create: `internal/translator/translator_test.go`

- [ ] **Step 1: Install dependencies**

Run:
```bash
go get github.com/firebase/genkit/go/genkit
go get github.com/firebase/genkit/go/ai
go get github.com/firebase/genkit/go/plugins/googlegenai
go get github.com/firebase/genkit/go/plugins/compat_oai/openai
go get github.com/firebase/genkit/go/plugins/compat_oai/anthropic
go get github.com/firebase/genkit/go/plugins/ollama
go get github.com/openai/openai-go
go get github.com/rivo/uniseg
```

**Note:** `github.com/openai/openai-go` is required by the Anthropic plugin's `option.WithAPIKey`. Verify Genkit's `genkit.Init` return signature — if it returns `(*genkit.Genkit, error)`, adapt `initGenkit()` in Task 11 accordingly.

- [ ] **Step 2: Write failing tests**

Testing the Translator requires an actual LLM, so we test the grapheme counting and pipeline logic with a mock approach — inject a `translateFunc` for unit tests.

```go
// internal/translator/translator_test.go
package translator

import (
	"context"
	"strings"
	"testing"

	"github.com/rivo/uniseg"
)

func TestCountGraphemes(t *testing.T) {
	tests := []struct {
		text string
		want int
	}{
		{"hello", 5},
		{"한국어", 3},
		{"👨‍👩‍👧‍👦", 1},  // family emoji = 1 grapheme
		{"hello 🌍", 7},
	}

	for _, tt := range tests {
		got := uniseg.GraphemeClusterCount(tt.text)
		if got != tt.want {
			t.Errorf("GraphemeClusterCount(%q) = %d, want %d", tt.text, got, tt.want)
		}
	}
}

func TestNeedsOverflowSummary(t *testing.T) {
	footer := "\n\n🌐 Translated by #DDaraBot"
	footerLen := uniseg.GraphemeClusterCount(footer)

	shortText := strings.Repeat("a", 200)
	if needsSummary(shortText, footer, footerLen) {
		t.Error("short text should not need summary")
	}

	longText := strings.Repeat("a", 300)
	if !needsSummary(longText, footer, footerLen) {
		t.Error("long text should need summary")
	}
}

func TestBuildReplyText(t *testing.T) {
	footer := "\n\n🌐 Translated by #DDaraBot"
	result := buildReplyText("Hello world", footer)
	want := "Hello world\n\n🌐 Translated by #DDaraBot"
	if result != want {
		t.Errorf("buildReplyText() = %q, want %q", result, want)
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/translator/ -v`
Expected: FAIL

- [ ] **Step 4: Write implementation**

```go
// internal/translator/translator.go
package translator

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/rivo/uniseg"
)

const maxGraphemes = 300

type Translator struct {
	g      *genkit.Genkit
	model  string
	footer string
	logger *slog.Logger
}

func New(g *genkit.Genkit, model, footer string, logger *slog.Logger) *Translator {
	return &Translator{
		g:      g,
		model:  model,
		footer: footer,
		logger: logger,
	}
}

type TranslationResult struct {
	Lang string
	Text string
	Err  error
}

func (t *Translator) TranslateAll(ctx context.Context, text, sourceLang string, targetLangs []string) (map[string]string, []error) {
	ch := make(chan TranslationResult, len(targetLangs))
	for _, lang := range targetLangs {
		go func() {
			translated, err := t.translateWithRetry(ctx, text, sourceLang, lang)
			ch <- TranslationResult{Lang: lang, Text: translated, Err: err}
		}()
	}

	results := make(map[string]string)
	var errs []error
	for range targetLangs {
		r := <-ch
		if r.Err != nil {
			errs = append(errs, fmt.Errorf("translate to %s: %w", r.Lang, r.Err))
			t.logger.Error("translation failed", "lang", r.Lang, "error", r.Err)
		} else {
			results[r.Lang] = r.Text
		}
	}
	return results, errs
}

func (t *Translator) translateWithRetry(ctx context.Context, text, sourceLang, targetLang string) (string, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			delay := time.Duration(1<<uint(attempt-1)) * time.Second
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}
		result, err := t.translate(ctx, text, sourceLang, targetLang)
		if err == nil {
			return result, nil
		}
		lastErr = err
		t.logger.Warn("translation attempt failed", "lang", targetLang, "attempt", attempt+1, "error", err)
	}
	return "", fmt.Errorf("after 3 retries: %w", lastErr)
}

func (t *Translator) translate(ctx context.Context, text, sourceLang, targetLang string) (string, error) {
	resp, err := genkit.Generate(ctx, t.g,
		ai.WithModelName(t.model),
		ai.WithSystem(buildTranslateSystemPrompt(sourceLang, targetLang)),
		ai.WithPrompt(text),
	)
	if err != nil {
		return "", err
	}

	translated := resp.Text()
	footerLen := uniseg.GraphemeClusterCount(t.footer)

	if needsSummary(translated, t.footer, footerLen) {
		maxChars := maxGraphemes - footerLen
		resp, err = genkit.Generate(ctx, t.g,
			ai.WithModelName(t.model),
			ai.WithSystem(buildSummarizeSystemPrompt(sourceLang, targetLang, maxChars)),
			ai.WithPrompt(text),
		)
		if err != nil {
			return "", fmt.Errorf("summarize: %w", err)
		}
		translated = resp.Text()
	}

	return buildReplyText(translated, t.footer), nil
}

func needsSummary(text, footer string, footerLen int) bool {
	return uniseg.GraphemeClusterCount(text)+footerLen > maxGraphemes
}

func buildReplyText(translated, footer string) string {
	return translated + footer
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/translator/ -v`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git add internal/translator/ go.mod go.sum
git commit -m "feat: add Genkit-based translator with retry and grapheme counting"
```

---

## Chunk 4: Bluesky Client

### Task 7: Types and Auth

**Files:**
- Create: `internal/bluesky/types.go`
- Create: `internal/bluesky/auth.go`
- Create: `internal/bluesky/auth_test.go`

- [ ] **Step 1: Write types**

```go
// internal/bluesky/types.go
package bluesky

// XRPC request/response types for AT Protocol

type CreateSessionRequest struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

type Session struct {
	AccessJwt  string `json:"accessJwt"`
	RefreshJwt string `json:"refreshJwt"`
	DID        string `json:"did"`
	Handle     string `json:"handle"`
}

type CreateRecordRequest struct {
	Repo       string      `json:"repo"`
	Collection string      `json:"collection"`
	Record     interface{} `json:"record"`
}

type CreateRecordResponse struct {
	URI string `json:"uri"`
	CID string `json:"cid"`
}

type PostRecord struct {
	Type      string      `json:"$type"`
	Text      string      `json:"text"`
	CreatedAt string      `json:"createdAt"`
	Langs     []string    `json:"langs,omitempty"`
	Reply     *ReplyRef   `json:"reply,omitempty"`
	Facets    []PostFacet `json:"facets,omitempty"`
}

type ReplyRef struct {
	Root   StrongRef `json:"root"`
	Parent StrongRef `json:"parent"`
}

type StrongRef struct {
	URI string `json:"uri"`
	CID string `json:"cid"`
}

type PostFacet struct {
	Index    FacetIndex     `json:"index"`
	Features []FacetFeature `json:"features"`
}

type FacetIndex struct {
	ByteStart int `json:"byteStart"`
	ByteEnd   int `json:"byteEnd"`
}

type FacetFeature struct {
	Type string `json:"$type"`
	Tag  string `json:"tag,omitempty"`
}

type XRPCError struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}
```

- [ ] **Step 2: Write failing auth tests**

```go
// internal/bluesky/auth_test.go
package bluesky

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuth_CreateSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/xrpc/com.atproto.server.createSession" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		json.NewEncoder(w).Encode(Session{
			AccessJwt:  "access-token",
			RefreshJwt: "refresh-token",
			DID:        "did:plc:test",
			Handle:     "test.bsky.social",
		})
	}))
	defer server.Close()

	auth := NewAuth(server.URL, "test.bsky.social", "test-password")
	session, err := auth.CreateSession(context.Background())
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if session.AccessJwt != "access-token" {
		t.Errorf("AccessJwt = %q, want %q", session.AccessJwt, "access-token")
	}
	if session.DID != "did:plc:test" {
		t.Errorf("DID = %q, want %q", session.DID, "did:plc:test")
	}
}

func TestAuth_RefreshSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/xrpc/com.atproto.server.refreshSession" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer old-refresh-token" {
			t.Errorf("Authorization = %q, want %q", auth, "Bearer old-refresh-token")
		}
		json.NewEncoder(w).Encode(Session{
			AccessJwt:  "new-access-token",
			RefreshJwt: "new-refresh-token",
			DID:        "did:plc:test",
		})
	}))
	defer server.Close()

	auth := NewAuth(server.URL, "test.bsky.social", "test-password")
	auth.session = &Session{RefreshJwt: "old-refresh-token"}

	session, err := auth.RefreshSession(context.Background())
	if err != nil {
		t.Fatalf("RefreshSession() error = %v", err)
	}
	if session.AccessJwt != "new-access-token" {
		t.Errorf("AccessJwt = %q, want %q", session.AccessJwt, "new-access-token")
	}
}

func TestAuth_GetSession_AutoRefresh(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		switch r.URL.Path {
		case "/xrpc/com.atproto.server.createSession":
			json.NewEncoder(w).Encode(Session{
				AccessJwt:  "access-token",
				RefreshJwt: "refresh-token",
				DID:        "did:plc:test",
			})
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	auth := NewAuth(server.URL, "test.bsky.social", "test-password")

	// First call should create a session
	session, err := auth.GetSession(context.Background())
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if session.AccessJwt != "access-token" {
		t.Errorf("AccessJwt = %q, want %q", session.AccessJwt, "access-token")
	}
	if callCount != 1 {
		t.Errorf("callCount = %d, want 1", callCount)
	}

	// Second call should reuse session
	session, err = auth.GetSession(context.Background())
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if callCount != 1 {
		t.Errorf("callCount = %d, want 1 (should reuse)", callCount)
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/bluesky/ -v`
Expected: FAIL

- [ ] **Step 4: Write auth implementation**

```go
// internal/bluesky/auth.go
package bluesky

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

type Auth struct {
	pdsHost  string
	handle   string
	password string
	client   *http.Client
	session  *Session
	mu       sync.RWMutex
}

func NewAuth(pdsHost, handle, password string) *Auth {
	return &Auth{
		pdsHost:  pdsHost,
		handle:   handle,
		password: password,
		client:   &http.Client{},
	}
}

func (a *Auth) GetSession(ctx context.Context) (*Session, error) {
	a.mu.RLock()
	s := a.session
	a.mu.RUnlock()
	if s != nil {
		return s, nil
	}
	return a.CreateSession(ctx)
}

func (a *Auth) CreateSession(ctx context.Context) (*Session, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	body, err := json.Marshal(CreateSessionRequest{
		Identifier: a.handle,
		Password:   a.password,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		a.pdsHost+"/xrpc/com.atproto.server.createSession",
		bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var xrpcErr XRPCError
		json.NewDecoder(resp.Body).Decode(&xrpcErr)
		return nil, fmt.Errorf("create session: %s %s", xrpcErr.Error, xrpcErr.Message)
	}

	var session Session
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return nil, fmt.Errorf("decode session: %w", err)
	}

	a.session = &session
	return &session, nil
}

func (a *Auth) RefreshSession(ctx context.Context) (*Session, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.session == nil {
		return nil, fmt.Errorf("no session to refresh")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		a.pdsHost+"/xrpc/com.atproto.server.refreshSession", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+a.session.RefreshJwt)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("refresh session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Refresh failed, clear session so next GetSession creates a new one
		a.session = nil
		return nil, fmt.Errorf("refresh session: status %d", resp.StatusCode)
	}

	var session Session
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return nil, fmt.Errorf("decode session: %w", err)
	}

	a.session = &session
	return &session, nil
}

// InvalidateSession clears the current session, forcing re-auth on next GetSession.
func (a *Auth) InvalidateSession() {
	a.mu.Lock()
	a.session = nil
	a.mu.Unlock()
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/bluesky/ -v`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git add internal/bluesky/
git commit -m "feat: add XRPC auth with session create/refresh"
```

---

### Task 8: Poster (Reply Creation)

**Files:**
- Create: `internal/bluesky/poster.go`
- Create: `internal/bluesky/poster_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/bluesky/poster_test.go
package bluesky

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBuildFacets(t *testing.T) {
	text := "Hello world\n\n🌐 Translated by #DDaraBot"
	facets := BuildHashtagFacets(text, "DDaraBot")

	if len(facets) != 1 {
		t.Fatalf("len(facets) = %d, want 1", len(facets))
	}

	f := facets[0]
	// #DDaraBot should be found in the text
	textBytes := []byte(text)
	tag := string(textBytes[f.Index.ByteStart:f.Index.ByteEnd])
	if tag != "#DDaraBot" {
		t.Errorf("facet tag text = %q, want %q", tag, "#DDaraBot")
	}
	if f.Features[0].Type != "app.bsky.richtext.facet#tag" {
		t.Errorf("feature type = %q, want %q", f.Features[0].Type, "app.bsky.richtext.facet#tag")
	}
	if f.Features[0].Tag != "DDaraBot" {
		t.Errorf("feature tag = %q, want %q", f.Features[0].Tag, "DDaraBot")
	}
}

func TestPoster_PostReply(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/xrpc/com.atproto.server.createSession":
			json.NewEncoder(w).Encode(Session{
				AccessJwt: "token", RefreshJwt: "refresh", DID: "did:plc:me",
			})
		case "/xrpc/com.atproto.repo.createRecord":
			if r.Header.Get("Authorization") != "Bearer token" {
				t.Error("missing auth header")
			}
			var req CreateRecordRequest
			json.NewDecoder(r.Body).Decode(&req)
			if req.Repo != "did:plc:me" {
				t.Errorf("repo = %q, want %q", req.Repo, "did:plc:me")
			}
			json.NewEncoder(w).Encode(CreateRecordResponse{
				URI: "at://did:plc:me/app.bsky.feed.post/reply123",
				CID: "bafytest",
			})
		}
	}))
	defer server.Close()

	auth := NewAuth(server.URL, "test.bsky.social", "password")
	poster := NewPoster(auth, server.URL, slog.Default(), false)

	original := OriginalPost{
		URI: "at://did:plc:author/app.bsky.feed.post/orig123",
		CID: "bafyorig",
	}

	err := poster.PostReply(context.Background(), original, "en",
		"Hello world\n\n🌐 Translated by #DDaraBot")
	if err != nil {
		t.Fatalf("PostReply() error = %v", err)
	}
}

func TestPoster_DryRun(t *testing.T) {
	poster := NewPoster(nil, "", slog.Default(), true)

	err := poster.PostReply(context.Background(), OriginalPost{
		URI: "at://test/app.bsky.feed.post/123",
		CID: "bafytest",
	}, "en", "translated text")

	if err != nil {
		t.Fatalf("PostReply() dry-run error = %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/bluesky/ -v -run TestBuild`
Expected: FAIL

- [ ] **Step 3: Write poster implementation**

```go
// internal/bluesky/poster.go
package bluesky

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"net/http"
	"time"
)

type OriginalPost struct {
	URI string
	CID string
}

type Poster struct {
	auth     *Auth
	pdsHost  string
	logger   *slog.Logger
	dryRun   bool
	client   *http.Client
}

func NewPoster(auth *Auth, pdsHost string, logger *slog.Logger, dryRun bool) *Poster {
	return &Poster{
		auth:    auth,
		pdsHost: pdsHost,
		logger:  logger,
		dryRun:  dryRun,
		client:  &http.Client{},
	}
}

func (p *Poster) PostReply(ctx context.Context, original OriginalPost, lang, text string) error {
	if p.dryRun {
		p.logger.Info("[dry-run] would post reply", "lang", lang, "text", text, "parent", original.URI)
		return nil
	}

	session, err := p.auth.GetSession(ctx)
	if err != nil {
		return fmt.Errorf("get session: %w", err)
	}

	record := PostRecord{
		Type:      "app.bsky.feed.post",
		Text:      text,
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Langs:     []string{lang},
		Reply: &ReplyRef{
			Root:   StrongRef{URI: original.URI, CID: original.CID},
			Parent: StrongRef{URI: original.URI, CID: original.CID},
		},
		Facets: BuildHashtagFacets(text, "DDaraBot"),
	}

	reqBody := CreateRecordRequest{
		Repo:       session.DID,
		Collection: "app.bsky.feed.post",
		Record:     record,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.pdsHost+"/xrpc/com.atproto.repo.createRecord",
		bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+session.AccessJwt)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("create record: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		// Token expired, try refresh
		p.auth.InvalidateSession()
		return fmt.Errorf("session expired, will retry")
	}

	if resp.StatusCode != http.StatusOK {
		var xrpcErr XRPCError
		json.NewDecoder(resp.Body).Decode(&xrpcErr)
		return fmt.Errorf("create record: %d %s %s", resp.StatusCode, xrpcErr.Error, xrpcErr.Message)
	}

	var result CreateRecordResponse
	json.NewDecoder(resp.Body).Decode(&result)
	p.logger.Info("posted reply", "lang", lang, "uri", result.URI)
	return nil
}

// PostAll posts replies for all translations concurrently with semaphore.
func (p *Poster) PostAll(ctx context.Context, original OriginalPost, translations map[string]string, maxConcurrent int) []error {
	if maxConcurrent <= 0 {
		maxConcurrent = 3
	}

	type result struct {
		lang string
		err  error
	}

	sem := make(chan struct{}, maxConcurrent)
	ch := make(chan result, len(translations))

	for lang, text := range translations {
		go func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			ch <- result{lang: lang, err: p.PostReply(ctx, original, lang, text)}
		}()
	}

	var errs []error
	for range translations {
		r := <-ch
		if r.err != nil {
			errs = append(errs, fmt.Errorf("post %s: %w", r.lang, r.err))
		}
	}
	return errs
}

// BuildHashtagFacets creates facets for hashtags in the text.
func BuildHashtagFacets(text string, tag string) []PostFacet {
	hashTag := "#" + tag
	textBytes := []byte(text)
	idx := strings.Index(text, hashTag)
	if idx == -1 {
		return nil
	}

	byteStart := len([]byte(text[:idx]))
	byteEnd := byteStart + len([]byte(hashTag))
	if byteEnd > len(textBytes) {
		return nil
	}

	return []PostFacet{
		{
			Index: FacetIndex{ByteStart: byteStart, ByteEnd: byteEnd},
			Features: []FacetFeature{
				{
					Type: "app.bsky.richtext.facet#tag",
					Tag:  tag,
				},
			},
		},
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/bluesky/ -v`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/bluesky/
git commit -m "feat: add XRPC poster for reply creation with facets"
```

---

## Chunk 5: Event Pipeline

### Task 9: Jetstream Listener

**Files:**
- Create: `internal/jetstream/listener.go`
- Create: `internal/jetstream/listener_test.go`

**Note:** The official `bluesky-social/jetstream/pkg/client` uses a `Scheduler` interface. Our listener wraps it and feeds events into a channel. The Jetstream client pulls in `indigo` as a transitive dependency for Account/Identity event types, but we only consume commit events.

- [ ] **Step 1: Install Jetstream client**

Run: `go get github.com/bluesky-social/jetstream/pkg/client`

Consult the Jetstream docs at https://github.com/bluesky-social/jetstream to verify the latest API. The `pkg/client` package exports `NewClient`, `ClientConfig`, and the `Scheduler` interface. If the API has changed, adapt accordingly.

- [ ] **Step 2: Write listener implementation**

```go
// internal/jetstream/listener.go
package jetstream

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	jsclient "github.com/bluesky-social/jetstream/pkg/client"
	jsmodels "github.com/bluesky-social/jetstream/pkg/models"
)

// Post represents the relevant fields from a Bluesky post record.
type Post struct {
	DID        string
	RKey       string
	CID        string
	TimeUS     int64
	Text       string
	Facets     json.RawMessage
	Record     json.RawMessage
}

type postRecord struct {
	Type   string          `json:"$type"`
	Text   string          `json:"text"`
	Facets json.RawMessage `json:"facets"`
	Reply  *json.RawMessage `json:"reply"`
}

// Listener wraps the Jetstream client and sends posts to a channel.
type Listener struct {
	config   *jsclient.ClientConfig
	logger   *slog.Logger
	eventCh  chan<- Post
	saveCursor func(int64) error
}

func NewListener(
	url string,
	watchedDIDs []string,
	logger *slog.Logger,
	eventCh chan<- Post,
	saveCursor func(int64) error,
) *Listener {
	cfg := &jsclient.ClientConfig{
		Compress:          true,
		WebsocketURL:      url,
		WantedDids:        watchedDIDs,
		WantedCollections: []string{"app.bsky.feed.post"},
	}
	return &Listener{
		config:     cfg,
		logger:     logger,
		eventCh:    eventCh,
		saveCursor: saveCursor,
	}
}

// scheduler implements jsclient.Scheduler
type scheduler struct {
	listener *Listener
	count    int64
	lastSave time.Time
}

func (s *scheduler) AddWork(ctx context.Context, repo string, evt *jsmodels.Event) error {
	if evt.Kind != jsmodels.EventKindCommit || evt.Commit == nil {
		return nil
	}
	if evt.Commit.Operation != jsmodels.CommitOperationCreate {
		return nil
	}

	var rec postRecord
	if err := json.Unmarshal(evt.Commit.Record, &rec); err != nil {
		return nil // skip unparseable records
	}

	post := Post{
		DID:    evt.Did,
		RKey:   evt.Commit.RKey,
		CID:    evt.Commit.CID,
		TimeUS: evt.TimeUS,
		Text:   rec.Text,
		Facets: rec.Facets,
		Record: evt.Commit.Record,
	}

	select {
	case s.listener.eventCh <- post:
	case <-ctx.Done():
		return ctx.Err()
	}

	// Periodic cursor save: every 100 events or 30 seconds
	s.count++
	if s.count%100 == 0 || time.Since(s.lastSave) >= 30*time.Second {
		if err := s.listener.saveCursor(evt.TimeUS); err != nil {
			s.listener.logger.Warn("failed to save cursor", "error", err)
		}
		s.lastSave = time.Now()
	}

	return nil
}

func (s *scheduler) Shutdown() {}

// Run connects to Jetstream and reads events until context is cancelled.
// Implements reconnection with exponential backoff.
func (l *Listener) Run(ctx context.Context, cursor *int64) error {
	backoff := 1 * time.Second
	maxBackoff := 60 * time.Second

	for {
		sched := &scheduler{listener: l, lastSave: time.Now()}
		client, err := jsclient.NewClient(l.config, l.logger, sched)
		if err != nil {
			return fmt.Errorf("create jetstream client: %w", err)
		}

		l.logger.Info("connecting to jetstream", "url", l.config.WebsocketURL)
		err = client.ConnectAndRead(ctx, cursor)

		if ctx.Err() != nil {
			// Graceful shutdown: save final cursor
			if cursor != nil {
				l.saveCursor(*cursor)
			}
			return ctx.Err()
		}

		l.logger.Warn("jetstream disconnected, reconnecting", "error", err, "backoff", backoff)

		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return ctx.Err()
		}

		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}
```

- [ ] **Step 3: Write basic test**

```go
// internal/jetstream/listener_test.go
package jetstream

import (
	"log/slog"
	"testing"
)

func TestNewListener(t *testing.T) {
	ch := make(chan Post, 10)
	l := NewListener(
		"wss://jetstream2.us-east.bsky.network/subscribe",
		[]string{"did:plc:test"},
		slog.Default(),
		ch,
		func(cursor int64) error { return nil },
	)

	if l.config.WebsocketURL != "wss://jetstream2.us-east.bsky.network/subscribe" {
		t.Errorf("URL = %q", l.config.WebsocketURL)
	}
	if len(l.config.WantedDids) != 1 || l.config.WantedDids[0] != "did:plc:test" {
		t.Errorf("WantedDids = %v", l.config.WantedDids)
	}
	if !l.config.Compress {
		t.Error("Compress should be true")
	}
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/jetstream/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/jetstream/ go.mod go.sum
git commit -m "feat: add Jetstream listener with reconnection and cursor tracking"
```

---

### Task 10: Bot Orchestrator

**Files:**
- Create: `internal/bot/bot.go`
- Create: `internal/bot/bot_test.go`

- [ ] **Step 1: Write failing test**

```go
// internal/bot/bot_test.go
package bot

import (
	"testing"
)

func TestPostURI(t *testing.T) {
	uri := buildPostURI("did:plc:test", "abc123")
	want := "at://did:plc:test/app.bsky.feed.post/abc123"
	if uri != want {
		t.Errorf("buildPostURI() = %q, want %q", uri, want)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/bot/ -v`
Expected: FAIL

- [ ] **Step 3: Write bot implementation**

```go
// internal/bot/bot.go
package bot

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/huketo/ddarabot/internal/bluesky"
	"github.com/huketo/ddarabot/internal/config"
	"github.com/huketo/ddarabot/internal/filter"
	"github.com/huketo/ddarabot/internal/jetstream"
	"github.com/huketo/ddarabot/internal/store"
	"github.com/huketo/ddarabot/internal/translator"
)

type Bot struct {
	cfg        *config.Config
	store      *store.Store
	translator *translator.Translator
	poster     *bluesky.Poster
	listener   *jetstream.Listener
	eventCh    chan jetstream.Post
	logger     *slog.Logger
}

func New(
	cfg *config.Config,
	st *store.Store,
	tr *translator.Translator,
	poster *bluesky.Poster,
	logger *slog.Logger,
) *Bot {
	eventCh := make(chan jetstream.Post, 64)
	listener := jetstream.NewListener(
		cfg.Jetstream.URL,
		cfg.Jetstream.WatchedDIDs,
		logger,
		eventCh,
		st.SaveCursor,
	)

	return &Bot{
		cfg:        cfg,
		store:      st,
		translator: tr,
		poster:     poster,
		listener:   listener,
		eventCh:    eventCh,
		logger:     logger,
	}
}

func (b *Bot) Run(ctx context.Context) error {
	// Get cursor for reconnection
	var cursor *int64
	if c, ok := b.store.GetCursor(); ok {
		cursor = &c
		b.logger.Info("resuming from cursor", "cursor", c)
	}

	// Start Jetstream listener in a goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- b.listener.Run(ctx, cursor)
	}()

	// Process events
	for {
		select {
		case post := <-b.eventCh:
			b.processPost(ctx, post)
		case err := <-errCh:
			return fmt.Errorf("jetstream listener: %w", err)
		case <-ctx.Done():
			b.logger.Info("shutting down bot")
			return ctx.Err()
		}
	}
}

func (b *Bot) processPost(ctx context.Context, post jetstream.Post) {
	triggerTag := b.cfg.Translation.TriggerHashtag

	// Check for trigger tag
	if !filter.HasTriggerTag(post.Facets, triggerTag) {
		return
	}

	// Skip replies (MVP)
	if filter.IsReply(post.Record) {
		b.logger.Debug("skipping reply", "did", post.DID, "rkey", post.RKey)
		return
	}

	// Check if already processed
	uri := buildPostURI(post.DID, post.RKey)
	if b.store.IsProcessed(uri) {
		b.logger.Debug("already processed", "uri", uri)
		return
	}

	// Remove trigger tag from text
	cleanText := filter.RemoveTriggerTag(post.Text, post.Facets, triggerTag)
	b.logger.Info("processing post", "uri", uri, "text", cleanText)

	// Translate to all target languages
	translations, errs := b.translator.TranslateAll(
		ctx,
		cleanText,
		b.cfg.Translation.SourceLanguage,
		b.cfg.Translation.TargetLanguages,
	)
	for _, err := range errs {
		b.logger.Error("translation error", "error", err)
	}

	if len(translations) == 0 {
		b.logger.Error("all translations failed, skipping post", "uri", uri)
		return
	}

	// Post replies
	original := bluesky.OriginalPost{URI: uri, CID: post.CID}
	postErrs := b.poster.PostAll(ctx, original, translations, 3)
	for _, err := range postErrs {
		b.logger.Error("posting error", "error", err)
	}

	// Mark as processed
	langs := make([]string, 0, len(translations))
	for lang := range translations {
		langs = append(langs, lang)
	}
	if err := b.store.MarkProcessed(uri, langs); err != nil {
		b.logger.Error("mark processed failed", "uri", uri, "error", err)
	}
}

func buildPostURI(did, rkey string) string {
	return "at://" + did + "/app.bsky.feed.post/" + rkey
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/bot/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/bot/
git commit -m "feat: add bot orchestrator with event pipeline"
```

---

## Chunk 6: CLI & Build Artifacts

### Task 11: CLI (main.go)

**Files:**
- Modify: `cmd/ddarabot/main.go`

- [ ] **Step 1: Write full main.go**

```go
// cmd/ddarabot/main.go
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/compat_oai/anthropic"
	"github.com/firebase/genkit/go/plugins/compat_oai/openai"
	"github.com/firebase/genkit/go/plugins/googlegenai"
	"github.com/firebase/genkit/go/plugins/ollama"
	"github.com/openai/openai-go/option"

	"github.com/huketo/ddarabot/internal/bluesky"
	"github.com/huketo/ddarabot/internal/bot"
	"github.com/huketo/ddarabot/internal/config"
	"github.com/huketo/ddarabot/internal/store"
	"github.com/huketo/ddarabot/internal/translator"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version":
			fmt.Printf("ddarabot %s\n", version)
			return
		case "validate":
			os.Args = append(os.Args[:1], os.Args[2:]...)
			runValidate()
			return
		}
	}

	runBot()
}

func runBot() {
	fs := flag.NewFlagSet("ddarabot", flag.ExitOnError)
	configPath := fs.String("config", "config.toml", "path to config file")
	shortConfig := fs.String("c", "config.toml", "path to config file (short)")
	dryRun := fs.Bool("dry-run", false, "translate but do not post replies")
	fs.Parse(os.Args[1:])

	cfgPath := *configPath
	if *shortConfig != "config.toml" {
		cfgPath = *shortConfig
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	logger := setupLogger(cfg.Log.Level)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	g, err := initGenkit(ctx, &cfg.LLM)
	if err != nil {
		logger.Error("genkit init failed", "error", err)
		os.Exit(1)
	}

	st, err := store.New(cfg.Store.Path)
	if err != nil {
		logger.Error("store init failed", "error", err)
		os.Exit(1)
	}
	defer st.Close()

	auth := bluesky.NewAuth(cfg.Bluesky.PDSHost, cfg.Bluesky.Handle, cfg.Bluesky.AppPassword)
	poster := bluesky.NewPoster(auth, cfg.Bluesky.PDSHost, logger, *dryRun)
	tr := translator.New(g, cfg.LLM.Model, cfg.Translation.Footer, logger)

	b := bot.New(cfg, st, tr, poster, logger)

	logger.Info("starting ddarabot",
		"version", version,
		"model", cfg.LLM.Model,
		"targets", cfg.Translation.TargetLanguages,
		"dry-run", *dryRun,
	)

	if err := b.Run(ctx); err != nil && err != context.Canceled {
		logger.Error("bot stopped", "error", err)
		os.Exit(1)
	}

	logger.Info("ddarabot stopped gracefully")
}

func runValidate() {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	configPath := fs.String("config", "config.toml", "path to config file")
	shortConfig := fs.String("c", "config.toml", "path to config file (short)")
	fs.Parse(os.Args[1:])

	cfgPath := *configPath
	if *shortConfig != "config.toml" {
		cfgPath = *shortConfig
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("config OK")

	ctx := context.Background()
	g, err := initGenkit(ctx, &cfg.LLM)
	if err != nil {
		fmt.Fprintf(os.Stderr, "genkit init error: %v\n", err)
		os.Exit(1)
	}

	tr := translator.New(g, cfg.LLM.Model, cfg.Translation.Footer, slog.Default())
	results, errs := tr.TranslateAll(ctx, "Hello", cfg.Translation.SourceLanguage, cfg.Translation.TargetLanguages[:1])
	if len(errs) > 0 {
		fmt.Fprintf(os.Stderr, "LLM test error: %v\n", errs[0])
		os.Exit(1)
	}
	for lang, text := range results {
		fmt.Printf("LLM test OK (%s): %s\n", lang, text)
	}
}

func initGenkit(ctx context.Context, cfg *config.LLM) (*genkit.Genkit, error) {
	provider := cfg.Provider()

	var plugin genkit.Plugin
	switch provider {
	case "openai":
		plugin = &openai.OpenAI{APIKey: cfg.OpenAI.APIKey}
	case "anthropic":
		plugin = &anthropic.Anthropic{
			Opts: []option.RequestOption{option.WithAPIKey(cfg.Anthropic.APIKey)},
		}
	case "googleai":
		plugin = &googlegenai.GoogleAI{APIKey: cfg.GoogleAI.APIKey}
	case "vertexai":
		plugin = &googlegenai.VertexAI{ProjectID: cfg.VertexAI.ProjectID, Location: cfg.VertexAI.Location}
	case "ollama":
		plugin = &ollama.Ollama{ServerAddress: cfg.Ollama.ServerAddress, Timeout: cfg.Ollama.Timeout}
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %q (from model %q)", provider, cfg.Model)
	}

	g := genkit.Init(ctx, genkit.WithPlugins(plugin))
	return g, nil
}

func setupLogger(level string) *slog.Logger {
	var logLevel slog.Level
	switch strings.ToLower(level) {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel}))
}
```

- [ ] **Step 2: Verify build**

Run: `make build && ./bin/ddarabot version`
Expected: `ddarabot dev` (or git hash)

- [ ] **Step 3: Commit**

```bash
git add cmd/ddarabot/main.go
git commit -m "feat: add CLI with run/validate/version commands"
```

---

### Task 12: Build Artifacts

**Files:**
- Create: `config.example.toml`
- Create: `Dockerfile`
- Create: `docker-compose.yml`

- [ ] **Step 1: Create config.example.toml**

```toml
# DDaraBot 설정 파일
# 이 파일을 config.toml로 복사하고 값을 채워주세요.
# cp config.example.toml config.toml

[bluesky]
handle = "my-handle.bsky.social"
app_password = "xxxx-xxxx-xxxx-xxxx"      # 환경변수: DDARA_BLUESKY_APP_PASSWORD
pds_host = "https://bsky.social"

[jetstream]
url = "wss://jetstream2.us-east.bsky.network/subscribe"
watched_dids = [
  "did:plc:my-did-here",
]

[translation]
source_language = "ko"                     # BCP 47
target_languages = ["en", "ja", "zh"]      # BCP 47
trigger_hashtag = "ddara"
summarize_on_overflow = true
footer = "\n\n🌐 Translated by #DDaraBot"

[llm]
# provider/model 형식. provider는 자동 결정됨.
# 지원: openai/, anthropic/, googleai/, ollama/, vertexai/
model = "googleai/gemini-2.5-flash"

[llm.openai]
api_key = ""                               # 환경변수: OPENAI_API_KEY

[llm.anthropic]
api_key = ""                               # 환경변수: ANTHROPIC_API_KEY

[llm.googleai]
api_key = ""                               # 환경변수: GOOGLE_API_KEY

[llm.ollama]
server_address = "http://localhost:11434"
timeout = 60

[llm.vertexai]
project_id = ""
location = "us-central1"

[store]
path = "./ddarabot.db"

[log]
level = "info"
```

- [ ] **Step 2: Create Dockerfile**

```dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o ddarabot ./cmd/ddarabot/

FROM alpine:3.21
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/ddarabot .
VOLUME ["/app/data"]
ENTRYPOINT ["./ddarabot"]
CMD ["--config", "/app/data/config.toml"]
```

- [ ] **Step 3: Create docker-compose.yml**

```yaml
services:
  ddarabot:
    image: ddarabot:latest
    build: .
    restart: unless-stopped
    volumes:
      - ./data:/app/data
    environment:
      - TZ=Asia/Seoul
      # - DDARA_BLUESKY_APP_PASSWORD=xxxx
      # - OPENAI_API_KEY=sk-...
      # - ANTHROPIC_API_KEY=sk-ant-...
      # - GOOGLE_API_KEY=AI...
```

- [ ] **Step 4: Verify full test suite**

Run: `go test ./... -v`
Expected: All packages PASS

- [ ] **Step 5: Verify Docker build**

Run: `docker build -t ddarabot:dev .`
Expected: Build succeeds

- [ ] **Step 6: Commit**

```bash
git add config.example.toml Dockerfile docker-compose.yml
git commit -m "feat: add build artifacts (Dockerfile, docker-compose, config example)"
```

---

### Task 13: Final Verification

- [ ] **Step 1: Run full test suite**

Run: `make test`
Expected: All PASS

- [ ] **Step 2: Build binary**

Run: `make build`
Expected: `bin/ddarabot` created

- [ ] **Step 3: Test version command**

Run: `./bin/ddarabot version`
Expected: version string printed

- [ ] **Step 4: Test validate with example config (should fail on empty API key)**

Run: `cp config.example.toml config.toml && ./bin/ddarabot validate -c config.toml`
Expected: Error about LLM connection (expected since no real API key)

- [ ] **Step 5: Clean up test config**

Run: `rm -f config.toml`

- [ ] **Step 6: Final commit with any fixes**

```bash
git add -A
git commit -m "chore: final MVP verification and cleanup"
```
