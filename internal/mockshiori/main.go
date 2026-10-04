// Command mockshiori emulates the subset of the shiori.ai API that
// shioriai2api uses, so the proxy can be exercised end-to-end without real
// credentials. It is used by scripts/smoke_test.sh and by developers.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:19192", "listen address")
	flag.Parse()

	mux := http.NewServeMux()

	// POST /api/auth — exchanges a refresh token for a JWT.
	mux.HandleFunc("/api/auth", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Action string `json:"action"`
			Args   struct {
				RefreshToken string `json:"refreshToken"`
			} `json:"args"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Args.RefreshToken == "" {
			http.Error(w, `{"error":"Authentication required"}`, http.StatusUnauthorized)
			return
		}
		// A fake but structurally valid JWT with an exp one hour out.
		token := "eyJhbGciOiJSUzI1NiJ9." + b64url(`{"sub":"mock-user","exp":4102444800}`) + ".mock-signature"
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tokens": map[string]any{"token": token, "refreshToken": body.Args.RefreshToken},
		})
	})

	// GET /api/models/catalog — public model catalog.
	mux.HandleFunc("/api/models/catalog", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"id":    "shiori",
				"title": "Shiori",
				"models": []map[string]any{
					{"id": "glm-5.3-flash", "name": "GLM-5.3 Flash", "reasoning": true, "isEnabled": true},
					{"id": "gpt-6-luna", "name": "GPT-6 Luna", "reasoning": true, "isEnabled": true},
					{"id": "gemini-2.5-flash", "name": "Gemini 2.5 Flash", "reasoning": false, "isEnabled": true},
				},
			},
		})
	})

	// POST /api/chat — SSE UI Message Stream, requires a bearer token.
	mux.HandleFunc("/api/chat", func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			http.Error(w, `{"error":"Authentication required"}`, http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		f, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "no flusher", http.StatusInternalServerError)
			return
		}
		frame := func(v any) {
			raw, _ := json.Marshal(v)
			fmt.Fprintf(w, "data: %s\n\n", raw)
			f.Flush()
		}
		frame(map[string]any{"type": "start", "messageId": "msg_mock_1",
			"messageMetadata": map[string]any{"model": "glm-5.3-flash", "chatId": "mock-chat"}})
		frame(map[string]any{"type": "start-step"})
		frame(map[string]any{"type": "text-start", "id": "t1"})
		frame(map[string]any{"type": "text-delta", "id": "t1", "delta": "Hello "})
		frame(map[string]any{"type": "text-delta", "id": "t1", "delta": "from mock!"})
		frame(map[string]any{"type": "text-end", "id": "t1"})
		frame(map[string]any{"type": "finish-step"})
		frame(map[string]any{"type": "finish", "finishReason": "stop",
			"messageMetadata": map[string]any{"model": "glm-5.3-flash", "inputTokens": 11, "outputTokens": 5, "totalTokens": 16}})
		fmt.Fprint(w, "data: [DONE]\n\n")
		f.Flush()
	})

	log.Printf("mock shiori listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}

func b64url(s string) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	var b strings.Builder
	data := []byte(s)
	for i := 0; i < len(data); i += 3 {
		var n uint32
		rem := len(data) - i
		n = uint32(data[i]) << 16
		if rem > 1 {
			n |= uint32(data[i+1]) << 8
		}
		if rem > 2 {
			n |= uint32(data[i+2])
		}
		b.WriteByte(alphabet[(n>>18)&63])
		b.WriteByte(alphabet[(n>>12)&63])
		if rem > 1 {
			b.WriteByte(alphabet[(n>>6)&63])
		}
		if rem > 2 {
			b.WriteByte(alphabet[n&63])
		}
	}
	return b.String()
}
