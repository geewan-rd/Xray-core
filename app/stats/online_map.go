package stats

import (
	"sync"
	"time"
)

// OnlineMap is an implementation of stats.OnlineMap.
type OnlineMap struct {
	access        sync.RWMutex
	value         int
	ipList        map[string]time.Time
	lastCleanup   time.Time
	cleanupPeriod time.Duration
}

// NewOnlineMap creates a new instance of OnlineMap.
func NewOnlineMap() *OnlineMap {
	return &OnlineMap{
		ipList:        make(map[string]time.Time),
		lastCleanup:   time.Now(),
		cleanupPeriod: 10 * time.Second,
	}
}

// Count implements stats.OnlineMap.
func (c *OnlineMap) Count() int {
	c.access.RLock()
	defer c.access.RUnlock()
	return c.value
}

// List implements stats.OnlineMap.
func (c *OnlineMap) List() []string {
	return c.GetKeys()
}

// AddIP implements stats.OnlineMap.
func (c *OnlineMap) AddIP(ip string) {
	if ip == "127.0.0.1" {
		return
	}

	c.access.Lock()
	defer c.access.Unlock()

	c.ipList[ip] = time.Now()

	if time.Since(c.lastCleanup) > c.cleanupPeriod {
		c.removeExpiredIPsLocked()
		c.lastCleanup = time.Now()
	}

	c.value = len(c.ipList)
}

func (c *OnlineMap) GetKeys() []string {
	c.access.RLock()
	defer c.access.RUnlock()

	keys := make([]string, 0, len(c.ipList))
	for k := range c.ipList {
		keys = append(keys, k)
	}
	return keys
}

// removeExpiredIPsLocked removes expired entries from c.ipList.
// The caller must hold c.access (write lock).
func (c *OnlineMap) removeExpiredIPsLocked() {
	now := time.Now()
	for k, t := range c.ipList {
		if now.Sub(t).Seconds() > 20 {
			delete(c.ipList, k)
		}
	}
}
