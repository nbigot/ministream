// Command loader benchmarks a ministream server binary (Linux only, see README.md).
//
// It starts the server pinned to some CPUs (taskset), runs the benchmark phases against it
// over HTTP, measures the CPU time and memory of the server process (/proc), and writes the
// results as JSON. The "report" sub command aggregates the result files into markdown tables.
//
// Usage:
//
//	loader run -impl current -storage inmemory -server bin/ministream-current -config config/inmemory.yaml -out results/current-inmemory-1.json
//	loader report [-impls current,other] results/*.json
//	loader iterators -workers 8 -rounds 100   (creates iterators concurrently on a running server)
package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
	json "github.com/goccy/go-json"
	"github.com/valyala/fasthttp"
)

const clockTicksPerSecond = 100 // CLK_TCK on linux/amd64

type Phase struct {
	Name          string             `json:"name"`
	Concurrency   int                `json:"concurrency"`
	Requests      int64              `json:"requests"`
	Records       int64              `json:"records"`
	Errors        int64              `json:"errors"`
	WallSec       float64            `json:"wallSec"`
	PersistSec    float64            `json:"persistSec,omitempty"` // until every record is stored (mysql, jsonfile)
	ReqPerSec     float64            `json:"reqPerSec"`
	RecPerSec     float64            `json:"recPerSec"`
	LatencyMs     map[string]float64 `json:"latencyMs"`
	ServerCPUSec  float64            `json:"serverCpuSec"`
	ServerRSSMB   float64            `json:"serverRssMB"`
	ServerThreads int                `json:"serverThreads"`
	Connections   int64              `json:"connections"`     // TCP connections opened by the loader
	Retries       int64              `json:"retries"`         // requests sent again because the server closed the connection
	MySQL         map[string]int64   `json:"mysql,omitempty"` // global status counters delta
}

type Result struct {
	Impl        string   `json:"impl"`
	Storage     string   `json:"storage"`
	Run         int      `json:"run"`
	Server      string   `json:"server"`
	BinaryMB    float64  `json:"binaryMB"`
	ServerCPUs  string   `json:"serverCpus"`
	StartupMs   float64  `json:"startupMs"`
	IdleRSSMB   float64  `json:"idleRssMB"`
	PeakRSSMB   float64  `json:"peakRssMB"`
	FinalRSSMB  float64  `json:"finalRssMB"`
	DiskMB      float64  `json:"diskMB,omitempty"`
	TotalCPUSec float64  `json:"totalCpuSec"`
	Phases      []*Phase `json:"phases"`
}

type bench struct {
	impl, storage string
	addr          string
	pid           int
	client        *fasthttp.HostClient
	db            *sql.DB
	dataDir       string
	conns         atomic.Int64
	retries       atomic.Int64
	// the iterators are created one at a time: older versions crash when iterators are
	// created concurrently (data race in the validation of the iterator requests)
	createIteratorMu sync.Mutex
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: loader run|report|iterators ...")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "run":
		if err := run(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "ERROR:", err)
			os.Exit(1)
		}
	case "iterators":
		if err := concurrentIterators(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "ERROR:", err)
			os.Exit(1)
		}
	case "report":
		if err := report(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "ERROR:", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintln(os.Stderr, "unknown command", os.Args[1])
		os.Exit(2)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	impl := fs.String("impl", "", "name of the server build (column of the report)")
	storage := fs.String("storage", "", "inmemory, jsonfile or mysql")
	runNo := fs.Int("run", 1, "run number")
	server := fs.String("server", "", "server binary")
	config := fs.String("config", "", "server config file")
	cpus := fs.String("cpus", "0,1", "CPUs of the server (taskset)")
	addr := fs.String("addr", "127.0.0.1:8080", "server address")
	out := fs.String("out", "", "result file")
	logFile := fs.String("log", "", "server log file")
	mysqlDSN := fs.String("mysql-dsn", "", "DSN of a MySQL admin connection (mysql storage)")
	dataDir := fs.String("data-dir", "", "data directory of the jsonfile storage")
	pingDuration := fs.Duration("ping-duration", 10*time.Second, "duration of the ping phase")
	singleN := fs.Int("single", 100000, "records put one per request")
	batchN := fs.Int("batch", 500000, "records put by batches")
	batchSize := fs.Int("batch-size", 100, "records per batch request")
	readers := fs.Int("readers", 4, "concurrent iterators of the read phases")
	_ = fs.Parse(args)

	b := &bench{impl: *impl, storage: *storage, addr: *addr, dataDir: *dataDir}
	b.client = &fasthttp.HostClient{Addr: *addr, MaxConns: 256, ReadTimeout: 120 * time.Second, WriteTimeout: 120 * time.Second, MaxResponseBodySize: 256 << 20, MaxIdemponentCallAttempts: 300,
		Dial: func(addr string) (net.Conn, error) {
			b.conns.Add(1)
			return fasthttp.Dial(addr)
		},
		// a server closing its idle keep-alive connections makes requests fail before the
		// server reads them: send them again, whatever their method
		RetryIfErr: func(_ *fasthttp.Request, attempts int, err error) (bool, bool) {
			retry := attempts < 300 && !errors.Is(err, fasthttp.ErrTimeout) && !errors.Is(err, fasthttp.ErrBodyTooLarge)
			if retry {
				b.retries.Add(1)
			}
			return false, retry
		},
	}
	if *mysqlDSN != "" {
		db, err := sql.Open("mysql", *mysqlDSN)
		if err != nil {
			return err
		}
		db.SetMaxOpenConns(2)
		b.db = db
	}

	res := &Result{Impl: *impl, Storage: *storage, Run: *runNo, Server: *server, ServerCPUs: *cpus}
	if st, err := os.Stat(*server); err == nil {
		res.BinaryMB = float64(st.Size()) / 1e6
	}

	// start the server
	logw := io.Discard
	if *logFile != "" {
		f, err := os.Create(*logFile)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		logw = f
	}
	cmd := exec.Command("taskset", "-c", *cpus, *server, "-config", *config)
	cmd.Stdout, cmd.Stderr = logw, logw
	t0 := time.Now()
	if err := cmd.Start(); err != nil {
		return err
	}
	b.pid = cmd.Process.Pid
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	defer func() {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		select {
		case <-exited:
		case <-time.After(20 * time.Second):
			_ = cmd.Process.Kill()
		}
	}()
	for {
		if code, _, err := b.do("GET", "/api/v1/utils/ping", nil, ""); err == nil && code == 200 {
			break
		}
		select {
		case err := <-exited:
			return fmt.Errorf("server exited at startup: %v", err)
		default:
		}
		if time.Since(t0) > 60*time.Second {
			return errors.New("server not ready after 60s")
		}
		time.Sleep(2 * time.Millisecond)
	}
	res.StartupMs = float64(time.Since(t0).Microseconds()) / 1000
	time.Sleep(time.Second)
	res.IdleRSSMB, _, _ = b.procStatus()
	fmt.Printf("[%s/%s/%d] startup %.1f ms, idle rss %.1f MB\n", *impl, *storage, *runNo, res.StartupMs, res.IdleRSSMB)

	phases := []func() (*phaseResult, error){
		func() (*phaseResult, error) { return b.phasePing(64, *pingDuration) },
		func() (*phaseResult, error) { return b.phasePut("put-record", 64, *singleN, 1) },
		func() (*phaseResult, error) { return b.phasePut("put-records", 16, *batchN, *batchSize) },
	}
	var batchStream string
	for i, p := range phases {
		ph, err := p()
		if err != nil {
			return fmt.Errorf("phase %d: %w", i, err)
		}
		if ph.Name == "put-records" {
			batchStream = ph.streamUUID
		}
		res.Phases = append(res.Phases, &ph.Phase)
		printPhase(ph)
	}
	for _, rp := range []struct {
		name, jq string
		expected int
	}{
		{"read", "", *batchN},
		{"read-jq", `select(.m.n % 10 == 0) | {i, user: .m.user, value: .m.value}`, *batchN / 10},
	} {
		ph, err := b.phaseRead(rp.name, batchStream, *readers, rp.jq, rp.expected)
		if err != nil {
			return fmt.Errorf("phase %s: %w", rp.name, err)
		}
		res.Phases = append(res.Phases, &ph.Phase)
		printPhase(ph)
	}

	var hwm float64
	res.FinalRSSMB, hwm, _ = b.procStatus()
	res.PeakRSSMB = hwm
	res.TotalCPUSec = b.cpuSec()
	if b.dataDir != "" {
		res.DiskMB = float64(dirSize(b.dataDir)) / 1e6
	}
	if b.db != nil {
		// the table sizes of information_schema are cached (24 hours by default), and the
		// InnoDB statistics are updated in the background: analyze the tables first
		ctx := context.Background()
		var size sql.NullInt64
		if conn, err := b.db.Conn(ctx); err == nil {
			var tables []string
			if rows, err := conn.QueryContext(ctx, "SELECT table_name FROM information_schema.tables WHERE table_schema = 'ministream'"); err == nil {
				for rows.Next() {
					var t string
					if rows.Scan(&t) == nil {
						tables = append(tables, t)
					}
				}
				_ = rows.Close()
			}
			for _, t := range tables {
				_, _ = conn.ExecContext(ctx, "ANALYZE TABLE ministream.`"+t+"`")
			}
			_, _ = conn.ExecContext(ctx, "SET SESSION information_schema_stats_expiry = 0")
			_ = conn.QueryRowContext(ctx, "SELECT SUM(data_length + index_length) FROM information_schema.tables WHERE table_schema = 'ministream'").Scan(&size)
			_ = conn.Close()
		}
		res.DiskMB = float64(size.Int64) / 1e6
	}
	fmt.Printf("[%s/%s/%d] peak rss %.1f MB, final rss %.1f MB, cpu %.1f s, disk %.1f MB\n", *impl, *storage, *runNo, res.PeakRSSMB, res.FinalRSSMB, res.TotalCPUSec, res.DiskMB)

	data, _ := json.MarshalIndent(res, "", "  ")
	return os.WriteFile(*out, data, 0o644)
}

type phaseResult struct {
	Phase
	streamUUID string
}

func printPhase(p *phaseResult) {
	extra := ""
	if p.PersistSec > 0 {
		extra = fmt.Sprintf(" persisted in %.2fs (%.0f rec/s)", p.PersistSec, float64(p.Records)/p.PersistSec)
	}
	fmt.Printf("  %-12s c=%-3d req=%-8d rec=%-8d err=%d %.2fs %9.0f req/s %9.0f rec/s p50=%.2fms p99=%.2fms cpu=%.2fs rss=%.0fMB conns=%d retries=%d%s\n",
		p.Name, p.Concurrency, p.Requests, p.Records, p.Errors, p.WallSec, p.ReqPerSec, p.RecPerSec,
		p.LatencyMs["p50"], p.LatencyMs["p99"], p.ServerCPUSec, p.ServerRSSMB, p.Connections, p.Retries, extra)
}

// do sends a request and returns the status code and a copy of the body.
func (b *bench) do(method, uri string, body []byte, ctype string) (int, []byte, error) {
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)
	req.Header.SetMethod(method)
	req.SetRequestURI("http://" + b.addr + uri)
	if body != nil {
		req.SetBodyRaw(body)
		if ctype == "" {
			ctype = "application/json"
		}
		req.Header.SetContentType(ctype)
	}
	if err := b.client.Do(req, resp); err != nil {
		return 0, nil, err
	}
	return resp.StatusCode(), append([]byte(nil), resp.Body()...), nil
}

func (b *bench) createStream(name string) (string, error) {
	code, body, err := b.do("POST", "/api/v1/stream/", []byte(`{"properties":{"name":"`+name+`","bench":"true"}}`), "")
	if err != nil {
		return "", err
	}
	var info struct {
		UUID string `json:"uuid"`
	}
	if code != 200 && code != 201 {
		return "", fmt.Errorf("create stream: status %d: %s", code, body)
	}
	if err := json.Unmarshal(body, &info); err != nil || info.UUID == "" {
		return "", fmt.Errorf("create stream: unexpected response %s", body)
	}
	return info.UUID, nil
}

var events = []string{"click", "view", "purchase", "login", "logout"}

func appendRecord(dst []byte, n int) []byte {
	dst = append(dst, `{"user":"user-`...)
	dst = strconv.AppendInt(dst, int64(n%5000), 10)
	dst = append(dst, `","event":"`...)
	dst = append(dst, events[n%len(events)]...)
	dst = append(dst, `","n":`...)
	dst = strconv.AppendInt(dst, int64(n), 10)
	dst = append(dst, `,"value":`...)
	dst = strconv.AppendFloat(dst, float64(n%100000)/100, 'f', 2, 64)
	dst = append(dst, `,"tags":["alpha","beta"],"ok":true,"ts":"2026-10-15T12:00:00Z"}`...)
	return dst
}

// latencies merges the per worker latencies and returns the percentiles in milliseconds.
func latencies(all [][]time.Duration) map[string]float64 {
	var merged []time.Duration
	for _, l := range all {
		merged = append(merged, l...)
	}
	if len(merged) == 0 {
		return map[string]float64{}
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i] < merged[j] })
	pct := func(p float64) float64 {
		idx := int(p * float64(len(merged)-1))
		return float64(merged[idx].Microseconds()) / 1000
	}
	return map[string]float64{"p50": pct(0.50), "p90": pct(0.90), "p99": pct(0.99), "max": pct(1)}
}

// begin returns a function finishing the phase: wall time, server cpu time, rss, mysql counters.
func (b *bench) begin(name string, concurrency int) func(p *phaseResult) {
	cpu0 := b.cpuSec()
	my0 := b.mysqlStatus()
	conns0, retries0 := b.conns.Load(), b.retries.Load()
	t0 := time.Now()
	return func(p *phaseResult) {
		p.Name = name
		p.Concurrency = concurrency
		if p.WallSec == 0 {
			p.WallSec = time.Since(t0).Seconds()
		}
		p.ReqPerSec = float64(p.Requests) / p.WallSec
		p.RecPerSec = float64(p.Records) / p.WallSec
		p.ServerCPUSec = b.cpuSec() - cpu0
		p.ServerRSSMB, _, p.ServerThreads = b.procStatus()
		p.Connections, p.Retries = b.conns.Load()-conns0, b.retries.Load()-retries0
		if my0 != nil {
			my1 := b.mysqlStatus()
			p.MySQL = map[string]int64{}
			for k, v := range my1 {
				p.MySQL[k] = v - my0[k]
			}
		}
	}
}

func (b *bench) phasePing(concurrency int, d time.Duration) (*phaseResult, error) {
	p := &phaseResult{}
	end := b.begin("ping", concurrency)
	deadline := time.Now().Add(d)
	lat := make([][]time.Duration, concurrency)
	var reqs, errs atomic.Int64
	var wg sync.WaitGroup
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for time.Now().Before(deadline) {
				t := time.Now()
				code, _, err := b.do("GET", "/api/v1/utils/ping", nil, "")
				lat[w] = append(lat[w], time.Since(t))
				reqs.Add(1)
				if err != nil || code != 200 {
					errs.Add(1)
				}
			}
		}(w)
	}
	wg.Wait()
	p.Requests, p.Errors = reqs.Load(), errs.Load()
	p.LatencyMs = latencies(lat)
	end(p)
	return p, nil
}

func (b *bench) phasePut(name string, concurrency, total, batchSize int) (*phaseResult, error) {
	stream, err := b.createStream(name)
	if err != nil {
		return nil, err
	}
	p := &phaseResult{streamUUID: stream}
	end := b.begin(name, concurrency)
	t0 := time.Now()
	uri := "/api/v1/stream/" + stream + "/records"
	if batchSize == 1 {
		uri = "/api/v1/stream/" + stream + "/record"
	}
	nbReq := (total + batchSize - 1) / batchSize
	lat := make([][]time.Duration, concurrency)
	var next, errs atomic.Int64
	var firstErr atomic.Value
	var wg sync.WaitGroup
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			body := make([]byte, 0, 256*batchSize)
			for {
				r := int(next.Add(1) - 1)
				if r >= nbReq {
					return
				}
				body = body[:0]
				first, last := r*batchSize, min((r+1)*batchSize, total)
				if batchSize == 1 {
					body = appendRecord(body, first)
				} else {
					body = append(body, '[')
					for n := first; n < last; n++ {
						if n > first {
							body = append(body, ',')
						}
						body = appendRecord(body, n)
					}
					body = append(body, ']')
				}
				t := time.Now()
				code, respBody, err := b.do("PUT", uri, body, "")
				lat[w] = append(lat[w], time.Since(t))
				if err != nil || code != 202 {
					errs.Add(1)
					firstErr.CompareAndSwap(nil, fmt.Sprintf("status %d err %v body %.200s", code, err, respBody))
				}
			}
		}(w)
	}
	wg.Wait()
	p.WallSec = time.Since(t0).Seconds()
	p.Requests, p.Records, p.Errors = int64(nbReq), int64(total), errs.Load()
	if e := firstErr.Load(); e != nil {
		fmt.Println("  first error:", e)
	}
	p.LatencyMs = latencies(lat)
	// wait until every record is stored
	switch {
	case b.db != nil:
		table := "ministream.stream_" + strings.ReplaceAll(stream, "-", "_")
		for {
			var maxID sql.NullInt64
			if err := b.db.QueryRow("SELECT MAX(id) FROM " + table).Scan(&maxID); err != nil {
				return nil, err
			}
			if maxID.Int64 >= int64(total) {
				break
			}
			if time.Since(t0) > 15*time.Minute {
				return nil, fmt.Errorf("only %d records stored", maxID.Int64)
			}
			time.Sleep(10 * time.Millisecond)
		}
		p.PersistSec = time.Since(t0).Seconds()
	case b.dataDir != "":
		file := filepath.Join(b.dataDir, "streams", stream, "data.jsonl")
		var lines int64
		var offset int64
		buf := make([]byte, 1<<20)
		for lines < int64(total) {
			f, err := os.Open(file)
			if err == nil {
				_, _ = f.Seek(offset, io.SeekStart)
				for {
					n, err := f.Read(buf)
					lines += int64(bytes.Count(buf[:n], []byte{'\n'}))
					offset += int64(n)
					if err != nil || n == 0 {
						break
					}
				}
				_ = f.Close()
			}
			if lines >= int64(total) {
				break
			}
			if time.Since(t0) > 15*time.Minute {
				return nil, fmt.Errorf("only %d records stored", lines)
			}
			time.Sleep(10 * time.Millisecond)
		}
		p.PersistSec = time.Since(t0).Seconds()
	default:
		// in memory: the records become readable at the next flush of the ingest buffer
		time.Sleep(2500 * time.Millisecond)
	}
	end(p)
	return p, nil
}

func (b *bench) phaseRead(name, stream string, readers int, jq string, expected int) (*phaseResult, error) {
	p := &phaseResult{}
	end := b.begin(name, readers)
	lat := make([][]time.Duration, readers)
	var reqs, recs, errs atomic.Int64
	var firstErr atomic.Value
	var wg sync.WaitGroup
	for w := 0; w < readers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			fail := func(msg string) {
				errs.Add(1)
				firstErr.CompareAndSwap(nil, msg)
			}
			itReq := map[string]any{"iteratorType": "FIRST_MESSAGE", "name": fmt.Sprintf("%s-%d", name, w)}
			if jq != "" {
				itReq["jqFilter"] = jq
			}
			reqBody, _ := json.Marshal(itReq)
			b.createIteratorMu.Lock()
			code, body, err := b.do("POST", "/api/v1/stream/"+stream+"/iterator", reqBody, "")
			b.createIteratorMu.Unlock()
			var it struct {
				UUID string `json:"streamIteratorUUID"`
			}
			if err != nil || code != 200 || json.Unmarshal(body, &it) != nil || it.UUID == "" {
				fail(fmt.Sprintf("create iterator: status %d err %v body %.200s", code, err, body))
				return
			}
			uri := "/api/v1/stream/" + stream + "/iterator/" + it.UUID + "/records?maxRecords=1000"
			read := 0
			for {
				t := time.Now()
				code, body, err := b.do("GET", uri, nil, "")
				lat[w] = append(lat[w], time.Since(t))
				reqs.Add(1)
				var r struct {
					Count       int  `json:"count"`
					CountErrors int  `json:"countErrors"`
					Remain      bool `json:"remain"`
				}
				if err != nil || code != 200 || json.Unmarshal(body, &r) != nil {
					fail(fmt.Sprintf("get records: status %d err %v body %.200s", code, err, body))
					return
				}
				read += r.Count
				recs.Add(int64(r.Count))
				if r.CountErrors > 0 {
					fail(fmt.Sprintf("get records: %d jq errors", r.CountErrors))
				}
				if !r.Remain || r.Count == 0 {
					break
				}
			}
			if read != expected {
				fail(fmt.Sprintf("reader %d read %d records instead of %d", w, read, expected))
			}
			_, _, _ = b.do("DELETE", "/api/v1/stream/"+stream+"/iterator/"+it.UUID, nil, "")
		}(w)
	}
	wg.Wait()
	if e := firstErr.Load(); e != nil {
		fmt.Println("  first error:", e)
	}
	p.Requests, p.Records, p.Errors = reqs.Load(), recs.Load(), errs.Load()
	p.LatencyMs = latencies(lat)
	end(p)
	return p, nil
}

// cpuSec returns the user + system CPU time of the server process.
func (b *bench) cpuSec() float64 {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", b.pid))
	if err != nil {
		return 0
	}
	s := string(data)
	fields := strings.Fields(s[strings.LastIndexByte(s, ')')+2:])
	utime, _ := strconv.ParseFloat(fields[11], 64)
	stime, _ := strconv.ParseFloat(fields[12], 64)
	return (utime + stime) / clockTicksPerSecond
}

// procStatus returns the resident memory, the peak resident memory (MB) and the threads count.
func (b *bench) procStatus() (rss, hwm float64, threads int) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", b.pid))
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		v, _ := strconv.ParseFloat(f[1], 64)
		switch f[0] {
		case "VmRSS:":
			rss = v / 1024
		case "VmHWM:":
			hwm = v / 1024
		case "Threads:":
			threads = int(v)
		}
	}
	return
}

var mysqlCounters = []string{"Questions", "Com_insert", "Com_select", "Com_commit", "Com_stmt_prepare", "Com_stmt_execute", "Innodb_rows_inserted", "Innodb_rows_read", "Bytes_received", "Bytes_sent"}

func (b *bench) mysqlStatus() map[string]int64 {
	if b.db == nil {
		return nil
	}
	rows, err := b.db.Query("SHOW GLOBAL STATUS")
	if err != nil {
		return nil
	}
	defer func() { _ = rows.Close() }()
	m := map[string]int64{}
	for rows.Next() {
		var k, v string
		if rows.Scan(&k, &v) == nil {
			for _, c := range mysqlCounters {
				if k == c {
					m[k], _ = strconv.ParseInt(v, 10, 64)
				}
			}
		}
	}
	return m
}

func dirSize(dir string) int64 {
	var size int64
	_ = filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}
