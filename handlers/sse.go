package handlers

import (
	"encoding/json"
	"net/http"
)

// sseWriter emits OpenAI chat.completion.chunk frames.
type sseWriter struct {
	w            http.ResponseWriter
	flusher      http.Flusher
	id           string
	model        string
	created      int64
	conversation string
	opened       bool
	failed       bool
}

func newSSEWriter(w http.ResponseWriter, id, model string, created int64, conversation string) *sseWriter {
	return &sseWriter{w: w, id: id, model: model, created: created, conversation: conversation}
}

func (s *sseWriter) open() error {
	h := s.w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	f, ok := s.w.(http.Flusher)
	if !ok {
		s.failed = true
		return errNoFlusher
	}
	s.flusher = f
	s.opened = true
	return s.send(map[string]any{"role": "assistant"}, nil)
}

func (s *sseWriter) delta(text string) {
	s.emit(map[string]any{"content": text})
}

func (s *sseWriter) deltaThink(text string) {
	s.emit(map[string]any{"reasoning_content": text})
}

func (s *sseWriter) emit(d map[string]any) {
	if s.failed || !s.opened || len(d) == 0 {
		return
	}
	if err := s.send(d, nil); err != nil {
		s.failed = true
	}
}

func (s *sseWriter) finish(usage map[string]int, finishReason string, includeUsage bool) {
	if s.failed {
		return
	}
	if finishReason == "" {
		finishReason = "stop"
	}
	if err := s.send(map[string]any{}, &finishReason); err != nil {
		s.failed = true
		return
	}
	if includeUsage && usage != nil {
		s.writeFrame(map[string]any{
			"id":              s.id,
			"object":          "chat.completion.chunk",
			"created":         s.created,
			"model":           s.model,
			"conversation_id": s.conversation,
			"choices":         []any{},
			"usage":           usage,
		})
	}
	s.writeRaw("data: [DONE]\n\n")
}

// errorText reports an upstream failure on a stream that already started.
func (s *sseWriter) errorText(msg string) {
	if s.failed || !s.opened {
		return
	}
	stop := "stop"
	if err := s.send(map[string]any{"error": msg}, &stop); err != nil {
		s.failed = true
		return
	}
	s.writeRaw("data: [DONE]\n\n")
}

func (s *sseWriter) send(delta map[string]any, finish *string) error {
	return s.writeFrame(map[string]any{
		"id":              s.id,
		"object":          "chat.completion.chunk",
		"created":         s.created,
		"model":           s.model,
		"conversation_id": s.conversation,
		"choices": []any{map[string]any{
			"index":         0,
			"delta":         delta,
			"finish_reason": finish,
		}},
	})
}

func (s *sseWriter) writeFrame(frame map[string]any) error {
	raw, err := json.Marshal(frame)
	if err != nil {
		return err
	}
	return s.writeRaw("data: " + string(raw) + "\n\n")
}

func (s *sseWriter) writeRaw(str string) error {
	if _, err := s.w.Write([]byte(str)); err != nil {
		s.failed = true
		return err
	}
	if s.flusher != nil {
		s.flusher.Flush()
	}
	return nil
}

type flusherError struct{}

func (flusherError) Error() string { return "response writer does not support flushing" }

var errNoFlusher = flusherError{}
