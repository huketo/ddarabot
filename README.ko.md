<p align="center">
  <h1 align="center">DDaraBot (따라봇)</h1>
  <p align="center">
    내 Bluesky 포스트를 다국어로 자동 번역해주는 셀프 호스팅 봇
  </p>
  <p align="center">
    <a href="README.md">English</a> · <a href="README.ja.md">日本語</a>
  </p>
</p>

---

> **따라봇** — 따라 + 로봇, 내 포스트를 따라가며 다국어로 확장해주는 봇

별도 봇 계정 없이, 내 계정의 [앱 비밀번호](https://bsky.app/settings/app-passwords)를 사용하여 내 계정으로 번역 답글을 답니다. 팔로워 입장에서는 내가 직접 다국어 답글을 단 것처럼 보입니다.

## 동작 방식

```
나의 포스트: "오늘 날씨가 좋네요! #ddara"
  ↓ Jetstream WebSocket으로 실시간 감지
  ↓ Genkit LLM으로 번역

답글 (en): "The weather is nice today! 🌐 Translated by #DDaraBot"
답글 (ja): "今日はいい天気ですね！ 🌐 Translated by #DDaraBot"
답글 (zh): "今天天气真好！ 🌐 Translated by #DDaraBot"
```

1. `#ddara` 해시태그가 포함된 내 포스트를 [Jetstream](https://github.com/bluesky-social/jetstream)으로 실시간 감지
2. [Genkit](https://genkit.dev/)을 통해 설정된 타겟 언어들로 번역
3. 내 계정으로 원본 포스트에 `#DDaraBot` 태그가 포함된 번역 답글을 자동 게시

## 지원 LLM 프로바이더

[Genkit](https://genkit.dev/)의 통합 API를 통해 모든 프로바이더를 지원합니다:

| 프로바이더 | 모델 형식 예시 |
|-----------|---------------|
| OpenAI | `openai/gpt-4o-mini` |
| Anthropic | `anthropic/claude-sonnet-4-20250514` |
| Google AI | `googleai/gemini-2.5-flash` |
| Ollama | `ollama/llama3` |
| Vertex AI | `vertexai/gemini-2.5-flash` |

## 빠른 시작

### 사전 요구사항

- Bluesky [앱 비밀번호](https://bsky.app/settings/app-passwords)
- LLM API 키 (사용할 프로바이더에 따라)

### Docker 사용 (권장)

빌드 없이 설정 파일만 만들면 바로 실행할 수 있습니다:

```bash
# 1. 예제 설정 파일 다운로드
mkdir -p data
curl -o data/config.toml https://raw.githubusercontent.com/huketo/ddarabot/main/config.example.toml

# 2. data/config.toml 편집 (Bluesky 핸들, 앱 비밀번호, LLM API 키 입력)

# 3. 실행
docker run -d --restart unless-stopped \
  -v ./data:/app/data \
  huketo/ddarabot:latest
```

### Docker Compose 사용

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
# ./data/에 config.toml 배치 후:
docker compose up -d
```

### 소스에서 빌드

Go 1.24+ 필요.

```bash
git clone https://github.com/huketo/ddarabot.git
cd ddarabot
make build

cp config.example.toml config.toml
# config.toml 편집

./bin/ddarabot --config config.toml
```

## 설정

`config.example.toml`을 `config.toml`로 복사하고 값을 채워주세요.

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

> DID는 시작 시 `bluesky.handle`로부터 자동으로 resolve됩니다 — 직접 조회할 필요가 없습니다.

### 환경변수 오버라이드

민감한 값은 환경변수로 주입할 수 있습니다:

| 환경변수 | 오버라이드 대상 |
|----------|----------------|
| `DDARA_BLUESKY_APP_PASSWORD` | `bluesky.app_password` |
| `OPENAI_API_KEY` | `llm.openai.api_key` |
| `ANTHROPIC_API_KEY` | `llm.anthropic.api_key` |
| `GOOGLE_API_KEY` | `llm.googleai.api_key` |

## CLI

```bash
ddarabot --config config.toml            # 봇 실행
ddarabot --config config.toml --dry-run  # 번역만 하고 게시하지 않음 (테스트 모드)
ddarabot validate --config config.toml   # 설정 검증 + LLM 연결 테스트
ddarabot version                         # 버전 출력
```

## 개발

```bash
make build          # 바이너리 빌드
make test           # 테스트 실행
make lint           # gofmt + go vet 검사
make fmt            # 코드 자동 포맷
make release        # 전 플랫폼 크로스 컴파일
make docker-build   # Docker 이미지 로컬 빌드
make docker-deploy  # docker compose로 배포
make clean          # 빌드 산출물 삭제
```

## 라이선스

[MIT](LICENSE)
