# DDaraBot (따라봇)

내 Bluesky 포스트를 다국어로 자동 번역해주는 Go 기반 셀프 번역 봇

> "따라봇" — 따라 + 로봇, 내 포스트를 따라가며 다국어로 확장해주는 봇

별도 봇 계정 없이, 내 계정의 앱 비밀번호를 사용하여 내가 쓴 포스트에 내 계정으로 번역 답글을 답니다.
팔로워 입장에서는 내가 직접 다국어 답글을 단 것처럼 보입니다.

## 동작 방식

1. `#ddara` 해시태그가 포함된 내 포스트를 Jetstream WebSocket으로 실시간 감지
2. Genkit LLM으로 설정된 타겟 언어들로 번역
3. 내 계정으로 원본 포스트에 번역 답글을 자동 게시

```
나의 포스트: "오늘 날씨가 좋네요! #ddara"
    ↓
자동 답글 (en): "The weather is nice today! 🌐 Translated by #DDaraBot"
자동 답글 (ja): "今日はいい天気ですね！ 🌐 Translated by #DDaraBot"
```

## 지원 LLM 프로바이더

[Genkit](https://genkit.dev/)을 통해 다음 프로바이더를 모두 지원합니다:

| 프로바이더 | 모델 형식 예시 |
|-----------|---------------|
| OpenAI | `openai/gpt-4o-mini` |
| Anthropic | `anthropic/claude-sonnet-4-20250514` |
| Google AI | `googleai/gemini-2.5-flash` |
| Ollama | `ollama/llama3` |
| Vertex AI | `vertexai/gemini-2.5-flash` |

## 빠른 시작

### 사전 요구사항

- Go 1.24+
- Bluesky 계정의 [앱 비밀번호](https://bsky.app/settings/app-passwords)
- LLM API 키 (사용할 프로바이더에 따라)

### 설치 및 실행

```bash
# 빌드
git clone https://github.com/huketo/ddarabot.git
cd ddarabot
make build

# 설정
cp config.example.toml config.toml
# config.toml을 편집하여 Bluesky 앱 비밀번호, DID, LLM API 키 등을 입력

# 실행
./bin/ddarabot --config config.toml
```

### Docker

```bash
# 빌드 및 실행
docker compose up -d

# 또는 직접 빌드
docker build -t ddarabot .
docker run -v ./data:/app/data ddarabot
```

## 설정

`config.example.toml`을 참고하여 `config.toml`을 작성합니다.

### 주요 설정 항목

```toml
[bluesky]
handle = "my-handle.bsky.social"
app_password = "xxxx-xxxx-xxxx-xxxx"

[jetstream]
watched_dids = ["did:plc:my-did-here"]

[translation]
source_language = "ko"
target_languages = ["en", "ja", "zh"]
trigger_hashtag = "ddara"

[llm]
model = "googleai/gemini-2.5-flash"

[llm.googleai]
api_key = "your-api-key"
```

### 환경변수 오버라이드

민감한 값은 환경변수로 주입할 수 있습니다:

| 환경변수 | 설정 항목 |
|----------|-----------|
| `DDARA_BLUESKY_APP_PASSWORD` | `bluesky.app_password` |
| `OPENAI_API_KEY` | `llm.openai.api_key` |
| `ANTHROPIC_API_KEY` | `llm.anthropic.api_key` |
| `GOOGLE_API_KEY` | `llm.googleai.api_key` |

## CLI

```bash
# 봇 실행
ddarabot --config config.toml

# 번역은 하되 실제 게시 안 함 (테스트용)
ddarabot --config config.toml --dry-run

# 설정 파일 검증 + LLM 연결 테스트
ddarabot validate --config config.toml

# 버전 확인
ddarabot version
```

## 개발

```bash
make build    # 바이너리 빌드
make test     # 테스트 실행
make lint     # gofmt + go vet 검사
make fmt      # gofmt 자동 포맷
make clean    # 빌드 산출물 삭제
```

## 라이선스

MIT
