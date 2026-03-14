# DDaraBot (따라봇) - 설계 명세서

> 내 Bluesky 포스트를 다국어로 자동 번역해주는 Go 기반 오픈소스 개인 도구
>
> "따라봇(DDaraBot)" — 따라 + 로봇, 내 포스트를 따라가며 다국어로 확장해주는 셀프 번역 봇
>
> **컨셉:** 별도 봇 계정이 아닌, 내 계정의 앱 비밀번호를 사용하여
> 내가 쓴 포스트에 내 계정으로 번역 답글을 다는 방식

---

## 1. 프로젝트 개요

| 항목 | 내용 |
|------|------|
| **프로젝트명** | DDaraBot (따라봇) |
| **Go 모듈** | `github.com/huketo/ddarabot` |
| **개발 언어** | Go |
| **라이선스** | MIT |
| **버전 전략** | v0.1 MVP → 점진적 기능 추가 |
| **배포 형태** | 단일 바이너리 + Docker 이미지 |

### 핵심 기능

- 내 Bluesky 계정에서 `#ddara` 해시태그가 포함된 포스트를 감지
- Genkit을 이용해 설정된 타겟 언어들로 번역
- 내 계정으로 원본 포스트에 번역 답글(reply)을 자동 게시
- 번역 답글 푸터에 `#DDaraBot` 태그(facet) 포함

---

## 2. 핵심 설계 결정

| 결정 사항 | 선택 | 근거 |
|-----------|------|------|
| LLM 통합 | Genkit 전면 채택 | 통합 API로 모든 프로바이더 지원, 프로바이더별 코드 불필요 |
| Genkit 활용 범위 | LLM 호출 전용 | 단순 파이프라인이므로 Flow/observability 불필요, Go 관용적 패턴 유지 |
| AT Protocol 클라이언트 | XRPC 직접 구현 (`net/http`) | 3개 엔드포인트뿐이라 경량 구현 가능, indigo 무거운 의존성 회피 |
| Jetstream 클라이언트 | `bluesky-social/jetstream/pkg/client` | 공식, 경량, indigo 독립 |
| 원문 언어 | 설정 파일에서 고정 (BCP 47) | 자동 감지 불필요, 단순성 우선 |
| 푸터 이모지 | 🌐 (지구본) 통일 | 언어→국기 매핑이 부적절 |
| 동시성 | goroutine + channel 기반 fan-out | Go 특성 활용, 멀티 언어 번역/포스팅 병렬 처리, 부분 성공 허용 |

---

## 3. 아키텍처

```
┌─────────────────────────────────────────────────────┐
│                    DDaraBot                          │
│                                                     │
│  ┌──────────────┐  ┌───────────┐  ┌──────────────┐ │
│  │  Jetstream    │─▶│  Filter   │─▶│  Translator  │ │
│  │  Client       │  │  Engine   │  │  (Genkit)    │ │
│  │  (공식 pkg)   │  └───────────┘  └──────┬───────┘ │
│  └──────────────┘                         │         │
│                                           │         │
│  ┌──────────────┐  ┌───────────┐  ┌───────▼──────┐ │
│  │   BoltDB     │◀─│   State   │◀─│   Poster     │ │
│  │  (중복방지)   │─▶│  Manager  │─▶│ (XRPC 직접)  │ │
│  └──────────────┘  └───────────┘  └──────────────┘ │
│                                                     │
│  ┌──────────────────────────────────────────────┐   │
│  │  Config (TOML) + Genkit Init                 │   │
│  └──────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────┘
```

### 데이터 흐름

```
[Jetstream WebSocket] (bluesky-social/jetstream/pkg/client)
       │
       ▼
 1. 이벤트 수신: wantedDids + collection=app.bsky.feed.post
       │
       ▼
 2. Filter: facets에서 #ddara 태그 감지
       │
       ▼
 3. State: BoltDB에서 중복 확인 (post URI 기준)
       │
       ▼
 4. 전처리: 원문에서 #ddara 태그 텍스트 제거
       │
       ▼
 5. Translator: 각 타겟 언어별 genkit.Generate() 병렬 호출 (errgroup)
       │  ├─ 번역 결과 + 푸터 ≤ 300자 → 사용
       │  └─ 300자 초과 → genkit.Generate()로 요약 재요청
       │
       ▼
 6. Poster: XRPC createRecord로 답글 병렬 게시 (semaphore 제한)
       │  └─ 푸터: "{번역문}\n\n🌐 Translated by #DDaraBot"
       │
       ▼
 7. State: BoltDB에 처리 완료 기록
```

---

## 4. 모듈 구조

```
ddarabot/
├── cmd/
│   └── ddarabot/
│       └── main.go              # 엔트리포인트, Genkit Init, 시그널 핸들링
├── internal/
│   ├── config/
│   │   └── config.go            # TOML 설정 로딩 + 환경변수 오버라이드
│   ├── bluesky/
│   │   ├── auth.go              # 세션 생성/갱신 (XRPC 직접)
│   │   ├── poster.go            # 답글 생성 (XRPC createRecord 직접)
│   │   └── types.go             # XRPC 요청/응답 구조체
│   ├── jetstream/
│   │   └── listener.go          # 공식 Jetstream client 래핑, 재연결 관리
│   ├── filter/
│   │   └── filter.go            # #ddara 감지, 태그 제거, 텍스트 전처리
│   ├── translator/
│   │   ├── translator.go        # Genkit 기반 번역 (Generate 호출)
│   │   └── prompt.go            # 번역/요약 프롬프트 템플릿
│   ├── store/
│   │   └── store.go             # BoltDB 중복 방지 + 커서 저장
│   └── bot/
│       └── bot.go               # 메인 오케스트레이션 (파이프라인 연결)
├── config.example.toml
├── Dockerfile
├── Makefile
├── go.mod
└── go.sum
```

---

## 5. CLI 설계

```
# 기본 실행 (서브커맨드 없이 바로 데몬 시작)
ddarabot --config config.toml

# 설정 파일 검증만
ddarabot validate --config config.toml

# 버전 확인
ddarabot version
```

| 플래그 | 축약 | 기본값 | 설명 |
|--------|------|--------|------|
| `--config` | `-c` | `config.toml` | 설정 파일 경로 |
| `--dry-run` | | `false` | 번역은 하되 실제 답글 게시 안 함 (로그만 출력) |

| 서브커맨드 | 설명 |
|------------|------|
| (없음) | 봇 실행 (기본 동작) |
| `validate` | 설정 파일 파싱 + LLM 간단 호출 테스트 (짧은 텍스트 번역 1회) 후 종료 |
| `version` | 버전 정보 출력 |

구현: `os.Args` + 표준 `flag` 패키지 (서브커맨드 2개뿐이라 cobra 불필요)

---

## 6. 설정 파일 (TOML)

```toml
# DDaraBot 설정 파일

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
# model 이름에서 프로바이더 자동 결정 (provider/model 형식)
model = "googleai/gemini-2.5-flash"

# 사용하는 프로바이더 섹션만 채우면 됨
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
level = "info"                             # debug, info, warn, error
# 로깅: log/slog (Go 표준 라이브러리) 사용
```

### 환경변수 오버라이드

| 환경변수 | 오버라이드 대상 |
|----------|----------------|
| `DDARA_BLUESKY_APP_PASSWORD` | `bluesky.app_password` |
| `OPENAI_API_KEY` | `llm.openai.api_key` |
| `ANTHROPIC_API_KEY` | `llm.anthropic.api_key` |
| `GOOGLE_API_KEY` | `llm.googleai.api_key` |

Ollama와 Vertex AI는 API 키 방식이 아니므로 환경변수 오버라이드 불필요.
(Vertex AI는 Application Default Credentials 사용)

---

## 7. 핵심 컴포넌트 상세 설계

### 7.1 Jetstream Listener (`internal/jetstream/listener.go`)

- `bluesky-social/jetstream/pkg/client` 공식 클라이언트 래핑
- `wantedDids` + `wantedCollections=app.bsky.feed.post`로 필터링
- 이벤트를 버퍼 채널(`chan Event`, 버퍼 64)로 전송
- 재연결: 지수 백오프 (1s → 2s → 4s → ... → max 60s)
- 커서(unix microseconds)를 BoltDB에 저장, 재시작 시 이어받기
- 커서 저장 주기: 이벤트 100건마다 또는 30초 간격 중 먼저 도달하는 조건. Graceful shutdown 시에도 마지막 커서 저장.

```
연결 URL:
wss://jetstream2.us-east.bsky.network/subscribe
  ?wantedCollections=app.bsky.feed.post
  &wantedDids=did:plc:xxx
```

### 7.2 Filter Engine (`internal/filter/filter.go`)

- `record.facets`에서 `app.bsky.richtext.facet#tag` 타입의 `tag == "ddara"` 감지 (대소문자 무시)
- facet의 `byteStart`/`byteEnd`로 원문에서 `#ddara` 텍스트 제거
- 해시태그는 번역하지 않음 (원문 그대로 유지)
- `record.reply` 필드가 있는 포스트는 무시 (MVP에서는 답글 번역 미지원)

### 7.3 Translator (`internal/translator/translator.go`, `prompt.go`)

Genkit `genkit.Generate()` 기반. 프로바이더별 구현 없이 단일 코드로 모든 LLM 지원.

**Genkit 초기화 (`cmd/ddarabot/main.go`):**

`config.llm.model`에서 `/` 앞부분으로 프로바이더를 판별하고, 해당 플러그인만 등록:

```go
import (
    "github.com/firebase/genkit/go/genkit"
    "github.com/firebase/genkit/go/plugins/compat_oai/openai"
    "github.com/firebase/genkit/go/plugins/compat_oai/anthropic"
    "github.com/firebase/genkit/go/plugins/googlegenai"
    "github.com/firebase/genkit/go/plugins/ollama"
    "github.com/openai/openai-go/option"
)

func initGenkit(ctx context.Context, cfg config.LLM) (*genkit.Genkit, error) {
    provider := strings.SplitN(cfg.Model, "/", 2)[0]

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
        return nil, fmt.Errorf("unsupported provider: %s", provider)
    }

    g := genkit.Init(ctx, genkit.WithPlugins(plugin))
    return g, nil
}
```

**Translator 구조체:**

```go
type Translator struct {
    g     *genkit.Genkit  // Genkit 인스턴스 참조
    model string          // "provider/model" 형식
}
```

**번역 프롬프트 (`ai.WithSystem` + `ai.WithPrompt` 사용):**

시스템 프롬프트:
```
You are a professional translator. Translate the following social media post
from {source_lang} to {target_lang}.

Rules:
- Keep the tone and nuance of the original
- Preserve @mentions, URLs, and emoji as-is
- Do not add explanations or notes
- Output only the translated text
- Keep hashtags in their original language (do not translate hashtags)
```

300 grapheme 초과 시 요약 시스템 프롬프트:
```
You are a professional translator. The following social media post needs to be
translated from {source_lang} to {target_lang} and condensed to fit within
{max_chars} graphemes.

Rules:
- Preserve the core meaning
- Keep the tone natural for social media
- Output only the translated and condensed text
```

**Genkit Generate 호출 패턴:**
```go
func (t *Translator) translate(ctx context.Context, text, sourceLang, targetLang string) (string, error) {
    resp, err := genkit.Generate(ctx, t.g,
        ai.WithModelName(t.model),
        ai.WithSystem(buildTranslateSystemPrompt(sourceLang, targetLang)),
        ai.WithPrompt(text),
    )
    if err != nil {
        return "", err
    }
    return resp.Text(), nil
}
```

**번역 파이프라인 (언어별 goroutine으로 병렬 실행):**

300 grapheme 제한은 Unicode grapheme cluster 단위 (`rivo/uniseg` 사용):

```
원문 텍스트
    │
    ▼
[1단계] genkit.Generate(translate prompt)
    │
    ├─ 결과 + 푸터 ≤ 300 graphemes → 완료
    │
    └─ 결과 + 푸터 > 300 graphemes
         │
         ▼
    [2단계] genkit.Generate(summarize prompt, maxGraphemes = 300 - 푸터길이)
         │
         ▼
       결과 + 푸터 → 완료
```

**멀티 언어 동시 번역 (개별 에러 수집, 부분 성공 허용):**
```go
func (t *Translator) TranslateAll(ctx context.Context, text, sourceLang string, targetLangs []string) (map[string]string, []error) {
    type result struct {
        lang       string
        translated string
        err        error
    }

    ch := make(chan result, len(targetLangs))
    for _, lang := range targetLangs {
        go func() {
            translated, err := t.translate(ctx, text, sourceLang, lang)
            ch <- result{lang: lang, translated: translated, err: err}
        }()
    }

    results := make(map[string]string)
    var errs []error
    for range targetLangs {
        r := <-ch
        if r.err != nil {
            errs = append(errs, fmt.Errorf("translate to %s: %w", r.lang, r.err))
        } else {
            results[r.lang] = r.translated
        }
    }
    return results, errs
}
```

**재시도:** 각 `translate()` 호출 내부에서 3회 재시도 (지수 백오프). 실패 시 해당 언어만 스킵.

### 7.4 Poster (`internal/bluesky/poster.go`, `auth.go`, `types.go`)

**인증 (`auth.go`):**
- XRPC `com.atproto.server.createSession` / `com.atproto.server.refreshSession` 직접 호출
- accessJwt 만료 시 refreshJwt로 갱신, 실패 시 createSession 재인증

**답글 게시 (`poster.go`):**
- XRPC `com.atproto.repo.createRecord`로 답글 게시
- `reply.root`/`reply.parent` 설정
- `langs` 필드에 타겟 언어 코드 설정 (BCP 47)
- 푸터의 `#DDaraBot`은 `app.bsky.richtext.facet#tag` facet으로 생성 (byteStart/byteEnd 계산)

**답글 포맷:**
```
{번역된 텍스트}

🌐 Translated by #DDaraBot
```

**동시 포스팅 (semaphore 제한, 부분 성공 허용):**
```go
func (p *Poster) PostAll(ctx context.Context, original Post, translations map[string]string) []error {
    type result struct {
        lang string
        err  error
    }

    sem := make(chan struct{}, p.maxConcurrent)  // 설정 가능 (기본값 3)
    ch := make(chan result, len(translations))

    for lang, text := range translations {
        go func() {
            sem <- struct{}{}
            defer func() { <-sem }()
            ch <- result{lang: lang, err: p.postReply(ctx, original, lang, text)}
        }()
    }

    var errs []error
    for range translations {
        r := <-ch
        if r.err != nil {
            errs = append(errs, fmt.Errorf("post %s reply: %w", r.lang, r.err))
        }
    }
    return errs
}
```

### 7.5 State Manager (`internal/store/store.go`)

BoltDB 버킷 구조 (값은 `encoding/json`으로 직렬화):
```
Bucket: "processed_posts"
  Key:   "at://did:plc:xxx/app.bsky.feed.post/rkey"
  Value: {"timestamp": 1725516665, "languages": ["en","ja","zh"]}

Bucket: "cursor"
  Key:   "jetstream_cursor"
  Value: "1725516665333808"  (plain string)
```

### 7.6 Bot Orchestrator (`internal/bot/bot.go`)

모든 컴포넌트를 연결하는 메인 루프:

```
Jetstream Listener (goroutine)
       │
       │ chan Event (버퍼 64)
       ▼
Bot 이벤트 소비 루프
       │
       ├─ Filter: #ddara 확인
       ├─ Store: 중복 확인
       ├─ Translator: goroutine + channel fan-out 병렬 번역
       ├─ Poster: goroutine + semaphore 제한 병렬 게시
       └─ Store: 처리 완료 기록
```

---

## 8. 동시성 설계

### 전체 파이프라인

```
Jetstream Listener (goroutine)
       │
       │ chan Event (버퍼 채널, 64)
       ▼
Bot Orchestrator (이벤트 소비 루프)
       │
       ├─ Filter: #ddara 확인
       ├─ Store: 중복 확인
       │
       ▼
  Translator (goroutine + channel fan-out, 부분 성공 허용)
       │
       ├─ goroutine: genkit.Generate → "en"
       ├─ goroutine: genkit.Generate → "ja"
       └─ goroutine: genkit.Generate → "zh"
       │
       │ 모든 번역 완료 대기
       ▼
  Poster (goroutine + semaphore 제한, 부분 성공 허용)
       │
       ├─ goroutine: createRecord → en 답글
       ├─ goroutine: createRecord → ja 답글
       └─ goroutine: createRecord → zh 답글
       │
       ▼
  Store: 처리 완료 기록
```

### Graceful Shutdown

```go
ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
defer cancel()
```

ctx 취소 시:
- Jetstream listener: WebSocket 닫고 종료
- 진행 중 번역: context 취소로 LLM 호출 중단
- 진행 중 포스팅: 현재 요청 완료 후 종료
- BoltDB: 정상 Close

---

## 9. 에러 처리

| 상황 | 동작 |
|------|------|
| 번역 1개 언어 실패 | 해당 언어만 스킵, 나머지 정상 게시 + 로그 |
| 번역 전체 실패 | 해당 포스트 스킵 + 에러 로그 |
| 포스팅 실패 | 3회 재시도 (지수 백오프), 429 시 Retry-After 준수 |
| Jetstream 연결 끊김 | 지수 백오프 재연결 (max 60s), 커서 기반 이어받기 |
| 세션 만료 | refreshJwt → 실패 시 createSession 재인증 |
| LLM API 실패 | 3회 재시도 (지수 백오프), 실패 시 해당 언어 스킵 후 로그 |

### 무한루프 방지

봇이 내 계정으로 답글을 달기 때문에 Jetstream에서 봇이 작성한 답글도 다시 수신됨. DID 비교만으로는 구분 불가.

방지 전략:
- **답글 무시:** `record.reply` 필드가 있는 포스트는 무시 (MVP)
- **처리 기록 확인:** BoltDB에 이미 기록된 포스트 URI는 스킵
- **해시태그 부재:** 번역 답글에는 `#ddara`를 포함하지 않으므로 트리거되지 않음

---

## 10. 빌드 및 배포

### Makefile

```makefile
APP_NAME := ddarabot
VERSION  := $(shell git describe --tags --always --dirty)

.PHONY: build run test clean docker

build:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/$(APP_NAME) ./cmd/ddarabot/

run:
	go run ./cmd/ddarabot/ --config config.toml

test:
	go test ./...

clean:
	rm -rf bin/

docker:
	docker build -t $(APP_NAME):$(VERSION) .
```

### Dockerfile

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

### Docker Compose

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

---

## 11. 의존성

| 패키지 | 용도 |
|--------|------|
| `github.com/bluesky-social/jetstream/pkg/client` | Jetstream WebSocket 구독 |
| `github.com/firebase/genkit/go/genkit` | Genkit 코어 |
| `github.com/firebase/genkit/go/ai` | Genkit AI 인터페이스 |
| `github.com/firebase/genkit/go/plugins/compat_oai/openai` | OpenAI 플러그인 |
| `github.com/firebase/genkit/go/plugins/compat_oai/anthropic` | Anthropic 플러그인 |
| `github.com/firebase/genkit/go/plugins/googlegenai` | Google AI / Vertex AI 플러그인 |
| `github.com/firebase/genkit/go/plugins/ollama` | Ollama 플러그인 |
| `github.com/BurntSushi/toml` | TOML 설정 파일 파싱 |
| `go.etcd.io/bbolt` | BoltDB 키-값 저장소 |
| `golang.org/x/sync/errgroup` | 동시성 에러 그룹 |
| `github.com/rivo/uniseg` | Unicode grapheme cluster 카운팅 (300 grapheme 제한) |

XRPC 통신은 `net/http` + `encoding/json` 표준 라이브러리로 직접 구현.
로깅은 `log/slog` (Go 1.21+ 표준 라이브러리) 사용.

---

## 12. 보안 고려사항

- **앱 비밀번호** 유출 시 내 계정으로 포스트 가능 — 설정 파일 퍼미션 `600` 권장
- 앱 비밀번호는 비밀번호 변경/2FA 설정 불가하여 피해 범위 제한
- Docker 사용 시 secrets 또는 환경변수 주입
- BoltDB 파일은 봇 프로세스만 접근 가능하도록 권한 설정
- LLM API 키는 환경변수 오버라이드 우선 사용 권장

---

## 13. MVP 스코프 (v0.1)

### 포함

- Jetstream 구독 및 `#ddara` 포스트 감지
- Genkit 기반 멀티 프로바이더 번역 (OpenAI, Anthropic, Google AI, Ollama, Vertex AI)
- 언어별 답글 병렬 게시
- BoltDB 중복 방지
- TOML 설정 파일
- 바이너리 빌드 + Docker 이미지
- 기본 텍스트 로그
- 300자 초과 시 요약 번역
- Graceful shutdown (SIGINT/SIGTERM)
- `--dry-run` 모드
- `validate` 서브커맨드

### v0.2 이후 로드맵

- 답글(reply) 번역 기능 (`translate_replies`)
- 미디어 처리 (`copy`, `translate` 모드)
- BoltDB 보관 기간 자동 정리 (`retention_days`)
- 설정 핫 리로드 (SIGHUP)
- 번역 품질 개선 (컨텍스트 활용 프롬프트)
- 웹 대시보드 (번역 통계, 상태 모니터링)

---

## 참고 문서

- Bluesky 개발자 문서: https://docs.bsky.app/docs/get-started
- Bluesky HTTP Reference: https://docs.bsky.app/docs/category/http-reference
- Genkit Go 시작하기: https://genkit.dev/docs/go/get-started
- Jetstream 저장소: https://github.com/bluesky-social/jetstream
