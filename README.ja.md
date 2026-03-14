<p align="center">
  <h1 align="center">DDaraBot (따라봇)</h1>
  <p align="center">
    Blueskyの投稿を多言語に自動翻訳するセルフホスティングボット
  </p>
  <p align="center">
    <a href="README.md">English</a> · <a href="README.ko.md">한국어</a>
  </p>
</p>

---

> **DDaraBot** (따라봇) — *ddara*（따라、「ついていく」）+ *bot*（ロボット）
>
> あなたの投稿をフォローし、多言語に展開するボットです。

別途ボットアカウントは不要です。DDaraBotはあなた自身の[アプリパスワード](https://bsky.app/settings/app-passwords)を使用して、あなたのアカウントから翻訳リプライを投稿します。フォロワーからは、あなた自身が多言語でリプライしたように見えます。

## 仕組み

```
あなたの投稿: "오늘 날씨가 좋네요! #ddara"
  ↓ Jetstream WebSocketでリアルタイム検知
  ↓ Genkit LLMで翻訳

リプライ (en): "The weather is nice today! 🌐 Translated by #DDaraBot"
リプライ (ja): "今日はいい天気ですね！ 🌐 Translated by #DDaraBot"
リプライ (zh): "今天天气真好！ 🌐 Translated by #DDaraBot"
```

1. `#ddara` ハッシュタグを含む投稿を [Jetstream](https://github.com/bluesky-social/jetstream) でリアルタイム検知
2. [Genkit](https://genkit.dev/) を通じて設定されたターゲット言語に翻訳
3. あなたのアカウントから `#DDaraBot` タグ付きの翻訳リプライを自動投稿

## 対応LLMプロバイダー

[Genkit](https://genkit.dev/) の統一APIを通じて全プロバイダーに対応しています：

| プロバイダー | モデル形式の例 |
|-------------|---------------|
| OpenAI | `openai/gpt-4o-mini` |
| Anthropic | `anthropic/claude-sonnet-4-20250514` |
| Google AI | `googleai/gemini-2.5-flash` |
| Ollama | `ollama/llama3` |
| Vertex AI | `vertexai/gemini-2.5-flash` |

## クイックスタート

### 前提条件

- Bluesky [アプリパスワード](https://bsky.app/settings/app-passwords)
- LLM APIキー（選択したプロバイダーに応じて）

### Docker使用（推奨）

ビルド不要。設定ファイルを作成するだけですぐ実行できます：

```bash
# 1. 設定ファイルのダウンロード
mkdir -p data
curl -o data/config.toml https://raw.githubusercontent.com/huketo/ddarabot/main/config.example.toml

# 2. data/config.toml を編集（Blueskyハンドル、アプリパスワード、LLM APIキーを入力）

# 3. 実行
docker run -d --restart unless-stopped \
  -v ./data:/app/data \
  huketo/ddarabot:latest
```

### Docker Compose使用

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
# ./data/にconfig.tomlを配置してから：
docker compose up -d
```

### ソースからビルド

Go 1.24+が必要です。

```bash
git clone https://github.com/huketo/ddarabot.git
cd ddarabot
make build

cp config.example.toml config.toml
# config.tomlを編集

./bin/ddarabot --config config.toml
```

## 設定

`config.example.toml` を `config.toml` にコピーして値を入力してください。

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

> DIDは起動時に `bluesky.handle` から自動的にresolveされます — 手動で調べる必要はありません。

### 環境変数オーバーライド

機密情報は環境変数で注入できます：

| 変数 | 上書き対象 |
|------|-----------|
| `DDARA_BLUESKY_APP_PASSWORD` | `bluesky.app_password` |
| `OPENAI_API_KEY` | `llm.openai.api_key` |
| `ANTHROPIC_API_KEY` | `llm.anthropic.api_key` |
| `GOOGLE_API_KEY` | `llm.googleai.api_key` |

## CLI

```bash
ddarabot --config config.toml            # ボットを実行
ddarabot --config config.toml --dry-run  # 翻訳のみ、投稿しない（テストモード）
ddarabot validate --config config.toml   # 設定検証 + LLM接続テスト
ddarabot version                         # バージョン表示
```

## 開発

```bash
make build          # バイナリビルド
make test           # テスト実行
make lint           # gofmt + go vet チェック
make fmt            # コード自動フォーマット
make release        # 全プラットフォームクロスコンパイル
make docker-build   # Dockerイメージローカルビルド
make docker-deploy  # docker composeでデプロイ
make clean          # ビルド成果物の削除
```

## ライセンス

[MIT](LICENSE)
