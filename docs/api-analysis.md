# Upstream API analysis

This document describes the `shiori.ai` web API that `shioriai2api` speaks. It
is the reverse-engineering reference for the client in [`shiori/`](../shiori)
and for anyone maintaining the proxy when upstream changes. It is not an
official shiori.ai document.

## Architecture

shiori.ai is a Next.js app backed by **Convex**
(`*.convex.cloud`). The browser talks to server routes under
`https://www.shiori.ai/api/*`, which proxy Convex actions. The whole site sits
behind **Cloudflare**.

## Authentication

shiori.ai uses Convex Auth with a two-token scheme:

| Location | Key | Lifetime |
| --- | --- | --- |
| `localStorage` | `__convexAuthJWT_https<cautiouspuma129>convexcloud` | ~1 hour |
| `localStorage` | `__convexAuthRefreshToken_https<cautiouspuma129>convexcloud` | long-lived |
| HttpOnly cookie | `__Host-__convexAuthJWT` | ~1 hour |
| HttpOnly cookie | `__Host-__convexAuthRefreshToken` | long-lived |

The JWT is sent as `Authorization: Bearer <JWT>` on every API call.

### Exchanging the refresh token

```
POST /api/auth
Content-Type: application/json

{"action":"auth:signIn","args":{"refreshToken":"<refresh token>"}}
```

Response:

```json
{"tokens":{"token":"<JWT>","refreshToken":"<rotated refresh token>"}}
```

Without a valid credential the API returns
`401 {"error":"Authentication required"}`.

## Cloudflare

Every request from outside the browser is challenged with a Cloudflare
"Just a moment..." page unless it carries a valid **`cf_clearance`** cookie.

`cf_clearance` is bound to the client's IP address and TLS fingerprint. This
has a practical consequence for the proxy:

- Run it on the **same machine / IP** as the browser that produced the cookie
  (the normal case: your own laptop). Then no cookie configuration is needed.
- On a different IP (a VPS, a datacenter), you must set `SHIORI_COOKIE` to the
  browser's full cookie string **and** route the proxy through an
  `SHIORI_PROXY` whose egress IP matches where the cookie was issued.

## Endpoints used

### `GET /api/models/catalog`

Public. Returns an array of provider groups:

```json
[{"id":"shiori","title":"Shiori",
  "models":[{"id":"glm-5.3-flash","name":"GLM-5.3 Flash",
             "reasoning":true,"multiModal":true,"isEnabled":true,
             "contextWindow":1048576}]}]
```

Fields used by the proxy: `id`, `name`, `reasoning`, `multiModal`,
`isEnabled`, `isPremiumModel`, `contextWindow`.

### `POST /api/chat`

The streaming chat endpoint. Request body:

```json
{
  "modelId": "glm-5.3-flash",
  "chatId": "<client-generated conversation id>",
  "reasoningEnabled": true,
  "reasoningEffort": "medium",
  "id": "<same as chatId for the first message>",
  "message": {"id":"<uuid>","role":"user","parts":[{"type":"text","text":"..."}]},
  "trigger": "submit-message"
}
```

- `chatId` identifies the conversation; the server keeps history keyed by it.
  Reuse it to continue a conversation and send only the new user message.
- `reasoningEffort` is one of `max, xhigh, high, medium, low, minimal, none`.
- `message.parts` is an array so non-text parts (images) can be added.

## Response stream

The response is `text/event-stream` in the **Vercel AI SDK v5 UI Message
Stream** format. Each `data:` line is a JSON event:

```json
{"type":"start","messageId":"msg_...","messageMetadata":{"model":"glm-5.3-flash","chatId":"..."}}
{"type":"start-step"}
{"type":"reasoning-start","id":"..."}
{"type":"reasoning-delta","id":"...","delta":"The user "}
{"type":"reasoning-end","id":"..."}
{"type":"text-start","id":"..."}
{"type":"text-delta","id":"...","delta":"Hello"}
{"type":"text-end","id":"..."}
{"type":"finish-step"}
{"type":"finish","finishReason":"stop","messageMetadata":{
   "model":"glm-5.3-flash","duration":1840,"ttftMs":1707,
   "inputTokens":1897,"outputTokens":5,"reasoningTokens":0,
   "cachedInputTokens":0,"totalTokens":1902,"stepCount":1}}
data: [DONE]
```

The proxy maps:

| shiori.ai event | OpenAI field |
| --- | --- |
| `text-delta.delta` | `choices[].delta.content` |
| `reasoning-delta.delta` | `choices[].delta.reasoning_content` |
| `finish.finishReason` | `choices[].finish_reason` |
| `finish.messageMetadata.*Tokens` | `usage` |

## Known limitations

- **No tool calls.** The upstream stream carries text and reasoning only; the
  proxy does not fabricate tool-call frames.
- **No upstream session listing.** History lives on shiori.ai's side; the proxy
  only tracks conversation ids in memory.
- **Image input** is not yet wired through `message.parts`; text is supported.
