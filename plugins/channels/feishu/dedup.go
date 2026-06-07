package feishu

import (
	"sync"
	"time"
)

// Deduplicator tracks seen message IDs and deduplicates based on TTL.
type Deduplicator struct {
	seen map[string]time.Time
	mu   sync.Mutex
	ttl  time.Duration
	done chan struct{}
}

// NewDeduplicator creates a new Deduplicator with the given TTL.
// A background goroutine runs periodically to clean up expired entries.
func NewDeduplicator(ttl time.Duration) *Deduplicator {
	d := &Deduplicator{
		seen: make(map[string]time.Time),
		ttl:  ttl,
		done: make(chan struct{}),
	}
	go d.cleanup()
	return d
}

// Stop signals the cleanup goroutine to exit.
func (d *Deduplicator) Stop() {
	close(d.done)
}

// IsDuplicate checks if a message ID has been seen before.
// If not seen, it records the message ID and returns false.
// If already seen, it returns true.
// Uses a single lock for the entire check-and-insert to prevent TOCTOU races.
func (d *Deduplicator) IsDuplicate(messageID string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.seen[messageID]; exists {
		return true
	}
	d.seen[messageID] = time.Now()
	return false
}

// cleanup periodically removes expired entries from the seen map.
func (d *Deduplicator) cleanup() {
	ticker := time.NewTicker(d.ttl)
	defer ticker.Stop()

	for {
		select {
		case <-d.done:
			return
		case <-ticker.C:
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
}
