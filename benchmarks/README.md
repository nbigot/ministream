# Benchmarks

Measures the performance of one or several ministream server builds with each storage provider (in memory, JSON
files, MySQL): throughput, latency, CPU time and memory of the server. Use it to compare two versions (for example
before and after a change), or any other ministream compatible server.

Everything runs in Linux containers with Docker, on a single machine:

- the server runs on 2 CPUs (CPUs 0 and 1),
- the load generator runs on 2 other CPUs (CPUs 2 and 3), MySQL shares them,
- each run starts a new server, with new data (new JSON files directory, new MySQL database).

The machine (or the Docker Desktop VM) needs 4 CPUs and about 3 GB of memory.


## Usage

```sh
cd benchmarks
./build.sh            # builds bin/loader and bin/ministream-current (this repository, for Linux)
SMOKE=1 RUNS=1 ./run.sh   # quick check (small sizes), about 1 minute per build and storage
./run.sh              # 3 runs of each build with each storage
go run . report -impls current results/*.json
```

To compare builds, put their Linux binary in `bin/ministream-<name>`. For example to compare with the version `v1.2.3`:

```sh
git worktree add ../../ministream-v1.2.3 v1.2.3
(cd ../../ministream-v1.2.3 && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o "$OLDPWD/bin/ministream-v1.2.3" ./cmd/ministream)
./run.sh
go run . report -impls v1.2.3,current results/*.json
```

`run.sh` benchmarks every `bin/ministream-*` binary, it takes these optional environment variables:

| variable | default | |
|---|---|---|
| `IMPLS` | every `bin/ministream-<name>` | names of the builds to benchmark, separated by spaces |
| `STORAGES` | `inmemory jsonfile mysql` | |
| `RUNS` | `3` | the order of the builds changes at each run |
| `RESULTS` | `results` | directory of the results (JSON) and of the server logs |
| `SMOKE` | | `1`: small sizes, to check that everything works |

The MySQL container (`mbench-mysql`) is kept between invocations, remove it with
`docker rm -f mbench-mysql && docker network rm mbench`.


## Scenario

The configuration files of `config/` are the `minimal-*` templates of `config-templates`, only the listen address,
the data directory and the MySQL host differ. The records are JSON objects of about 150 bytes.

| phase | what | in memory, JSON files | MySQL |
|---|---|---|---|
| `ping` | `GET /api/v1/utils/ping`, 64 concurrent requests | 10 s | 10 s |
| `put-record` | `PUT .../record`, one record per request, 64 concurrent requests | 100,000 records | 20,000 records |
| `put-records` | `PUT .../records`, 100 records per request, 16 concurrent requests | 500,000 records | 100,000 records |
| `read` | 4 iterators read every record of the `put-records` stream, 1,000 records per request | 4 × 500,000 | 4 × 100,000 |
| `read-jq` | same with the jq filter `select(.m.n % 10 == 0) \| {i, user: .m.user, value: .m.value}` (returns 1 record out of 10) | 4 × 50,000 | 4 × 10,000 |

The reads check that every record is returned exactly once.


## Results

The report prints the median of the runs, and the spread between runs ((max - min) / median) when it is above 5%.

- **records/s**: records accepted by the API per second. The API accepts the records before writing them, so for the
  JSON files and MySQL the report also shows **records/s stored**: until every record is written (data file lines,
  `MAX(id)` of the MySQL table).
- **latency p50, p99**: of the HTTP requests, in milliseconds.
- **server CPU ms per 1k records**: CPU time (user + system) of the server process. It shows the efficiency of a build
  even when the load generator is the bottleneck (`ping`).
- **MySQL statements per 1k records**: `Questions` counter of MySQL.
- **TCP connections opened**: by the load generator, which uses keep-alive connections (64 when the server keeps
  them open). The load generator sends a request again when the server closed the connection before answering.
- **memory**: resident memory of the server (idle after startup, peak, at the end).

Keep in mind:

- The `ping` throughput is usually limited by the load generator (2 CPUs), compare the server CPU time instead.
- The MySQL write throughput mostly depends on `streams.bulkMaxSize` (100 in the templates: one transaction every
  100 records) and on the disk.
- Tail latencies (p99) vary a lot between runs, more runs give more stable results.
- The load generator creates the iterators one at a time: older versions crash when iterators are created
  concurrently.

`go run . iterators -workers 8 -rounds 100` creates iterators concurrently on a running server (to check that it
does not crash).
