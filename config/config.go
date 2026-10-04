// Package config loads the proxy's runtime settings from the environment.
//
// Every setting has a safe default except PROXY_API_KEY, which is mandatory:
// it is the shared secret clients must present, and an unauthenticated proxy
// would expose the operator's shiori.ai account to the whole network.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the fully-resolved configuration for one server process.
type Config struct {
	// Port is the local HTTP listen port.
	Port string

	// ProxyAPIKey is the bearer token clients must send on every /v1/* call.
	ProxyAPIKey string

	// BaseURL is the shiori.ai origin (https://www.shiori.ai by default).
	BaseURL string

	// Accounts holds one shiori.ai refresh token per configured account.
	Accounts []string

	// AccountsFile is where extra refresh tokens are read from, one per line.
	AccountsFile string

	// Cookie is an optional Cookie header sent with every upstream request.
	// shiori.ai is behind Cloudflare, so passing the browser's cf_clearance
	// and auth cookies is what makes the proxy work from a server.
	Cookie string

	// Proxy is an optional upstream HTTP(S) proxy URL. Cloudflare binds the
	// cf_clearance cookie to the client's IP, so a datacenter host must route
	// through the same residential IP as the browser session.
	Proxy string

	// UserAgent overrides the upstream User-Agent (defaults to a Chrome UA).
	UserAgent string

	// DefaultModel is used when a request omits the model field.
	DefaultModel string

	// ConversationTTL is how long an idle conversation key is kept.
	ConversationTTL time.Duration

	// MaxConvs caps how many live conversations are tracked in memory.
	MaxConvs int
}

// Load reads configuration from the environment. It returns an error when a
// required value is missing or malformed so the process fails fast at boot
// instead of mid-request.
func Load() (*Config, error) {
	cfg := &Config{
		Port:            env("PORT", "8080"),
		ProxyAPIKey:     strings.TrimSpace(os.Getenv("PROXY_API_KEY")),
		BaseURL:         strings.TrimRight(env("SHIORI_BASE_URL", "https://www.shiori.ai"), "/"),
		AccountsFile:    env("SHIORI_ACCOUNTS_FILE", "accounts.txt"),
		Cookie:          envAny("SHIORI_COOKIE", "SHIORI_COOKIES"),
		Proxy:           envAny("SHIORI_PROXY", "SHIORI_PROXY_URL"),
		DefaultModel:    env("DEFAULT_MODEL", "glm-5.3-flash"),
		ConversationTTL: 30 * time.Minute,
		MaxConvs:        1024,
	}

	if cfg.ProxyAPIKey == "" {
		return nil, errors.New("PROXY_API_KEY is required")
	}

	if v := strings.TrimSpace(os.Getenv("CONVERSATION_TTL")); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("invalid CONVERSATION_TTL %q: %w", v, err)
		}
		if d <= 0 {
			return nil, fmt.Errorf("invalid CONVERSATION_TTL %q: must be positive", v)
		}
		cfg.ConversationTTL = d
	}

	if v := strings.TrimSpace(os.Getenv("MAX_CONVERSATIONS")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return nil, fmt.Errorf("invalid MAX_CONVERSATIONS %q: must be a positive integer", v)
		}
		cfg.MaxConvs = n
	}

	// The refresh token may be supplied inline or via the accounts file.
	if t := strings.TrimSpace(os.Getenv("SHIORI_REFRESH_TOKEN")); t != "" {
		cfg.Accounts = append(cfg.Accounts, t)
	}
	fileAccounts, err := loadAccountsFile(cfg.AccountsFile)
	if err != nil {
		return nil, err
	}
	cfg.Accounts = dedupe(append(cfg.Accounts, fileAccounts...))

	if len(cfg.Accounts) == 0 {
		return nil, fmt.Errorf("no shiori.ai account configured; set SHIORI_REFRESH_TOKEN or provide tokens in %s", cfg.AccountsFile)
	}
	return cfg, nil
}

// loadAccountsFile parses one refresh token per line, skipping blanks and
// # comments. A missing file is not an error: single-account setups rely on
// SHIORI_REFRESH_TOKEN.
func loadAccountsFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read accounts file %s: %w", path, err)
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out, nil
}

// dedupe drops repeated tokens while preserving order, so the same account is
// never added to the round-robin pool twice.
func dedupe(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

// envAny returns the first non-empty of the given environment variables.
func envAny(keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}
