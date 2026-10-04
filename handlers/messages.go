package handlers

import (
	"encoding/json"
	"strconv"
	"strings"
)

// chatMessage is the OpenAI message shape. Content is raw so both a plain
// string and an array of parts are accepted.
type chatMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// contentText extracts the text of a message, accepting either a plain
// string or the OpenAI parts array.
func contentText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) == nil {
		var b strings.Builder
		for _, p := range parts {
			if p.Type == "text" || p.Type == "" {
				b.WriteString(p.Text)
			}
		}
		return b.String()
	}
	return string(raw)
}

// contentFingerprint builds the conversation-prefix hash input for one
// message: its text. It never touches the network.
func contentFingerprint(raw json.RawMessage) string {
	return contentText(raw)
}

// reasoningEffortValues are the efforts shiori.ai accepts.
var reasoningEffortValues = map[string]struct{}{
	"max": {}, "xhigh": {}, "high": {}, "medium": {},
	"low": {}, "minimal": {}, "none": {},
}

// normalizeEffort clamps an arbitrary effort string to a supported value,
// returning ok=false when the caller supplied an invalid one.
func normalizeEffort(s string) (string, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return "", false
	}
	if _, ok := reasoningEffortValues[s]; ok {
		return s, true
	}
	return "", false
}

func atoiDefault(s string, def int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
		return n
	}
	return def
}
