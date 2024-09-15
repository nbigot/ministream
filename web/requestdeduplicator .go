package web

import (
	"errors"
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
	// Start the cleanup goroutine to remove expired entries periodically
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

func (rdm *RequestDeduplicatorManager) Stop() error {
	if rdm.stopChan == nil {
		// the manager has not been initialized
		return errors.New("RequestDeduplicatorManager has not been initialized")
	}
	close(rdm.stopChan)
	rdm.wg.Wait()
	rdm.Clear()
	return nil
}

func (rdm *RequestDeduplicatorManager) Exists(requestID string) bool {
	rdm.mu.Lock()
	defer rdm.mu.Unlock()

	_, exists := rdm.processedIDs[requestID]
	return exists
}

func (rdm *RequestDeduplicatorManager) Add(requestID string) {
	// Add the requestID to the map
	// a requestID is considered as processed when it is added to the map
	// a requestID is composed of "<stream UUID>:<request batch ID>"
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
	// In order to keep the map small in memory, we remove expired entries from the map periodically.
	// Keep the last n minutes of requestIDs in the map.
	// It means that if a new request with a same requestID is received after n minutes,
	// it will not be considered as a duplicate.
	// This is a trade-off between memory and performance.
	rdm.mu.Lock()
	for requestID, timestamp := range rdm.processedIDs {
		if time.Since(timestamp) > rdm.ttl {
			delete(rdm.processedIDs, requestID)
		}
	}
	rdm.mu.Unlock()
}

func (rdm *RequestDeduplicatorManager) Clear() {
	// Clear the map (remove all requestIDs from the map)
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
