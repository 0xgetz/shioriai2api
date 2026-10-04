package shiori

import (
	"bufio"
	cryptorand "crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// readEventStream consumes the shiori.ai chat response: a Vercel AI SDK v5
// "UI Message Stream" carried over SSE. It dispatches each complete frame to
// the matching callback and ends at "data: [DONE]" or EOF.
func readEventStream(r io.Reader, cb *StreamCallbacks) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)

	var dataLines []string
	dispatch := func() error {
		if len(dataLines) == 0 {
			return nil
		}
		payload := strings.Join(dataLines, "\n")
		dataLines = nil
		if payload == "[DONE]" {
			return errStreamDone
		}
		var frame struct {
			Type            string         `json:"type"`
			Delta           string         `json:"delta"`
			MessageID       string         `json:"messageId"`
			FinishReason    string         `json:"finishReason"`
			MessageMetadata map[string]any `json:"messageMetadata"`
		}
		if err := json.Unmarshal([]byte(payload), &frame); err != nil {
			return nil // tolerate unknown frames
		}
		switch frame.Type {
		case "start":
			if cb.OnStart != nil {
				cb.OnStart(frame.MessageID, frame.MessageMetadata)
			}
		case "reasoning-delta":
			if frame.Delta != "" && cb.OnReasoningDelta != nil {
				cb.OnReasoningDelta(frame.Delta)
			}
		case "text-delta":
			if frame.Delta != "" && cb.OnTextDelta != nil {
				cb.OnTextDelta(frame.Delta)
			}
		case "finish":
			if cb.OnFinish != nil {
				cb.OnFinish(frame.FinishReason, frame.MessageMetadata)
			}
		}
		return nil
	}

	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		switch {
		case line == "":
			if err := dispatch(); err != nil {
				if errors.Is(err, errStreamDone) {
					return nil
				}
				return err
			}
		case strings.HasPrefix(line, ":"):
			// comment / keep-alive
		case strings.HasPrefix(line, "event:"):
			// event name is ignored; the frame's "type" field is authoritative
		case strings.HasPrefix(line, "data:"):
			v := strings.TrimPrefix(line[len("data:"):], " ")
			dataLines = append(dataLines, v)
		}
	}
	if err := dispatch(); err != nil && !errors.Is(err, errStreamDone) {
		return err
	}
	if err := sc.Err(); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

// errStreamDone is an internal sentinel for "data: [DONE]".
var errStreamDone = errors.New("shiori: stream done")

func decodeSegment(s string) ([]byte, error) {
	if b, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	return base64.URLEncoding.DecodeString(s)
}

// errorsAs is a tiny wrapper so client.go stays free of the errors import.
func errorsAs(err error, target **APIError) bool {
	if err == nil {
		return false
	}
	return errors.As(err, target)
}

func newUUID() string {
	var b [16]byte
	if _, err := cryptorand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
