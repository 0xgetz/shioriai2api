<div align="center">

<img src="assets/wordmark.png" alt="shioriai2api" width="640">

**将 [shiori.ai](https://www.shiori.ai) 变成兼容 OpenAI 的 API。**

一个轻量、零依赖的 Go 代理，使用 OpenAI Chat Completions 协议。

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

## 概述

`shioriai2api` 将 **shiori.ai 网页应用** 封装在一个干净、兼容 OpenAI 的
HTTP API 之后。它仅使用标准库、以纯 Go 编写，因此以单个静态二进制文件
发布，没有任何运行时依赖。

把任意 OpenAI 客户端指向它——Cherry Studio、LobeChat、Open WebUI、官方
OpenAI SDK、`curl`——即可通过一个熟悉的端点访问 shiori.ai 的 **90+ 个
模型**（GLM、Claude、GPT、Gemini、DeepSeek、Qwen、Kimi、Grok、MiniMax
等）。

> **自带账号。** 你提供自己已登录浏览器会话中的凭据。代理绝不会回传任何
> 数据。

> **重要：** shiori.ai 位于 Cloudflare 之后。把代理运行在**与浏览器相同的
> 机器**上就无需额外配置。在不同 IP 上运行需要 `SHIORI_COOKIE` 与
> `SHIORI_PROXY` 设置——见下文 [Cloudflare](#cloudflare)。

## 功能特性

- **兼容 OpenAI Chat Completions** — `POST /v1/chat/completions`，支持流式
  与非流式。
- **实时模型列表** — `GET /v1/models`，来自 shiori.ai 的真实目录
  (`/api/models/catalog`)。
- **推理支持** — thinking 输出映射为 `reasoning_content`，并透传
  `reasoning_effort`（`max`…`none`）。
- **精确用量** — token 数量直接来自上游 `finish` 事件。
- **多账号令牌池** — 账号间轮询；凭据只保留在服务端。
- **多轮上下文** — 自动前缀缓存复用同一个 shiori.ai 会话；也支持显式
  `conversation_id` 透传。
- **自动刷新 JWT** — 长期有效的 refresh token 会被透明地换成短期 JWT，
  并在 `401` 时重试一次。
- **感知 Cloudflare** — 对挑战响应进行分类，并明确告诉你该修复哪个设置。
- **支持 Docker / Docker Compose**。

## 获取凭据

1. 在浏览器中登录 [shiori.ai](https://www.shiori.ai)。
2. 打开开发者工具（`F12`）→ **Console** 并运行：

   ```js
   Object.keys(localStorage)
     .filter(k => k.startsWith('__convexAuthRefreshToken'))
     .map(k => localStorage.getItem(k))[0]
   ```

3. 复制输出——这就是你的 **refresh token**。它是代理使用的长期凭据（代理
   会自行把它换成短期 JWT）。

> 在与浏览器不同的 IP 上运行？请同时把完整 cookie 字符串（包括
> `cf_clearance`）复制到 `SHIORI_COOKIE`。见 [Cloudflare](#cloudflare)。

## 快速开始

### 构建

```bash
go build -o shioriai2api .
```

### 运行

单账号：

```bash
PROXY_API_KEY='你的代理密钥' SHIORI_REFRESH_TOKEN='你的 refresh token' PORT=8080 ./shioriai2api
```

多账号——在工作目录创建 `accounts.txt`，每行一个 refresh token（空行与
`#` 注释会被忽略）：

```
token1
token2
```

```bash
PROXY_API_KEY='你的代理密钥' ./shioriai2api
```

### 测试

```bash
curl http://127.0.0.1:8080/v1/chat/completions \
  -H 'Authorization: Bearer 你的代理密钥' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "glm-5.3-flash",
    "messages": [{"role": "user", "content": "你好"}],
    "stream": true
  }'
```

### 验证构建

自包含的冒烟测试会启动模拟上游与代理，然后检查健康检查、鉴权、模型列表
以及流式/非流式对话：

```bash
./scripts/smoke_test.sh
```

## Cloudflare

shiori.ai 受 Cloudflare 保护，其 clearance cookie（`cf_clearance`）
**绑定到解决挑战的客户端的 IP 地址与 TLS 指纹**。这决定了部署规则：

| 代理运行位置 | 需要什么 |
| --- | --- |
| 与浏览器相同的机器/IP | 无需额外配置。只需 refresh token。 |
| 不同 IP 的 VPS 或数据中心 | `SHIORI_COOKIE`（完整浏览器 cookie 字符串）**以及** `SHIORI_PROXY`（出口 IP 与 cookie 一致的代理）。 |

`SHIORI_COOKIE` 是来自浏览器的原始 `name=value; name=value` 字符串。
`SHIORI_PROXY` 是 HTTP(S) 代理 URL，例如 `http://user:pass@host:port`。

## Docker

```bash
docker build -t shioriai2api .

docker run --rm -p 8080:8080 \
  -e PROXY_API_KEY='你的代理密钥' \
  -e SHIORI_REFRESH_TOKEN='你的 refresh token' \
  shioriai2api
```

或使用 Compose：

```bash
PROXY_API_KEY='你的代理密钥' SHIORI_REFRESH_TOKEN='你的 refresh token' docker compose up --build
```

## 配置

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `PORT` | `8080` | 本地 HTTP 端口 |
| `PROXY_API_KEY` | 无 — **必填** | 客户端调用代理所用的密钥 |
| `SHIORI_REFRESH_TOKEN` | 空 | shiori.ai refresh token |
| `SHIORI_ACCOUNTS_FILE` | `accounts.txt` | 多账号文件，每行一个 token |
| `SHIORI_COOKIE` | 空 | 用于 Cloudflare 的 Cookie 头（在浏览器 IP 之外需要） |
| `SHIORI_PROXY` | 空 | 上游 HTTP(S) 代理 URL |
| `SHIORI_BASE_URL` | `https://www.shiori.ai` | 上游基础 URL |
| `DEFAULT_MODEL` | `glm-5.3-flash` | 请求未指定模型时使用的模型 |
| `CONVERSATION_TTL` | `30m` | 会话键被遗忘前的空闲时长 |
| `MAX_CONVERSATIONS` | `1024` | 内存中保留的最大会话数（LRU 淘汰） |

## 多轮对话

- **自动模式（推荐）。** 客户端照常发送完整的 `messages` 数组。代理会对
  历史（除最后一条消息外的全部内容）做指纹；命中时复用同一个 shiori.ai
  `chatId` 并只发送新的回合，否则开始新会话。主流客户端无需任何改动。
- **透传模式。** 响应会包含一个非标准字段 `conversation_id`（shiori.ai 的
  聊天 id）。在后续请求体中发送它即可复用该会话。

## 鉴权

每个 `/v1/*` 请求都需要：

```
Authorization: Bearer <PROXY_API_KEY>
```

`SHIORI_REFRESH_TOKEN` / `accounts.txt` 仅在服务端使用，绝不会暴露给调用方。

## 支持的模型

代理提供 shiori.ai 的实时目录，因此列表会自动保持最新。示例：

| 分组 | 示例 |
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

## 注意事项与限制

- **用量**来自上游 `finish` 事件（`inputTokens`、`outputTokens`、
  `totalTokens`）。缺失时，completion token 按回复长度估算。
- 刷新后的上游 `401` 映射为 `401`；Cloudflare 挑战映射为 `502`，并在消息
  中指出修复方法。
- 工具调用与图像输入尚未接入；文本与推理已支持。
- **切勿**提交你的 token。`accounts.txt` 已在 `.gitignore` 中。

## 项目结构

```
main.go                    HTTP 服务器与路由
config/                    环境配置
handlers/                  兼容 OpenAI 的处理器、SSE、会话
shiori/                    上游客户端、鉴权、流式解析
internal/mockshiori/       用于端到端测试的模拟上游
docs/api-analysis.md       上游 API 逆向分析笔记
scripts/                   资源生成与冒烟测试辅助脚本
assets/                    Logo、字标与社交横幅
```

## 许可证

基于 [MIT 许可证](LICENSE) 发布。

与 shiori.ai 无关联。请负责任地使用并遵守上游条款。
