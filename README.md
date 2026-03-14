<p align="center">
  <h1 align="center">DDaraBot (따라봇)</h1>
  <p align="center">
    A self-hosted Bluesky bot that auto-translates your posts into multiple languages
  </p>
  <p align="center">
    <a href="README.ko.md">한국어</a> · <a href="README.ja.md">日本語</a>
  </p>
</p>

---

> **DDaraBot** (따라봇) — *ddara* (따라, "follow along") + *bot* (로봇)
>
> A bot that follows your posts and expands them into multiple languages.

No separate bot account needed. DDaraBot uses your own [app password](https://bsky.app/settings/app-passwords) to post translated replies from your account — your followers see the translations as if you wrote them yourself.

## How It Works

```
Your post: "오늘 날씨가 좋네요! #ddara"
  ↓ Detected via Jetstream WebSocket
  ↓ Translated via Genkit LLM

Reply (en): "The weather is nice today! 🌐 Translated by #DDaraBot"
Reply (ja): "今日はいい天気ですね！ 🌐 Translated by #DDaraBot"
Reply (zh): "今天天气真好！ 🌐 Translated by #DDaraBot"
```

1. Detects your posts containing `#ddara` in real-time via [Jetstream](https://github.com/bluesky-social/jetstream)
2. Translates into configured target languages using [Genkit](https://genkit.dev/)
3. Posts translated replies from your account with `#DDaraBot` tag

## Supported LLM Providers

All providers are supported through [Genkit](https://genkit.dev/)'s unified API:

| Provider | Model Example |
|----------|---------------|
| OpenAI | `openai/gpt-4o-mini` |
| Anthropic | `anthropic/claude-sonnet-4-20250514` |
| Google AI | `googleai/gemini-2.5-flash` |
| Ollama | `ollama/llama3` |
| Vertex AI | `vertexai/gemini-2.5-flash` |

## Quick Start

### Prerequisites

- A Bluesky [app password](https://bsky.app/settings/app-passwords)
- An LLM API key (depending on your chosen provider)

### Using Docker (recommended)

No build required. Just create a config file and run:

```bash
# 1. Download the example config
mkdir -p data
curl -o data/config.toml https://raw.githubusercontent.com/huketo/ddarabot/main/config.example.toml

# 2. Edit data/config.toml with your Bluesky handle, app password, and LLM API key

# 3. Run
docker run -d --restart unless-stopped \
  -v ./data:/app/data \
  huketo/ddarabot:latest
```

### Using Docker Compose

```yaml
# docker-compose.yml
services:
  ddarabot:
    image: huketo/ddarabot:latest
    restart: unless-stopped
    volumes:
      - ./data:/app/data
    environment:
      - TZ=Asia/Seoul
```

```bash
# Place config.toml in ./data/, then:
docker compose up -d
```

### Build from Source

Requires Go 1.24+.

```bash
git clone https://github.com/huketo/ddarabot.git
cd ddarabot
make build

cp config.example.toml config.toml
# Edit config.toml

./bin/ddarabot --config config.toml
```

## Configuration

See [`config.example.toml`](config.example.toml) for the full reference.

```toml
[bluesky]
handle = "my-handle.bsky.social"
app_password = "xxxx-xxxx-xxxx-xxxx"

[translation]
source_language = "ko"
target_languages = ["en", "ja", "zh"]
trigger_hashtag = "ddara"

[llm]
model = "googleai/gemini-2.5-flash"

[llm.googleai]
api_key = "your-api-key"
```

> Your DID is automatically resolved from `bluesky.handle` at startup — no need to look it up manually.

### Environment Variable Overrides

Sensitive values can be injected via environment variables:

| Variable | Overrides |
|----------|-----------|
| `DDARA_BLUESKY_APP_PASSWORD` | `bluesky.app_password` |
| `OPENAI_API_KEY` | `llm.openai.api_key` |
| `ANTHROPIC_API_KEY` | `llm.anthropic.api_key` |
| `GOOGLE_API_KEY` | `llm.googleai.api_key` |

## CLI

```bash
ddarabot --config config.toml            # Run the bot
ddarabot --config config.toml --dry-run  # Translate without posting (test mode)
ddarabot validate --config config.toml   # Validate config + test LLM connection
ddarabot version                         # Print version
```

## Development

```bash
make build          # Build binary
make test           # Run tests
make lint           # Check gofmt + go vet
make fmt            # Auto-format code
make release        # Cross-compile for all platforms
make docker-build   # Build Docker image locally
make docker-deploy  # Deploy with docker compose
make clean          # Remove build artifacts
```

## License

[MIT](LICENSE)
