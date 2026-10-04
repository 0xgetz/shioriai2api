package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/0xgetz/shioriai2api/config"
)

func testHandler() *Handler {
	cfg := &config.Config{
		ProxyAPIKey:  "secret",
		DefaultModel: "glm-5.3-flash",
		BaseURL:      "http://127.0.0.1:1",
	}
	return New(cfg, NewPool([]string{"token-a", "token-b"}, "", "", ""), NewConvStore(30*time.Minute, 8))
}

func TestAuthRejectsMissingKey(t *testing.T) {
	h := testHandler()
	rec := httptest.NewRecorder()
	h.Auth(h.ListModels)(rec, httptest.NewRequest(http.MethodGet, "/v1/models", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
}

func TestAuthAcceptsKey(t *testing.T) {
	h := testHandler()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer secret")
	h.Auth(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
}

func TestChatRejectsEmptyMessages(t *testing.T) {
	h := testHandler()
	rec := httptest.NewRecorder()
	h.ChatCompletion(rec, httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
		strings.NewReader(`{"model":"glm-5.3-flash","messages":[]}`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestContentText(t *testing.T) {
	if got := contentText(json.RawMessage(`"hello"`)); got != "hello" {
		t.Fatalf("plain string = %q", got)
	}
	parts := `[{"type":"text","text":"a"},{"type":"text","text":"b"}]`
	if got := contentText(json.RawMessage(parts)); got != "ab" {
		t.Fatalf("parts = %q", got)
	}
}

func TestMapFinishReason(t *testing.T) {
	cases := map[string]string{
		"stop": "stop", "length": "length", "tool-calls": "tool_calls",
		"content-filter": "content_filter", "": "stop",
	}
	for in, want := range cases {
		if got := mapFinishReason(in); got != want {
			t.Fatalf("mapFinishReason(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestUsageFromMeta(t *testing.T) {
	u := usageFromMeta(map[string]any{"inputTokens": float64(10), "outputTokens": float64(4), "totalTokens": float64(14)})
	if u == nil || u["prompt_tokens"] != 10 || u["completion_tokens"] != 4 || u["total_tokens"] != 14 {
		t.Fatalf("usage = %+v", u)
	}
	if usageFromMeta(map[string]any{}) != nil {
		t.Fatal("expected nil usage for empty meta")
	}
}

func TestPrefixKeyStability(t *testing.T) {
	msgs := []chatMessage{
		{Role: "system", Content: json.RawMessage(`"be brief"`)},
		{Role: "user", Content: json.RawMessage(`"hello"`)},
	}
	if PrefixKey(msgs, "glm-5.3-flash") != PrefixKey(msgs, "glm-5.3-flash") {
		t.Fatal("prefix key is not stable")
	}
	if PrefixKey(msgs, "glm-5.3-flash") == PrefixKey(msgs, "gpt-6-luna") {
		t.Fatal("prefix key must include the model")
	}
}

func TestNormalizeEffort(t *testing.T) {
	if e, ok := normalizeEffort("HIGH"); !ok || e != "high" {
		t.Fatalf("normalizeEffort(HIGH) = %q %v", e, ok)
	}
	if _, ok := normalizeEffort("turbo"); ok {
		t.Fatal("invalid effort should not normalise")
	}
}
