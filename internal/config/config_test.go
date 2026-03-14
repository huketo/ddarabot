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
