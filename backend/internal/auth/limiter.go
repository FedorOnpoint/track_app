package auth

import (
	"sync"
	"time"
)

type rateEntry struct {
	count int
	reset time.Time
}
type RateLimiter struct {
	mu              sync.Mutex
	entries         map[string]rateEntry
	limit, capacity int
	window          time.Duration
}

func NewRateLimiter(limit int, window time.Duration, capacity int) *RateLimiter {
	return &RateLimiter{entries: make(map[string]rateEntry), limit: limit, window: window, capacity: capacity}
}

func (limiter *RateLimiter) Allow(key string, now time.Time) bool {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	entry, exists := limiter.entries[key]
	if !exists || !now.Before(entry.reset) {
		entry = rateEntry{reset: now.Add(limiter.window)}
	}
	if entry.count >= limiter.limit {
		limiter.entries[key] = entry
		return false
	}
	entry.count++
	limiter.entries[key] = entry
	if len(limiter.entries) > limiter.capacity {
		limiter.evict(now)
	}
	return true
}

func (limiter *RateLimiter) evict(now time.Time) {
	for key, entry := range limiter.entries {
		if !now.Before(entry.reset) {
			delete(limiter.entries, key)
		}
	}
	for len(limiter.entries) > limiter.capacity {
		for key := range limiter.entries {
			delete(limiter.entries, key)
			break
		}
	}
}

func (limiter *RateLimiter) Size() int {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	return len(limiter.entries)
}
