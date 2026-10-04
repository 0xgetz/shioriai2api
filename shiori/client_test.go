package shiori

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestJWTExpiry(t *testing.T) {
	now := time.Unix(1700000000, 0)
	payload := `{"sub":"u","exp":1700003600}`
	token := "h." + b64(payload) + ".s"
	got := jwtExpiry(token, now)
	want := time.Unix(1700003600, 0).Add(-60 * time.Second)
	if !got.Equal(want) {
		t.Fatalf("jwtExpiry = %v, want %v", got, want)
	}
	// Malformed token falls back to ~1h.
	fallback := jwtExpiry("garbage", now)
	if fallback.Before(now.Add(50 * time.Minute)) {
		t.Fatalf("fallback expiry too soon: %v", fallback)
	}
}

func TestCatalogParses(t *testing.T) {
	raw := `[{"id":"shiori","title":"Shiori","models":[{"id":"glm-5.3-flash","name":"GLM-5.3 Flash","reasoning":true,"isEnabled":true}]}]`
	var groups []CatalogGroup
	if err := json.Unmarshal([]byte(raw), &groups); err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 || len(groups[0].Models) != 1 {
		t.Fatalf("unexpected groups: %+v", groups)
	}
	m := groups[0].Models[0]
	if m.ID != "glm-5.3-flash" || !m.Reasoning || !m.IsEnabled {
		t.Fatalf("bad model: %+v", m)
	}
}

func TestReadEventStreamParsesDeltas(t *testing.T) {
	stream := strings.Join([]string{
		`data: {"type":"start","messageId":"msg_1","messageMetadata":{"chatId":"c1"}}`,
		"",
		`data: {"type":"start-step"}`,
		"",
		`data: {"type":"reasoning-delta","delta":"think "}`,
		"",
		`data: {"type":"text-delta","delta":"Hello "}`,
		"",
		`data: {"type":"text-delta","delta":"world"}`,
		"",
		`data: {"type":"finish","finishReason":"stop","messageMetadata":{"inputTokens":3,"outputTokens":2,"totalTokens":5}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")

	var gotText, gotThink string
	var finish string
	var meta map[string]any
	cb := &StreamCallbacks{
		OnReasoningDelta: func(s string) { gotThink += s },
		OnTextDelta:      func(s string) { gotText += s },
		OnFinish:         func(r string, m map[string]any) { finish = r; meta = m },
	}
	if err := readEventStream(strings.NewReader(stream), cb); err != nil {
		t.Fatalf("readEventStream: %v", err)
	}
	if gotText != "Hello world" {
		t.Fatalf("text = %q", gotText)
	}
	if gotThink != "think " {
		t.Fatalf("think = %q", gotThink)
	}
	if finish != "stop" {
		t.Fatalf("finish = %q", finish)
	}
	if meta == nil || meta["totalTokens"] != float64(5) {
		t.Fatalf("meta = %+v", meta)
	}
}

func TestContextCancellation(t *testing.T) {
	// A cancelled context must surface as an error, not hang.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := NewClient("dummy", "", "", "")
	if _, err := c.Token(ctx); err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func b64(s string) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	data := []byte(s)
	var b strings.Builder
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
