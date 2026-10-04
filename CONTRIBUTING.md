# Contributing to shioriai2api

Thanks for wanting to help. This project is small on purpose: standard library
only, no runtime dependencies, easy to read end to end. Keep it that way.

## Getting set up

```bash
git clone https://github.com/0xgetz/shioriai2api.git
cd shioriai2api
go build ./...
go vet ./...
go test ./...
```

You need Go 1.24 or newer. The project has **zero third-party dependencies**;
please don't add any without a very good reason discussed in an issue first.

## Running the checks

Before opening a pull request, run:

```bash
go build ./...
go vet ./...
go test ./...
./scripts/smoke_test.sh
```

The smoke test spins up the mock upstream (`internal/mockshiori`) and the proxy
on local ports, then exercises health, auth, model listing and streaming and
non-streaming chat. It must pass.

## Guidelines

- **Match the existing style.** Small functions, clear names, no unnecessary
  abstractions.
- **No comments unless they add real value.** The code should read plainly.
- **Keep secrets out.** Never commit refresh tokens, cookies, proxy
  credentials, or a populated `accounts.txt`. It is already gitignored.
- **One change per pull request.** Bug fixes and features are easier to review
  when they are separate.
- **Add a test** for any behavior change in `shiori/` or `handlers/`. The
  existing table-driven tests are a good template.
- **Update the docs** if you change configuration, endpoints, or the request /
  response shape.

## Reporting bugs

Open an issue with:

- what you ran (command or request body, with secrets removed),
- what you expected,
- what happened, including the full error or log line,
- your OS and Go version.

## Feature scope

The project translates the shiori.ai chat API into the OpenAI chat completions
shape. Out of scope for now: tool calling, image inputs, embeddings, and
fine-tuning. Discussion is welcome, but a focused pull request that keeps the
proxy small and dependency-free is far more likely to be merged than a large
one.
