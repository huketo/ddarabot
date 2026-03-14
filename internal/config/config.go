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
	URL string `toml:"url"`
}

type Translation struct {
	SourceLanguage      string   `toml:"source_language"`
	TargetLanguages     []string `toml:"target_languages"`
	TriggerHashtag      string   `toml:"trigger_hashtag"`
	SummarizeOnOverflow bool     `toml:"summarize_on_overflow"`
	Footer              string   `toml:"footer"`
}

type LLM struct {
	Model     string          `toml:"model"`
	OpenAI    OpenAIConfig    `toml:"openai"`
	Anthropic AnthropicConfig `toml:"anthropic"`
	GoogleAI  GoogleAIConfig  `toml:"googleai"`
	Ollama    OllamaConfig    `toml:"ollama"`
	VertexAI  VertexAIConfig  `toml:"vertexai"`
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
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}
	return &cfg, nil
}

func (c *Config) Validate() error {
	if c.Bluesky.Handle == "" {
		return fmt.Errorf("bluesky.handle is required")
	}
	if c.Bluesky.AppPassword == "" {
		return fmt.Errorf("bluesky.app_password is required")
	}
	if !strings.HasPrefix(c.Bluesky.PDSHost, "https://") {
		return fmt.Errorf("bluesky.pds_host must start with https://")
	}
	if !strings.HasPrefix(c.Jetstream.URL, "wss://") {
		return fmt.Errorf("jetstream.url must start with wss://")
	}
	if c.LLM.Model == "" || !strings.Contains(c.LLM.Model, "/") {
		return fmt.Errorf("llm.model must be in provider/model format")
	}
	if len(c.Translation.TargetLanguages) == 0 {
		return fmt.Errorf("translation.target_languages must not be empty")
	}
	if c.Translation.SourceLanguage == "" {
		return fmt.Errorf("translation.source_language is required")
	}
	return nil
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
