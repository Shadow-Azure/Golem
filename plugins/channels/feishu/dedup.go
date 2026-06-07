package feishu

import (
	"sync"
	"time"
)

// Deduplicator tracks seen message IDs and deduplicates based on TTL.
type Deduplicator struct {
	seen map[string]time.Time
	mu   sync.RWMutex
	ttl  time.Duration
}

// NewDeduplicator creates a new Deduplicator with the given TTL.
// A background goroutine runs periodically to clean up expired entries.
func NewDeduplicator(ttl time.Duration) *Deduplicator {
	d := &Deduplicator{
		seen: make(map[string]time.Time),
		ttl:  ttl,
	}
	go d.cleanup()
	return d
}

// IsDuplicate checks if a message ID has been seen before.
// If not seen, it records the message ID and returns false.
// If already seen, it returns true.
func (d *Deduplicator) IsDuplicate(messageID string) bool {
	d.mu.RLock()
	_, exists := d.seen[messageID]
	d.mu.RUnlock()

	if exists {
		return true
	}

	d.mu.Lock()
	d.seen[messageID] = time.Now()
	d.mu.Unlock()

	return false
}

// cleanup periodically removes expired entries from the seen map.
func (d *Deduplicator) cleanup() {
	ticker := time.NewTicker(d.ttl)
	defer ticker.Stop()

	for range ticker.C {
		d.mu.Lock()
		cutoff := time.Now().Add(-d.ttl)
		for id, ts := range d.seen {
			if ts.Before(cutoff) {
				delete(d.seen, id)
			}
		}
		d.mu.Unlock()
	}
}
