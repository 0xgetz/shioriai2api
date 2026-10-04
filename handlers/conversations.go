// Package handlers implements the OpenAI-compatible HTTP surface backed by
// the shiori.ai API.
package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"sync"
	"time"

	"github.com/0xgetz/shioriai2api/shiori"
)

// Conversation is one live shiori.ai chat session plus the bookkeeping needed
// to continue it. shiori.ai keeps history server-side, keyed by chatId, so a
// conversation only needs to remember the id, the model, and usage.
type Conversation struct {
	mu        sync.Mutex
	Client    *shiori.Client
	ChatID    string
	ModelID   string
	Turn      int
	LastUsage map[string]int
	LastUsed  time.Time
}

// Lock guards the conversation for one completion round trip.
func (c *Conversation) Lock() { c.mu.Lock() }

// Unlock refreshes the idle timer and releases the turn lock.
func (c *Conversation) Unlock() {
	c.LastUsed = time.Now()
	c.mu.Unlock()
}

// ConvStore maps derived prefix keys and chat ids to conversations, with an
// LRU cap and a TTL used to forget idle entries.
type ConvStore struct {
	ttl      time.Duration
	maxSize  int
	mu       sync.Mutex
	byPrefix map[string]*Conversation
	byChat   map[string]*Conversation
}

// NewConvStore builds an empty store with a TTL and LRU cap.
func NewConvStore(ttl time.Duration, maxSize int) *ConvStore {
	return &ConvStore{
		ttl:      ttl,
		maxSize:  maxSize,
		byPrefix: make(map[string]*Conversation),
		byChat:   make(map[string]*Conversation),
	}
}

// ByPrefix looks up a conversation by its derived history key.
func (s *ConvStore) ByPrefix(key string) *Conversation {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.byPrefix[key]
}

// ByChat looks up a conversation by its shiori.ai chat id.
func (s *ConvStore) ByChat(chatID string) *Conversation {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.byChat[chatID]
}

// Register stores a conversation under a derived prefix key; the chat-id
// mapping is always kept so conversation_id passthrough keeps working.
func (s *ConvStore) Register(prefixKey string, conv *Conversation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byChat[conv.ChatID] = conv
	if prefixKey != "" {
		s.byPrefix[prefixKey] = conv
	}
	for len(s.byChat) > s.maxSize {
		var oldestKey string
		var oldest *Conversation
		for key, c := range s.byChat {
			if oldest == nil || c.LastUsed.Before(oldest.LastUsed) {
				oldestKey, oldest = key, c
			}
		}
		s.removeLocked(oldestKey, oldest)
	}
}

// removeLocked drops a conversation from both maps; callers hold s.mu.
func (s *ConvStore) removeLocked(chatKey string, conv *Conversation) {
	delete(s.byChat, chatKey)
	for k, v := range s.byPrefix {
		if v == conv {
			delete(s.byPrefix, k)
		}
	}
}

// SweepLoop forgets conversations idle past the TTL. shiori.ai history is
// server-side, so no upstream deletion is required.
func (s *ConvStore) SweepLoop(ctx context.Context) {
	interval := s.ttl / 6
	if interval < time.Minute {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sweepOnce()
		}
	}
}

func (s *ConvStore) sweepOnce() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, c := range s.byChat {
		if time.Since(c.LastUsed) >= s.ttl {
			s.removeLocked(key, c)
		}
	}
	log.Printf("conversation sweep: %d active", len(s.byChat))
}

// PrefixKey derives the cache key for "messages except the last one", so a
// client that appends one exchange per request keeps hitting the same
// shiori.ai conversation. The model is part of the key.
func PrefixKey(messages []chatMessage, model string) string {
	h := sha256.New()
	for _, m := range messages {
		h.Write([]byte(m.Role))
		h.Write([]byte{0x1f})
		h.Write([]byte(contentFingerprint(m.Content)))
		h.Write([]byte{0x1e})
	}
	h.Write([]byte{0x00})
	h.Write([]byte(model))
	return hex.EncodeToString(h.Sum(nil)[:16])
}
