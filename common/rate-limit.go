package common

import (
	"sync"
	"time"
)

type InMemoryRateLimiter struct {
	store              map[string]*[]int64
	mutex              sync.Mutex
	expirationDuration time.Duration
}

func (l *InMemoryRateLimiter) Init(expirationDuration time.Duration) {
	if l.store == nil {
		l.mutex.Lock()
		if l.store == nil {
			l.store = make(map[string]*[]int64)
			l.expirationDuration = expirationDuration
			if expirationDuration > 0 {
				go l.clearExpiredItems()
			}
		}
		l.mutex.Unlock()
	}
}

func (l *InMemoryRateLimiter) clearExpiredItems() {
	for {
		time.Sleep(l.expirationDuration)
		l.mutex.Lock()
		now := time.Now().Unix()
		for key := range l.store {
			queue := l.store[key]
			size := len(*queue)
			if size == 0 || now-(*queue)[size-1] > int64(l.expirationDuration.Seconds()) {
				delete(l.store, key)
			}
		}
		l.mutex.Unlock()
	}
}

// Request parameter duration's unit is seconds
func (l *InMemoryRateLimiter) Request(key string, maxRequestNum int, duration int64) bool {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	// [old <-- new]
	queue, ok := l.store[key]
	now := time.Now().Unix()
	if ok {
		if len(*queue) < maxRequestNum {
			*queue = append(*queue, now)
			return true
		} else {
			if now-(*queue)[0] >= duration {
				*queue = (*queue)[1:]
				*queue = append(*queue, now)
				return true
			} else {
				return false
			}
		}
	} else {
		s := make([]int64, 0, maxRequestNum)
		l.store[key] = &s
		*(l.store[key]) = append(*(l.store[key]), now)
	}
	return true
}

// GetUsage returns the number of active hits within the sliding window [now-duration, now]
// and the Unix timestamp when the oldest active hit expires.
func (l *InMemoryRateLimiter) GetUsage(key string, duration int64) (int, int64) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if l.store == nil {
		return 0, 0
	}
	queue, ok := l.store[key]
	if !ok || queue == nil || len(*queue) == 0 {
		return 0, 0
	}
	now := time.Now().Unix()
	cutoff := now - duration
	count := 0
	var oldest int64
	for _, ts := range *queue {
		if ts > cutoff {
			if count == 0 {
				oldest = ts
			}
			count++
		}
	}
	if count == 0 {
		return 0, 0
	}
	return count, oldest + duration
}
