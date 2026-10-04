<div align="center">

<img src="assets/wordmark.png" alt="shioriai2api" width="640">

**Ubah [shiori.ai](https://www.shiori.ai) menjadi API yang kompatibel dengan OpenAI.**

Proxy Go ringan tanpa dependensi yang berbicara protokol OpenAI Chat Completions.

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

## Ringkasan

`shioriai2api` membungkus **aplikasi web shiori.ai** di balik API HTTP yang
bersih dan kompatibel dengan OpenAI. Ditulis dengan Go murni hanya menggunakan
pustaka standar, sehingga dikirim sebagai satu biner statis tanpa dependensi
runtime.

Arahkan klien OpenAI apa pun ke sini — Cherry Studio, LobeChat, Open WebUI, SDK
resmi OpenAI, `curl` — dan akses **90+ model** shiori.ai (GLM, Claude, GPT,
Gemini, DeepSeek, Qwen, Kimi, Grok, MiniMax, dan lainnya) melalui satu endpoint
yang familier.

> **Bawa akun Anda sendiri.** Anda menyediakan kredensial dari sesi browser
> yang sudah login. Proxy tidak pernah mengirim data ke mana pun.

> **Penting:** shiori.ai berada di balik Cloudflare. Jalankan proxy di
> **mesin yang sama dengan browser** dan tidak ada konfigurasi tambahan yang
> diperlukan. Menjalankannya di IP berbeda memerlukan pengaturan
> `SHIORI_COOKIE` dan `SHIORI_PROXY` — lihat [Cloudflare](#cloudflare) di bawah.

## Fitur

- **Kompatibel dengan OpenAI Chat Completions** — `POST /v1/chat/completions`,
  streaming maupun non-streaming.
- **Daftar model langsung** — `GET /v1/models`, diambil dari katalog asli
  shiori.ai (`/api/models/catalog`).
- **Dukungan penalaran** — keluaran thinking dipetakan ke
  `reasoning_content`, dengan passthrough `reasoning_effort` (`max`…`none`).
- **Penggunaan akurat** — jumlah token langsung dari event `finish` upstream.
- **Kumpulan token multi-akun** — round-robin antar akun; kredensial tetap di
  sisi server.
- **Konteks multi-giliran** — cache prefiks otomatis menggunakan kembali
  percakapan shiori.ai yang sama; passthrough `conversation_id` eksplisit juga
  didukung.
- **Refresh JWT otomatis** — refresh token berumur panjang ditukar dengan JWT
  berumur pendek secara transparan, dan dicoba ulang sekali saat `401`.
- **Sadar Cloudflare** — mengklasifikasikan respons tantangan dan memberi tahu
  pengaturan mana yang harus diperbaiki.
- **Siap Docker / Docker Compose**.

## Mendapatkan kredensial

1. Masuk ke [shiori.ai](https://www.shiori.ai) di browser Anda.
2. Buka DevTools (`F12`) → **Console** dan jalankan:

   ```js
   Object.keys(localStorage)
     .filter(k => k.startsWith('__convexAuthRefreshToken'))
     .map(k => localStorage.getItem(k))[0]
   ```

3. Salin hasilnya — itulah **refresh token** Anda. Ini adalah kredensial
   berumur panjang yang digunakan proxy (proxy sendiri yang menukarnya dengan
   JWT berumur pendek).

> Menjalankan di IP berbeda dari browser? Salin juga seluruh string cookie
> (termasuk `cf_clearance`) ke `SHIORI_COOKIE`. Lihat [Cloudflare](#cloudflare).

## Mulai cepat

### Build

```bash
go build -o shioriai2api .
```

### Jalankan

Satu akun:

```bash
PROXY_API_KEY='kunci-proxy-anda' SHIORI_REFRESH_TOKEN='refresh-token-anda' PORT=8080 ./shioriai2api
```

Beberapa akun — buat `accounts.txt` di direktori kerja, satu refresh token per
baris (baris kosong dan komentar `#` diabaikan):

```
token1
token2
```

```bash
PROXY_API_KEY='kunci-proxy-anda' ./shioriai2api
```

### Uji

```bash
curl http://127.0.0.1:8080/v1/chat/completions \
  -H 'Authorization: Bearer kunci-proxy-anda' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "glm-5.3-flash",
    "messages": [{"role": "user", "content": "Halo"}],
    "stream": true
  }'
```

### Verifikasi build

Uji asap mandiri akan menjalankan upstream tiruan dan proxy, lalu memeriksa
health, autentikasi, daftar model, serta chat streaming/non-streaming:

```bash
./scripts/smoke_test.sh
```

## Cloudflare

shiori.ai dilindungi Cloudflare, dan cookie clearance-nya (`cf_clearance`)
**terikat pada alamat IP dan sidik jari TLS** klien yang memecahkan tantangan.
Ini menentukan aturan deployment:

| Di mana proxy dijalankan | Yang dibutuhkan |
| --- | --- |
| Mesin/IP yang sama dengan browser | Tidak ada tambahan. Cukup refresh token. |
| VPS atau datacenter di IP berbeda | `SHIORI_COOKIE` (string cookie browser lengkap) **dan** `SHIORI_PROXY` (proxy yang IP keluarnya sama dengan cookie). |

`SHIORI_COOKIE` adalah string mentah `name=value; name=value` dari browser.
`SHIORI_PROXY` adalah URL proxy HTTP(S), misalnya
`http://user:pass@host:port`.

## Docker

```bash
docker build -t shioriai2api .

docker run --rm -p 8080:8080 \
  -e PROXY_API_KEY='kunci-proxy-anda' \
  -e SHIORI_REFRESH_TOKEN='refresh-token-anda' \
  shioriai2api
```

Atau dengan Compose:

```bash
PROXY_API_KEY='kunci-proxy-anda' SHIORI_REFRESH_TOKEN='refresh-token-anda' docker compose up --build
```

## Konfigurasi

| Variabel | Default | Deskripsi |
| --- | --- | --- |
| `PORT` | `8080` | Port HTTP lokal |
| `PROXY_API_KEY` | tidak ada — **wajib** | Kunci yang dipakai klien untuk memanggil proxy |
| `SHIORI_REFRESH_TOKEN` | kosong | Refresh token shiori.ai |
| `SHIORI_ACCOUNTS_FILE` | `accounts.txt` | File multi-akun, satu token per baris |
| `SHIORI_COOKIE` | kosong | Header Cookie untuk Cloudflare (perlu di luar IP browser) |
| `SHIORI_PROXY` | kosong | URL proxy HTTP(S) upstream |
| `SHIORI_BASE_URL` | `https://www.shiori.ai` | URL dasar upstream |
| `DEFAULT_MODEL` | `glm-5.3-flash` | Model yang dipakai saat permintaan tidak menyertakan model |
| `CONVERSATION_TTL` | `30m` | Waktu idle sebelum kunci percakapan dilupakan |
| `MAX_CONVERSATIONS` | `1024` | Maksimum percakapan yang disimpan di memori (eviksi LRU) |

## Percakapan multi-giliran

- **Mode otomatis (disarankan).** Klien mengirim seluruh array `messages`
  seperti biasa. Proxy menyidik riwayat (semua kecuali pesan terakhir); jika
  cocok, `chatId` shiori.ai yang sama digunakan kembali dan hanya giliran baru
  yang dikirim, jika tidak percakapan baru dimulai. Klien populer berfungsi
  tanpa perubahan.
- **Mode passthrough.** Respons menyertakan bidang non-standar
  `conversation_id` (id chat shiori.ai). Kirimkan di body permintaan
  berikutnya untuk menggunakan kembali percakapan tersebut.

## Autentikasi

Setiap permintaan `/v1/*` memerlukan:

```
Authorization: Bearer <PROXY_API_KEY>
```

`SHIORI_REFRESH_TOKEN` / `accounts.txt` hanya dipakai di sisi server dan tidak
pernah dibuka ke pemanggil.

## Model yang didukung

Proxy menyajikan katalog langsung shiori.ai, sehingga daftarnya selalu
mutakhir. Contoh:

| Grup | Contoh |
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

## Catatan & batasan

- **Penggunaan** berasal dari event `finish` upstream (`inputTokens`,
  `outputTokens`, `totalTokens`). Jika tidak ada, token completion diperkirakan
  dari panjang balasan.
- `401` upstream setelah refresh dipetakan ke `401`; tantangan Cloudflare
  dipetakan ke `502` dengan pesan yang menyebutkan solusinya.
- Tool call dan input gambar belum disalurkan; teks dan penalaran sudah.
- **Jangan** commit token Anda. `accounts.txt` sudah ada di `.gitignore`.

## Struktur proyek

```
main.go                    Server HTTP dan routing
config/                    Konfigurasi lingkungan
handlers/                  Handler kompatibel OpenAI, SSE, percakapan
shiori/                    Klien upstream, auth, parser streaming
internal/mockshiori/       Upstream tiruan untuk uji end-to-end
docs/api-analysis.md       Catatan reverse-engineering API upstream
scripts/                   Pembuat aset dan pembantu uji asap
assets/                    Logo, wordmark, dan banner sosial
```

## Lisensi

Dirilis di bawah [Lisensi MIT](LICENSE).

Tidak berafiliasi dengan shiori.ai. Gunakan secara bertanggung jawab dan
hormati ketentuan upstream.
