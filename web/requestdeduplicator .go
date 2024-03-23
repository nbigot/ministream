package web

import (
	"sync"
	"time"
)

type RequestDeduplicatorManager struct {
	mu           sync.Mutex
	processedIDs map[string]time.Time
	ttl          time.Duration // Time to live for each requestID
	stopChan     chan struct{} // Channel to stop the cleanup goroutine
	wg           sync.WaitGroup
}

func (rdm *RequestDeduplicatorManager) Init() {
	rdm.stopChan = make(chan struct{})
	rdm.wg.Add(1)
	go func() {
		defer rdm.wg.Done()
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				rdm.CleanupExpiredEntries()
			case <-rdm.stopChan:
				return
			}
		}
	}()
}

func (rdm *RequestDeduplicatorManager) Stop() {
	close(rdm.stopChan)
	rdm.wg.Wait()
	rdm.Clear()
}

func (rdm *RequestDeduplicatorManager) Exists(requestID string) bool {
	rdm.mu.Lock()
	defer rdm.mu.Unlock()

	_, exists := rdm.processedIDs[requestID]
	return exists
}

func (rdm *RequestDeduplicatorManager) Add(requestID string) {
	rdm.mu.Lock()
	rdm.processedIDs[requestID] = time.Now()
	rdm.mu.Unlock()
}

func (rdm *RequestDeduplicatorManager) Remove(requestID string) {
	rdm.mu.Lock()
	delete(rdm.processedIDs, requestID)
	rdm.mu.Unlock()
}

// cleanupExpiredEntries periodically removes expired entries
func (rdm *RequestDeduplicatorManager) CleanupExpiredEntries() {
	rdm.mu.Lock()
	for requestID, timestamp := range rdm.processedIDs {
		if time.Since(timestamp) > rdm.ttl {
			delete(rdm.processedIDs, requestID)
		}
	}
	rdm.mu.Unlock()
}

func (rdm *RequestDeduplicatorManager) Clear() {
	rdm.mu.Lock()
	rdm.processedIDs = make(map[string]time.Time)
	rdm.mu.Unlock()
}

func NewRequestDeduplicatorManager(ttl time.Duration) *RequestDeduplicatorManager {
	return &RequestDeduplicatorManager{
		processedIDs: make(map[string]time.Time),
		ttl:          ttl,
	}
}
