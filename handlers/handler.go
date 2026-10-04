package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/0xgetz/shioriai2api/config"
	"github.com/0xgetz/shioriai2api/shiori"
)

// AccountPool round-robins the configured shiori.ai accounts.
type AccountPool struct {
	clients []*shiori.Client
	next    atomic.Uint64
}

// NewPool builds a pool from the configured refresh tokens, forwarding the
// optional Cloudflare cookie and upstream proxy to every client.
func NewPool(tokens []string, baseURL, cookie, proxyURL string) *AccountPool {
	p := &AccountPool{}
	for _, t := range tokens {
		p.clients = append(p.clients, shiori.NewClient(t, baseURL, cookie, proxyURL))
	}
	return p
}

// Pick returns the next client in round-robin order.
func (p *AccountPool) Pick() *shiori.Client {
	return p.clients[p.next.Add(1)%uint64(len(p.clients))]
}

// Handler serves the OpenAI-compatible API.
type Handler struct {
	cfg   *config.Config
	pool  *AccountPool
	convs *ConvStore
}

// New wires a handler from its dependencies.
func New(cfg *config.Config, pool *AccountPool, convs *ConvStore) *Handler {
	return &Handler{cfg: cfg, pool: pool, convs: convs}
}

type chatRequest struct {
	Model           string        `json:"model"`
	Messages        []chatMessage `json:"messages"`
	Stream          bool          `json:"stream"`
	ConversationID  string        `json:"conversation_id"`
	ReasoningEffort string        `json:"reasoning_effort"`
	StreamOptions   *struct {
		IncludeUsage bool `json:"include_usage"`
	} `json:"stream_options"`
}

type openAIError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code,omitempty"`
	} `json:"error"`
}

func writeError(w http.ResponseWriter, status int, errType, msg string) {
	var e openAIError
	e.Error.Message = msg
	e.Error.Type = errType
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(e)
}

// Auth guards /v1/* with the proxy API key.
func (h *Handler) Auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := ""
		if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
			key = strings.TrimPrefix(auth, "Bearer ")
		}
		if key == "" {
			key = r.Header.Get("X-Api-Key")
		}
		if key == "" || key != h.cfg.ProxyAPIKey {
			writeError(w, http.StatusUnauthorized, "authentication_error", "invalid proxy API key")
			return
		}
		next(w, r)
	}
}

// ListModels serves GET /v1/models from the live shiori.ai catalog.
func (h *Handler) ListModels(w http.ResponseWriter, r *http.Request) {
	type model struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		OwnedBy string `json:"owned_by"`
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	client := h.pool.Pick()
	groups, err := client.Catalog(ctx)
	now := time.Now().Unix()
	out := struct {
		Object string  `json:"object"`
		Data   []model `json:"data"`
	}{Object: "list"}
	if err != nil {
		// Fall back to the configured default so the endpoint never 500s.
		log.Printf("models: catalog fetch failed: %v", err)
		out.Data = append(out.Data, model{ID: h.cfg.DefaultModel, Object: "model", Created: now, OwnedBy: "shiori"})
	} else {
		for _, g := range groups {
			for _, m := range g.Models {
				if !m.IsEnabled {
					continue
				}
				out.Data = append(out.Data, model{ID: m.ID, Object: "model", Created: now, OwnedBy: "shiori"})
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// ChatCompletion serves POST /v1/chat/completions.
func (h *Handler) ChatCompletion(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	var req chatRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "bad JSON body: "+err.Error())
		return
	}

	modelName := req.Model
	if modelName == "" {
		modelName = h.cfg.DefaultModel
	}

	var systemParts []string
	var lastUserText string
	hasUser := false
	for _, msg := range req.Messages {
		switch msg.Role {
		case "system", "developer":
			if text := contentText(msg.Content); text != "" {
				systemParts = append(systemParts, text)
			}
		case "user":
			hasUser = true
			lastUserText = contentText(msg.Content)
		}
	}
	if !hasUser || strings.TrimSpace(lastUserText) == "" {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "messages must contain a user message with text")
		return
	}

	// Resolve reasoning: explicit request value wins, otherwise infer from
	// the model id heuristics (chat vs reasoning).
	reasoning := true
	effort := "medium"
	if e, ok := normalizeEffort(req.ReasoningEffort); ok {
		effort = e
		reasoning = e != "none"
	}

	conv, fresh := h.conversation(&req)
	conv.Lock()
	defer conv.Unlock()

	prompt := lastUserText
	if fresh && len(systemParts) > 0 {
		prompt = "[System]\n" + strings.Join(systemParts, "\n\n") + "\n[/System]\n\n" + prompt
	}

	completionID := "chatcmpl-" + randHex(16)
	createdAt := time.Now().Unix()

	var reply, thinking strings.Builder
	var finishReason = "stop"
	var usage map[string]int

	cb := &shiori.StreamCallbacks{
		OnReasoningDelta: func(text string) { thinking.WriteString(text) },
		OnTextDelta:      func(text string) { reply.WriteString(text) },
		OnFinish: func(reason string, meta map[string]any) {
			if reason != "" {
				finishReason = mapFinishReason(reason)
			}
			usage = usageFromMeta(meta)
		},
	}

	var stream *sseWriter
	if req.Stream {
		stream = newSSEWriter(w, completionID, modelName, createdAt, conv.ChatID)
		if err := stream.open(); err != nil {
			log.Printf("chat: client went away before stream start: %v", err)
			return
		}
		cb.OnReasoningDelta = func(text string) {
			thinking.WriteString(text)
			stream.deltaThink(text)
		}
		cb.OnTextDelta = func(text string) {
			reply.WriteString(text)
			stream.delta(text)
		}
		cb.OnFinish = func(reason string, meta map[string]any) {
			if reason != "" {
				finishReason = mapFinishReason(reason)
			}
			usage = usageFromMeta(meta)
		}
	}

	err := conv.Client.Chat(r.Context(), shiori.ChatInput{
		ModelID:         modelName,
		ChatID:          conv.ChatID,
		Reasoning:       reasoning,
		ReasoningEffort: effort,
		Text:            prompt,
	}, cb)

	conv.Turn++
	// Register the continuation key even on partial streams so follow-ups chain.
	key := PrefixKey(append(append([]chatMessage{}, req.Messages...),
		chatMessage{Role: "assistant", Content: json.RawMessage(mustJSON(reply.String()))}), modelName)
	h.convs.Register(key, conv)

	if err != nil {
		if r.Context().Err() != nil {
			return // client gone; upstream finishes on its own
		}
		log.Printf("chat: upstream chat failed: %v", err)
		if req.Stream {
			stream.errorText(err.Error())
			return
		}
		status, msg := mapUpstreamError(err)
		writeError(w, status, "api_error", msg)
		return
	}

	if usage == nil {
		usage = estimateUsage(reply.Len())
	}
	if req.Stream {
		stream.finish(usage, finishReason, req.StreamOptions != nil && req.StreamOptions.IncludeUsage)
		log.Printf("chat %s %s conv=%s turn=%d stream %s", completionID, modelName, conv.ChatID, conv.Turn, time.Since(start).Round(time.Millisecond))
		return
	}

	message := map[string]any{"role": "assistant", "content": reply.String()}
	if t := thinking.String(); t != "" {
		message["reasoning_content"] = t
	}
	resp := map[string]any{
		"id":              completionID,
		"object":          "chat.completion",
		"created":         createdAt,
		"model":           modelName,
		"conversation_id": conv.ChatID,
		"choices": []any{map[string]any{
			"index":         0,
			"message":       message,
			"finish_reason": finishReason,
		}},
		"usage": usage,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
	log.Printf("chat %s %s conv=%s turn=%d %s", completionID, modelName, conv.ChatID, conv.Turn, time.Since(start).Round(time.Millisecond))
}

// conversation finds a live conversation or starts a new shiori.ai chat id.
func (h *Handler) conversation(req *chatRequest) (*Conversation, bool) {
	if req.ConversationID != "" {
		if c := h.convs.ByChat(req.ConversationID); c != nil {
			return c, false
		}
	} else if len(req.Messages) > 0 {
		key := PrefixKey(req.Messages[:len(req.Messages)-1], req.Model)
		if c := h.convs.ByPrefix(key); c != nil {
			return c, false
		}
	}
	return &Conversation{
		Client:   h.pool.Pick(),
		ChatID:   newChatID(),
		LastUsed: time.Now(),
	}, true
}

// mapFinishReason maps shiori.ai finish reasons to OpenAI ones.
func mapFinishReason(reason string) string {
	switch strings.ToLower(reason) {
	case "length", "max_tokens":
		return "length"
	case "tool-calls", "tool_calls":
		return "tool_calls"
	case "content-filter", "content_filter":
		return "content_filter"
	default:
		return "stop"
	}
}

// usageFromMeta reads the token counts shiori.ai reports on finish.
func usageFromMeta(meta map[string]any) map[string]int {
	if meta == nil {
		return nil
	}
	get := func(key string) int {
		if v, ok := meta[key]; ok {
			switch n := v.(type) {
			case float64:
				return int(n)
			case int:
				return n
			}
		}
		return 0
	}
	prompt := get("inputTokens")
	completion := get("outputTokens")
	total := get("totalTokens")
	if total == 0 {
		total = prompt + completion
	}
	if prompt == 0 && completion == 0 && total == 0 {
		return nil
	}
	return map[string]int{
		"prompt_tokens":     prompt,
		"completion_tokens": completion,
		"total_tokens":      total,
	}
}

func estimateUsage(replyChars int) map[string]int {
	completion := replyChars / 4
	return map[string]int{
		"prompt_tokens":     0,
		"completion_tokens": completion,
		"total_tokens":      completion,
	}
}

func mapUpstreamError(err error) (int, string) {
	var ae *shiori.APIError
	if errors.As(err, &ae) {
		if ae.Cloudflare {
			return http.StatusBadGateway, "shiori is behind Cloudflare and blocked the request; configure SHIORI_COOKIE with a fresh cf_clearance cookie"
		}
		switch ae.HTTPStatus {
		case http.StatusUnauthorized, http.StatusForbidden:
			return http.StatusUnauthorized, "shiori rejected the configured credentials (refresh token or cookie invalid or expired)"
		case http.StatusTooManyRequests:
			return http.StatusTooManyRequests, "shiori rate limited this account; retry later"
		default:
			return http.StatusBadGateway, "shiori upstream error: " + ae.Error()
		}
	}
	return http.StatusBadGateway, "shiori upstream error: " + err.Error()
}

// newChatID returns a client-side conversation id in shiori.ai's id style.
func newChatID() string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 25)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b)
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func mustJSON(s string) []byte {
	b, _ := json.Marshal(s)
	return b
}

var _ = fmt.Sprintf
