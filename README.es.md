<div align="center">

<img src="assets/wordmark.png" alt="shioriai2api" width="640">

**Convierte [shiori.ai](https://www.shiori.ai) en una API compatible con OpenAI.**

Un proxy Go ligero y sin dependencias que habla el protocolo OpenAI Chat Completions.

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

## Descripción general

`shioriai2api` envuelve la **aplicación web shiori.ai** detrás de una API HTTP
limpia y compatible con OpenAI. Está escrito en Go puro usando solo la
biblioteca estándar, por lo que se distribuye como un único binario estático
sin dependencias en tiempo de ejecución.

Apunta cualquier cliente de OpenAI hacia él — Cherry Studio, LobeChat, Open
WebUI, los SDK oficiales de OpenAI, `curl` — y accede a los **más de 90
modelos** de shiori.ai (GLM, Claude, GPT, Gemini, DeepSeek, Qwen, Kimi, Grok,
MiniMax y más) a través de un único endpoint familiar.

> **Trae tu propia cuenta.** Tú proporcionas las credenciales de tu propia
> sesión de navegador iniciada. El proxy nunca envía datos a ningún sitio.

> **Importante:** shiori.ai está detrás de Cloudflare. Ejecuta el proxy en la
> **misma máquina que tu navegador** y no necesitarás configuración adicional.
> Ejecutarlo en otra IP requiere los ajustes `SHIORI_COOKIE` y `SHIORI_PROXY`
> — consulta [Cloudflare](#cloudflare) más abajo.

## Características

- **Compatible con OpenAI Chat Completions** — `POST /v1/chat/completions`,
  con streaming y sin streaming.
- **Listado de modelos en vivo** — `GET /v1/models`, servido desde el catálogo
  real de shiori.ai (`/api/models/catalog`).
- **Soporte de razonamiento** — la salida de pensamiento se mapea a
  `reasoning_content`, con paso de `reasoning_effort` (`max`…`none`).
- **Uso preciso** — los recuentos de tokens vienen directamente del evento
  `finish` del upstream.
- **Grupo de tokens multicuenta** — round-robin entre cuentas; las
  credenciales permanecen en el servidor.
- **Contexto multiturno** — la caché automática de prefijos reutiliza la misma
  conversación de shiori.ai; también se admite el paso explícito de
  `conversation_id`.
- **Renovación automática de JWT** — un refresh token de larga duración se
  cambia por JWT de corta duración de forma transparente, con un reintento en
  `401`.
- **Consciente de Cloudflare** — clasifica las respuestas de desafío y te dice
  exactamente qué ajuste corregir.
- **Listo para Docker / Docker Compose**.

## Obtener tus credenciales

1. Inicia sesión en [shiori.ai](https://www.shiori.ai) en tu navegador.
2. Abre DevTools (`F12`) → **Console** y ejecuta:

   ```js
   Object.keys(localStorage)
     .filter(k => k.startsWith('__convexAuthRefreshToken'))
     .map(k => localStorage.getItem(k))[0]
   ```

3. Copia la salida: ese es tu **refresh token**. Es la credencial de larga
   duración que usa el proxy (él mismo la cambia por JWT de corta duración).

> ¿Lo ejecutas en una IP distinta a la del navegador? Copia también la cadena
> completa de cookies (incluida `cf_clearance`) en `SHIORI_COOKIE`. Consulta
> [Cloudflare](#cloudflare).

## Inicio rápido

### Compilar

```bash
go build -o shioriai2api .
```

### Ejecutar

Una sola cuenta:

```bash
PROXY_API_KEY='tu-clave-proxy' SHIORI_REFRESH_TOKEN='tu-refresh-token' PORT=8080 ./shioriai2api
```

Varias cuentas — crea `accounts.txt` en el directorio de trabajo, un refresh
token por línea (las líneas vacías y los comentarios `#` se ignoran):

```
token1
token2
```

```bash
PROXY_API_KEY='tu-clave-proxy' ./shioriai2api
```

### Probar

```bash
curl http://127.0.0.1:8080/v1/chat/completions \
  -H 'Authorization: Bearer tu-clave-proxy' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "glm-5.3-flash",
    "messages": [{"role": "user", "content": "Hola"}],
    "stream": true
  }'
```

### Verificar la compilación

Una prueba de humo autocontenida levanta un upstream simulado y el proxy, y
comprueba salud, autenticación, listado de modelos y chat con y sin streaming:

```bash
./scripts/smoke_test.sh
```

## Cloudflare

shiori.ai está protegido por Cloudflare, y su cookie de permiso
(`cf_clearance`) está **vinculada a la dirección IP y a la huella TLS** del
cliente que resolvió el desafío. Esto determina las reglas de despliegue:

| Dónde ejecutas el proxy | Qué necesitas |
| --- | --- |
| La misma máquina/IP que tu navegador | Nada extra. Solo el refresh token. |
| Un VPS o centro de datos en otra IP | `SHIORI_COOKIE` (cadena completa de cookies del navegador) **y** `SHIORI_PROXY` (un proxy cuya IP de salida coincida con la cookie). |

`SHIORI_COOKIE` es la cadena sin procesar `name=value; name=value` del
navegador. `SHIORI_PROXY` es una URL de proxy HTTP(S), p. ej.
`http://user:pass@host:port`.

## Docker

```bash
docker build -t shioriai2api .

docker run --rm -p 8080:8080 \
  -e PROXY_API_KEY='tu-clave-proxy' \
  -e SHIORI_REFRESH_TOKEN='tu-refresh-token' \
  shioriai2api
```

O con Compose:

```bash
PROXY_API_KEY='tu-clave-proxy' SHIORI_REFRESH_TOKEN='tu-refresh-token' docker compose up --build
```

## Configuración

| Variable | Valor por defecto | Descripción |
| --- | --- | --- |
| `PORT` | `8080` | Puerto HTTP local |
| `PROXY_API_KEY` | ninguno — **obligatorio** | Clave que usan los clientes para llamar al proxy |
| `SHIORI_REFRESH_TOKEN` | vacío | Refresh token de shiori.ai |
| `SHIORI_ACCOUNTS_FILE` | `accounts.txt` | Archivo multicuenta, un token por línea |
| `SHIORI_COOKIE` | vacío | Cabecera Cookie para Cloudflare (necesaria fuera de la IP del navegador) |
| `SHIORI_PROXY` | vacío | URL de proxy HTTP(S) del upstream |
| `SHIORI_BASE_URL` | `https://www.shiori.ai` | URL base del upstream |
| `DEFAULT_MODEL` | `glm-5.3-flash` | Modelo usado cuando una petición lo omite |
| `CONVERSATION_TTL` | `30m` | Tiempo inactivo antes de olvidar la clave de conversación |
| `MAX_CONVERSATIONS` | `1024` | Máximo de conversaciones en memoria (expulsión LRU) |

## Conversaciones multiturno

- **Modo automático (recomendado).** Los clientes envían el array completo de
  `messages` como de costumbre. El proxy huella el historial (todo excepto el
  último mensaje); si coincide, reutiliza el mismo `chatId` de shiori.ai y
  envía solo el nuevo turno; si no, inicia una conversación nueva. Los clientes
  habituales funcionan sin cambios.
- **Modo passthrough.** Las respuestas incluyen un campo no estándar
  `conversation_id` (el id de chat de shiori.ai). Envíalo en el cuerpo de una
  petición posterior para reutilizar esa conversación.

## Autenticación

Cada petición `/v1/*` necesita:

```
Authorization: Bearer <PROXY_API_KEY>
```

`SHIORI_REFRESH_TOKEN` / `accounts.txt` se usan solo en el servidor y nunca se
exponen a quien llama.

## Modelos compatibles

El proxy sirve el catálogo en vivo de shiori.ai, por lo que la lista se
mantiene actualizada automáticamente. Una muestra:

| Grupo | Ejemplos |
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

## Notas y límites

- **El uso** proviene del evento `finish` del upstream (`inputTokens`,
  `outputTokens`, `totalTokens`). Cuando falta, los tokens de completion se
  estiman por la longitud de la respuesta.
- Un `401` del upstream tras renovar se mapea a `401`; un desafío de
  Cloudflare se mapea a `502` con un mensaje que indica la solución.
- Las llamadas a herramientas y la entrada de imágenes aún no están conectadas;
  el texto y el razonamiento sí.
- **No** subas tus tokens al repositorio. `accounts.txt` ya está en
  `.gitignore`.

## Estructura del proyecto

```
main.go                    Servidor HTTP y enrutado
config/                    Configuración por entorno
handlers/                  Manejadores compatibles con OpenAI, SSE, conversaciones
shiori/                    Cliente del upstream, auth, parser de streaming
internal/mockshiori/       Upstream simulado para pruebas de extremo a extremo
docs/api-analysis.md       Notas de ingeniería inversa de la API del upstream
scripts/                   Generación de recursos y ayudas de prueba de humo
assets/                    Logo, wordmark y banner social
```

## Licencia

Publicado bajo la [Licencia MIT](LICENSE).

Sin afiliación con shiori.ai. Úsalo con responsabilidad y respeta los términos
del upstream.
