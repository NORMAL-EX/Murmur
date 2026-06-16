// Package ratelimit implements a simple in-memory sliding-window limiter keyed
// by user id. It is safe for concurrent use.
package ratelimit

import (
	"sync"
	"time"
)

type Limiter struct {
	mu   sync.Mutex
	hits map[uint][]time.Time
}

func New() *Limiter {
	return &Limiter{hits: map[uint][]time.Time{}}
}

// Allow reports whether the user may perform an action now, given a limit of
// `limit` actions per `window`. limit <= 0 means unlimited. When blocked it
// returns the number of seconds the caller should wait before retrying.
func (l *Limiter) Allow(userID uint, limit int, window time.Duration) (bool, int) {
	if limit <= 0 {
		return true, 0
	}
	now := time.Now()
	cutoff := now.Add(-window)

	l.mu.Lock()
	defer l.mu.Unlock()

	times := l.hits[userID]
	kept := times[:0]
	for _, t := range times {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}

	if len(kept) >= limit {
		retry := int(window.Seconds()-now.Sub(kept[0]).Seconds()) + 1
		if retry < 1 {
			retry = 1
		}
		l.hits[userID] = kept
		return false, retry
	}

	kept = append(kept, now)
	l.hits[userID] = kept
	return true, 0
}
