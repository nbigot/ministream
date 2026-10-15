package main

import (
	"flag"
	"fmt"
	"sync"
	"time"

	json "github.com/goccy/go-json"
	"github.com/valyala/fasthttp"
)

// concurrentIterators creates records iterators concurrently on a running server, over warm
// keep-alive connections, and stops at the first failed request (server crashed).
func concurrentIterators(args []string) error {
	fs := flag.NewFlagSet("iterators", flag.ExitOnError)
	addr := fs.String("addr", "127.0.0.1:8080", "server address")
	workers := fs.Int("workers", 8, "concurrent requests")
	rounds := fs.Int("rounds", 100, "rounds (each round creates then closes workers iterators)")
	_ = fs.Parse(args)

	b := &bench{addr: *addr, client: &fasthttp.HostClient{Addr: *addr, MaxConns: *workers, ReadTimeout: 10 * time.Second}}
	stream, err := b.createStream("iterators")
	if err != nil {
		return err
	}
	if code, body, err := b.do("PUT", "/api/v1/stream/"+stream+"/records", []byte(`[{"a":1},{"a":2}]`), ""); err != nil || code != 202 {
		return fmt.Errorf("put records: %d %v %s", code, err, body)
	}
	// open the connections first, so that the iterators requests arrive at the same time
	var warm sync.WaitGroup
	for w := 0; w < *workers; w++ {
		warm.Add(1)
		go func() {
			defer warm.Done()
			_, _, _ = b.do("GET", "/api/v1/utils/ping", nil, "")
		}()
	}
	warm.Wait()
	for round := 1; round <= *rounds; round++ {
		var wg, ready sync.WaitGroup
		start := make(chan struct{})
		errs := make(chan string, *workers)
		ready.Add(*workers)
		for w := 0; w < *workers; w++ {
			wg.Add(1)
			go func(w int) {
				defer wg.Done()
				body := []byte(fmt.Sprintf(`{"iteratorType":"FIRST_MESSAGE","name":"it-%d-%d"}`, round, w))
				ready.Done()
				<-start
				code, resp, err := b.do("POST", "/api/v1/stream/"+stream+"/iterator", body, "")
				if err != nil || code != 200 {
					errs <- fmt.Sprintf("status %d err %v body %.200s", code, err, resp)
					return
				}
				var it struct {
					UUID string `json:"streamIteratorUUID"`
				}
				if json.Unmarshal(resp, &it) == nil {
					_, _, _ = b.do("DELETE", "/api/v1/stream/"+stream+"/iterator/"+it.UUID, nil, "")
				}
			}(w)
		}
		ready.Wait()
		close(start)
		wg.Wait()
		close(errs)
		if e, failed := <-errs; failed {
			fmt.Printf("round %d: request failed: %s\n", round, e)
			if code, _, err := b.do("GET", "/api/v1/utils/ping", nil, ""); err != nil || code != 200 {
				fmt.Printf("server is down (ping: %d %v)\n", code, err)
			}
			return nil
		}
	}
	fmt.Printf("ok: %d rounds of %d concurrent iterator creations\n", *rounds, *workers)
	return nil
}
