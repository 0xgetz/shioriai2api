<div align="center">

<img src="assets/wordmark.png" alt="shioriai2api" width="640">

**Turn [shiori.ai](https://www.shiori.ai) into an OpenAI-compatible API.**

A lightweight, dependency-free Go proxy that speaks the OpenAI Chat Completions protocol.

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

## Overview

`shioriai2api` wraps the **shiori.ai web app** behind a clean,
OpenAI-compatible HTTP API. It is written in pure Go using only the standard
library, so it ships as a single static binary with no runtime dependencies.

Point any OpenAI client at it — Cherry Studio, LobeChat, Open WebUI, the
official OpenAI SDKs, `curl` — and reach shiori.ai's **90+ models** (GLM,
Claude, GPT, Gemini, DeepSeek, Qwen, Kimi, Grok, MiniMax and more) through one
familiar endpoint.

> **Bring your own account.** You supply the credentials from your own
> logged-in browser session. The proxy never phones home.

> **Important:** shiori.ai is behind Cloudflare. Run the proxy on the **same
> machine as your browser** and no extra configuration is needed. Running it on
> a different IP requires the `SHIORI_COOKIE` and `SHIORI_PROXY` settings — see
> [Cloudflare](#cloudflare) below.

## Features

- **OpenAI Chat Completions compatible** — `POST /v1/chat/completions`,
  streaming and non-streaming.
- **Live model listing** — `GET /v1/models`, served from shiori.ai's real
  catalog (`/api/models/catalog`).
- **Reasoning support** — thinking output is mapped to `reasoning_content`,
  with `reasoning_effort` passthrough (`max`…`none`).
- **Accurate usage** — token counts come straight from the upstream
  `finish` event.
- **Multi-account token pool** — round-robin across accounts; credentials stay
  server-side.
- **Multi-turn context** — automatic prefix caching reuses the same shiori.ai
  conversation; an explicit `conversation_id` passthrough is also supported.
- **Automatic JWT refresh** — a long-lived refresh token is exchanged for
  short-lived JWTs transparently, and retried once on `401`.
- **Cloudflare-aware** — classifies challenge responses and tells you exactly
  which setting to fix.
- **Docker / Docker Compose** ready.

## Getting your credentials

1. Log in to [shiori.ai](https://www.shiori.ai) in your browser.
2. Open DevTools (`F12`) → **Console** and run:

   ```js
   Object.keys(localStorage)
     .filter(k => k.startsWith('__convexAuthRefreshToken'))
     .map(k => localStorage.getItem(k))[0]
   ```

3. Copy the output — that is your **refresh token**. It is the long-lived
   credential the proxy uses (it exchanges it for short-lived JWTs itself).

> Running on a different IP than the browser? Also copy the full cookie
> string (including `cf_clearance`) into `SHIORI_COOKIE`. See
> [Cloudflare](#cloudflare).

## Quick start

### Build

```bash
go build -o shioriai2api .
```

### Run

Single account:

```bash
PROXY_API_KEY='your-proxy-key' SHIORI_REFRESH_TOKEN='your-refresh-token' PORT=8080 ./shioriai2api
```

Multiple accounts — create `accounts.txt` in the working directory, one
refresh token per line (blank lines and `#` comments are ignored):

```
token1
token2
```

```bash
PROXY_API_KEY='your-proxy-key' ./shioriai2api
```

### Test

```bash
curl http://127.0.0.1:8080/v1/chat/completions \
  -H 'Authorization: Bearer your-proxy-key' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "glm-5.3-flash",
    "messages": [{"role": "user", "content": "Hello"}],
    "stream": true
  }'
```

### Verify the build

A self-contained smoke test spins up a mock upstream and the proxy, then
checks health, auth, model listing, and streaming/non-streaming chat:

```bash
./scripts/smoke_test.sh
```

## Cloudflare

shiori.ai is protected by Cloudflare, and its clearance cookie
(`cf_clearance`) is **bound to the IP address and TLS fingerprint** of the
client that solved the challenge. That drives the deployment rules:

| Where you run the proxy | What you need |
| --- | --- |
| Same machine/IP as your browser | Nothing extra. Just the refresh token. |
| A VPS or datacenter on a different IP | `SHIORI_COOKIE` (full browser cookie string) **and** `SHIORI_PROXY` (a proxy whose egress IP matches the cookie). |

`SHIORI_COOKIE` is the raw `name=value; name=value` string from the browser.
`SHIORI_PROXY` is an HTTP(S) proxy URL, e.g. `http://user:pass@host:port`.

## Docker

```bash
docker build -t shioriai2api .

docker run --rm -p 8080:8080 \
  -e PROXY_API_KEY='your-proxy-key' \
  -e SHIORI_REFRESH_TOKEN='your-refresh-token' \
  shioriai2api
```

Or with Compose:

```bash
PROXY_API_KEY='your-proxy-key' SHIORI_REFRESH_TOKEN='your-refresh-token' docker compose up --build
```

## Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `PORT` | `8080` | Local HTTP port |
| `PROXY_API_KEY` | none — **required** | Key clients use to call the proxy |
| `SHIORI_REFRESH_TOKEN` | empty | shiori.ai refresh token |
| `SHIORI_ACCOUNTS_FILE` | `accounts.txt` | Multi-account file, one token per line |
| `SHIORI_COOKIE` | empty | Cookie header for Cloudflare (needed off the browser's IP) |
| `SHIORI_PROXY` | empty | Upstream HTTP(S) proxy URL |
| `SHIORI_BASE_URL` | `https://www.shiori.ai` | Upstream base URL |
| `DEFAULT_MODEL` | `glm-5.3-flash` | Model used when a request omits one |
| `CONVERSATION_TTL` | `30m` | Idle time before a conversation key is forgotten |
| `MAX_CONVERSATIONS` | `1024` | Max conversations kept in memory (LRU eviction) |

## Multi-turn conversations

- **Automatic mode (recommended).** Clients send the full `messages` array as
  usual. The proxy fingerprints the history (everything except the last
  message); on a hit it reuses the same shiori.ai `chatId` and sends only the
  new turn, otherwise it starts a new conversation. Mainstream clients work
  with no changes.
- **Passthrough mode.** Responses include a non-standard `conversation_id`
  field (the shiori.ai chat id). Send it in a later request body to reuse that
  conversation.

## Authentication

Every `/v1/*` request needs:

```
Authorization: Bearer <PROXY_API_KEY>
```

`SHIORI_REFRESH_TOKEN` / `accounts.txt` are used server-side only and are never
exposed to callers.

## Supported models

The proxy serves shiori.ai's live catalog, so the list stays current
automatically. A sample:

| Group | Examples |
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

## Notes & limits

- **Usage** comes from the upstream `finish` event (`inputTokens`,
  `outputTokens`, `totalTokens`). When absent, completion tokens are estimated
  from the reply length.
- Upstream `401` after a refresh maps to `401`; a Cloudflare challenge maps to
  `502` with a message naming the fix.
- Tool calls and image input are not wired through yet; text and reasoning are.
- Do **not** commit your tokens. `accounts.txt` is already in `.gitignore`.

## Project layout

```
main.go                    HTTP server and routing
config/                    environment configuration
handlers/                  OpenAI-compatible handlers, SSE, conversations
shiori/                    upstream client, auth, streaming parser
internal/mockshiori/       mock upstream for end-to-end tests
docs/api-analysis.md       reverse-engineering notes on the upstream API
scripts/                   asset generation and smoke-test helpers
assets/                    logo, wordmark and social banner
```

## License

Released under the [MIT License](LICENSE).

Not affiliated with shiori.ai. Use responsibly and respect upstream terms.
