#!/bin/sh
# Benchmarks one server build with one storage, inside the runner container (see run.sh).
# usage: run-one.sh <name> <storage> <run>    (SMOKE=1 for a quick check with small sizes)
set -e
impl=$1 storage=$2 run=$3
mkdir -p /opt/bench /results/logs
cp /bench/bin/loader "/bench/bin/ministream-$impl" /opt/bench/ && chmod +x /opt/bench/*
if [ "$SMOKE" = 1 ]; then
  sizes="-ping-duration 2s -single 2000 -batch 10000"; mysql_sizes="$sizes"; prefix="smoke-"
else
  sizes="-single 100000 -batch 500000"; mysql_sizes="-single 20000 -batch 100000"; prefix=""
fi
case $storage in
  inmemory) extra="$sizes" ;;
  jsonfile) mkdir -p /data/storage && cp /bench/config/streams.json /data/storage/streams.json
            extra="$sizes -data-dir /data/storage" ;;
  mysql)    extra="$mysql_sizes -mysql-dsn root:benchpw@tcp(mbench-mysql:3306)/?tls=skip-verify" ;;
esac
# the server runs on the CPUs 0,1 and the load generator on the CPUs 2,3
exec taskset -c 2,3 /opt/bench/loader run -impl "$impl" -storage "$storage" -run "$run" \
  -server "/opt/bench/ministream-$impl" -config "/bench/config/$storage.yaml" -cpus 0,1 \
  -out "/results/$prefix$impl-$storage-$run.json" -log "/results/logs/$prefix$impl-$storage-$run.log" $extra
