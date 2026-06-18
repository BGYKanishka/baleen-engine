package api

import (
	"sync"
	"time"
)

var (
	lastActive time.Time
	activeMu   sync.Mutex
)

func UpdateHeartbeat() {
	activeMu.Lock()
	defer activeMu.Unlock()
	lastActive = time.Now()
}

func idleTime() time.Duration {
	activeMu.Lock()
	defer activeMu.Unlock()
	return time.Since(lastActive)
}
