package stream

import (
	"context"
	"sync"
	"testing"
)

// Iterators can be created concurrently: the validation of their requests must be thread safe.
// The data race is detected with "go test -race" (without -race it may crash the test binary).
func TestConcurrentValidateStreamIteratorRequest(t *testing.T) {
	requests := []string{
		`{"iteratorType":"FIRST_MESSAGE"}`,
		`{"iteratorType":"AFTER_LAST_MESSAGE","maxWaitTimeSeconds":5}`,
		`{"iteratorType":"AT_MESSAGE_ID","messageId":10}`,
		`{"iteratorType":"AT_TIMESTAMP","timestamp":"2026-10-15T12:00:00Z"}`,
	}
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(request string) {
			defer wg.Done()
			if err := ValidateStreamIteratorRequest(context.Background(), []byte(request)); err != nil {
				t.Errorf("request %s: %v", request, err)
			}
		}(requests[i%len(requests)])
	}
	wg.Wait()
}
