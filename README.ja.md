<div align="center">

<img src="assets/wordmark.png" alt="shioriai2api" width="640">

**[shiori.ai](https://www.shiori.ai) を OpenAI 互換 API に。**

OpenAI Chat Completions プロトコルを話す、軽量で依存ゼロの Go プロキシ。

[![GitHub stars](https://img.shields.io/github/stars/0xgetz/shioriai2api?style=for-the-badge&logo=github&color=966cff)](https://github.com/0xgetz/shioriai2api/stargazers)
[![GitHub forks](https://img.shields.io/github/forks/0xgetz/shioriai2api?style=for-the-badge&logo=github&color=ff6eaf)](https://github.com/0xgetz/shioriai2api/network/members)
[![License: MIT](https://img.shields.io/github/license/0xgetz/shioriai2api?style=for-the-badge&color=966cff)](LICENSE)

[![Go version](https://img.shields.io/github/go-mod/go-version/0xgetz/shioriai2api?style=for-the-badge&logo=go&color=00add8)](go.mod)
[![Zero dependencies](https://img.shields.io/badge/dependencies-0-brightgreen?style=for-the-badge)](go.mod)
[![Docker ready](https://img.shields.io/badge/docker-ready-2496ed?style=for-the-badge&logo=docker&logoColor=white)](Dockerfile)
[![PRs welcome](https://img.shields.io/badge/PRs-welcome-ff69b4?style=for-the-badge)](CONTRIBUTING.md)

[![English](https://img.shields.io/badge/lang-English-966cff?style=flat-square)](README.md)
[![Bahasa Indonesia](https://img.shields.io/badge/lang-Indonesia-ff6eaf?style=flat-square)](README.id.md)
[![中文](https://img.shields.io/badge/lang-中文-ff6b6b?style=flat-square)](README.zh.md)
[![日本語](https://img.shields.io/badge/lang-日本語-ffb454?style=flat-square)](README.ja.md)
[![Español](https://img.shields.io/badge/lang-Español-7c5cff?style=flat-square)](README.es.md)

</div>

---

## 概要

`shioriai2api` は **shiori.ai のウェブアプリ** を、クリーンで OpenAI 互換の
HTTP API の背後に包みます。標準ライブラリのみを使った純粋な Go で書かれて
いるため、ランタイム依存のない単一の静的バイナリとして配布されます。

任意の OpenAI クライアント — Cherry Studio、LobeChat、Open WebUI、公式
OpenAI SDK、`curl` — を接続するだけで、shiori.ai の **90 以上のモデル**
（GLM、Claude、GPT、Gemini、DeepSeek、Qwen、Kimi、Grok、MiniMax など）に
使い慣れた 1 つのエンドポイントからアクセスできます。

> **アカウントはご自身で用意してください。** ログイン済みブラウザセッション
> の認証情報を指定します。プロキシが外部に送信することはありません。

> **重要:** shiori.ai は Cloudflare の背後にあります。プロキシを
> **ブラウザと同じマシン**で実行すれば追加設定は不要です。別の IP で実行
> する場合は `SHIORI_COOKIE` と `SHIORI_PROXY` の設定が必要です — 下記の
> [Cloudflare](#cloudflare) を参照してください。

## 機能

- **OpenAI Chat Completions 互換** — `POST /v1/chat/completions`、ストリー
  ミングと非ストリーミングに対応。
- **ライブモデル一覧** — `GET /v1/models`、shiori.ai の実際のカタログ
  (`/api/models/catalog`) から提供。
- **推論サポート** — thinking 出力を `reasoning_content` にマッピングし、
  `reasoning_effort`（`max`…`none`）を透過します。
- **正確な使用量** — トークン数は上流の `finish` イベントから直接取得。
- **マルチアカウントのトークンプール** — アカウント間でラウンドロビン。
  認証情報はサーバー側に留まります。
- **マルチターンのコンテキスト** — 自動プレフィックスキャッシュが同じ
  shiori.ai 会話を再利用します。明示的な `conversation_id` パススルーも
  サポートします。
- **JWT の自動更新** — 長期の refresh token を短期 JWT に透過的に交換し、
  `401` の際に一度だけ再試行します。
- **Cloudflare 対応** — チャレンジ応答を分類し、修正すべき設定を明示します。
- **Docker / Docker Compose** 対応。

## 認証情報の取得

1. ブラウザで [shiori.ai](https://www.shiori.ai) にログインします。
2. DevTools（`F12`）→ **Console** を開き、次を実行します:

   ```js
   Object.keys(localStorage)
     .filter(k => k.startsWith('__convexAuthRefreshToken'))
     .map(k => localStorage.getItem(k))[0]
   ```

3. 出力をコピーします。これが **refresh token** です。プロキシが使用する
   長期の認証情報です（プロキシ自身が短期 JWT に交換します）。

> ブラウザと異なる IP で実行しますか? その場合は完全な cookie 文字列
> （`cf_clearance` を含む）を `SHIORI_COOKIE` にもコピーしてください。
> [Cloudflare](#cloudflare) を参照。

## クイックスタート

### ビルド

```bash
go build -o shioriai2api .
```

### 実行

単一アカウント:

```bash
PROXY_API_KEY='プロキシキー' SHIORI_REFRESH_TOKEN='refresh token' PORT=8080 ./shioriai2api
```

複数アカウント — 作業ディレクトリに `accounts.txt` を作成し、1 行に 1 つの
refresh token を記述します（空行と `#` コメントは無視されます）:

```
token1
token2
```

```bash
PROXY_API_KEY='プロキシキー' ./shioriai2api
```

### テスト

```bash
curl http://127.0.0.1:8080/v1/chat/completions \
  -H 'Authorization: Bearer プロキシキー' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "glm-5.3-flash",
    "messages": [{"role": "user", "content": "こんにちは"}],
    "stream": true
  }'
```

### ビルドの検証

自己完結型のスモークテストがモック上流とプロキシを起動し、ヘルスチェック、
認証、モデル一覧、ストリーミング/非ストリーミングのチャットを確認します:

```bash
./scripts/smoke_test.sh
```

## Cloudflare

shiori.ai は Cloudflare で保護されており、clearance cookie
（`cf_clearance`）は**チャレンジを解いたクライアントの IP アドレスと TLS
フィンガープリントにバインド**されています。これがデプロイのルールを決め
ます:

| プロキシの実行場所 | 必要なもの |
| --- | --- |
| ブラウザと同じマシン/IP | 追加設定なし。refresh token のみ。 |
| 別 IP の VPS やデータセンター | `SHIORI_COOKIE`（ブラウザの完全な cookie 文字列）**および** `SHIORI_PROXY`（cookie と一致する出口 IP のプロキシ）。 |

`SHIORI_COOKIE` はブラウザからの生の `name=value; name=value` 文字列です。
`SHIORI_PROXY` は HTTP(S) プロキシ URL、例: `http://user:pass@host:port`。

## Docker

```bash
docker build -t shioriai2api .

docker run --rm -p 8080:8080 \
  -e PROXY_API_KEY='プロキシキー' \
  -e SHIORI_REFRESH_TOKEN='refresh token' \
  shioriai2api
```

Compose を使う場合:

```bash
PROXY_API_KEY='プロキシキー' SHIORI_REFRESH_TOKEN='refresh token' docker compose up --build
```

## 設定

| 変数 | 既定値 | 説明 |
| --- | --- | --- |
| `PORT` | `8080` | ローカル HTTP ポート |
| `PROXY_API_KEY` | なし — **必須** | クライアントがプロキシを呼ぶ際のキー |
| `SHIORI_REFRESH_TOKEN` | 空 | shiori.ai の refresh token |
| `SHIORI_ACCOUNTS_FILE` | `accounts.txt` | 複数アカウント用ファイル、1 行 1 トークン |
| `SHIORI_COOKIE` | 空 | Cloudflare 用の Cookie ヘッダー（ブラウザ IP 以外では必須） |
| `SHIORI_PROXY` | 空 | 上流の HTTP(S) プロキシ URL |
| `SHIORI_BASE_URL` | `https://www.shiori.ai` | 上流のベース URL |
| `DEFAULT_MODEL` | `glm-5.3-flash` | リクエストがモデルを省略した場合に使うモデル |
| `CONVERSATION_TTL` | `30m` | 会話キーが忘れられるまでのアイドル時間 |
| `MAX_CONVERSATIONS` | `1024` | メモリに保持する最大会話数（LRU 淘汰） |

## マルチターン会話

- **自動モード（推奨）。** クライアントは通常どおり `messages` 配列全体を
  送ります。プロキシは履歴（最後のメッセージ以外）をフィンガープリントし、
  一致すれば同じ shiori.ai `chatId` を再利用して新しいターンだけを送信
  します。一致しなければ新しい会話を開始します。主要クライアントは変更
  なしで動作します。
- **パススルーモード。** レスポンスには非標準の `conversation_id` フィールド
  （shiori.ai のチャット id）が含まれます。後続のリクエストボディに送ると
  その会話を再利用します。

## 認証

すべての `/v1/*` リクエストには次が必要です:

```
Authorization: Bearer <PROXY_API_KEY>
```

`SHIORI_REFRESH_TOKEN` / `accounts.txt` はサーバー側でのみ使われ、呼び出し
元に公開されることはありません。

## 対応モデル

プロキシは shiori.ai のライブカタログを提供するため、一覧は自動的に最新の
状態に保たれます。例:

| グループ | 例 |
| --- | --- |
| Shiori | `openrouter/auto-beta` |
| OpenAI | `gpt-6-luna`, `gpt-6-astra`, `gpt-5.5`, `o3`, `gpt-4o` |
| Anthropic | `anthropic/claude-opus-5.5`, `anthropic/claude-sonnet-5.5` |
| Google | `gemini-3.7-flash`, `gemini-2.5-pro` |
| DeepSeek | `deepseek-v4-pro`, `deepseek-v4-flash` |
| Zhipu | `glm-5.3`, `glm-5.3-flash`, `glm-4.6` |
| Alibaba | `qwen/qwen3.8-27b`, `qwen/qwen3-coder` |
| Moonshot | `moonshotai/kimi-k3`, `moonshotai/kimi-k2-thinking` |
| xAI | `grok-4.6`, `grok-4.5` |
| MiniMax | `minimax/minimax-m3` |

## 注意事項と制限

- **使用量**は上流の `finish` イベント（`inputTokens`、`outputTokens`、
  `totalTokens`）から取得します。存在しない場合、completion トークンは
  返答の長さから推定します。
- 更新後の上流 `401` は `401` に、Cloudflare チャレンジは `502` にマッピング
  され、メッセージで修正方法を示します。
- ツール呼び出しと画像入力はまだ未接続です。テキストと推論は対応済みです。
- トークンを**コミットしないでください**。`accounts.txt` はすでに
  `.gitignore` に含まれています。

## プロジェクト構成

```
main.go                    HTTP サーバーとルーティング
config/                    環境設定
handlers/                  OpenAI 互換ハンドラ、SSE、会話
shiori/                    上流クライアント、認証、ストリーミング解析
internal/mockshiori/       エンドツーエンドテスト用モック上流
docs/api-analysis.md       上流 API のリバースエンジニアリング記録
scripts/                   アセット生成とスモークテストの補助
assets/                    ロゴ、ワードマーク、ソーシャルバナー
```

## ライセンス

[MIT ライセンス](LICENSE) の下で公開されています。

shiori.ai とは提携していません。責任を持って利用し、上流の規約を尊重して
ください。
