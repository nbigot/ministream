#!/bin/bash
# Benchmarks every server build of bin/ (bin/ministream-<name>) with every storage, RUNS times,
# rotating the order of the builds. See README.md.
# usage: [IMPLS="current other"] [STORAGES="inmemory jsonfile mysql"] [RUNS=3] [RESULTS=results] [SMOKE=1] ./run.sh
set -u
export MSYS_NO_PATHCONV=1 # Git Bash on Windows: do not convert the paths given to docker
cd "$(dirname "$0")"
B=$(pwd -W 2>/dev/null || pwd) # Windows path with Git Bash
RUNS=${RUNS:-3}
STORAGES=${STORAGES:-"inmemory jsonfile mysql"}
RESULTS=${RESULTS:-results}
RUNNER_IMAGE=${RUNNER_IMAGE:-debian:13-slim}
MYSQL_IMAGE=${MYSQL_IMAGE:-mysql:8.4}
if [ -n "${IMPLS:-}" ]; then
  read -ra impls <<< "$IMPLS"
else
  read -ra impls <<< "$(cd bin && ls ministream-* | sed 's/^ministream-//' | tr '\n' ' ')"
fi
[ ${#impls[@]} -gt 0 ] || { echo "no server build in bin/ (run ./build.sh)"; exit 1; }
mkdir -p "$RESULTS"

docker network inspect mbench >/dev/null 2>&1 || docker network create mbench >/dev/null
if [[ " $STORAGES " == *" mysql "* ]] && ! docker inspect mbench-mysql >/dev/null 2>&1; then
  # MySQL shares the CPUs of the load generator
  docker run -d --name mbench-mysql --network mbench --cpuset-cpus 2,3 \
    -e MYSQL_ROOT_PASSWORD=benchpw -e MYSQL_DATABASE=ministream -e MYSQL_USER=ministream -e MYSQL_PASSWORD=ministream \
    "$MYSQL_IMAGE" >/dev/null
  until docker exec mbench-mysql mysql -uroot -pbenchpw -e "SELECT 1" >/dev/null 2>&1; do sleep 1; done
fi

for run in $(seq 1 "$RUNS"); do
  for storage in $STORAGES; do
    for i in $(seq 0 $((${#impls[@]} - 1))); do
      impl=${impls[$(((i + run - 1) % ${#impls[@]}))]}
      if [ "$storage" = mysql ]; then
        docker exec mbench-mysql mysql -uroot -pbenchpw \
          -e "DROP DATABASE IF EXISTS ministream; CREATE DATABASE ministream; RESET BINARY LOGS AND GTIDS;" 2>&1 |
          grep -v "Using a password"
        sleep 2
      fi
      docker run --rm --name mbench-run --network mbench -e SMOKE="${SMOKE:-}" \
        -v "$B:/bench:ro" -v "$B/$RESULTS:/results" "$RUNNER_IMAGE" sh /bench/run-one.sh "$impl" "$storage" "$run" ||
        echo "FAILED: $impl $storage $run"
    done
  done
done
echo "remove the MySQL container with: docker rm -f mbench-mysql && docker network rm mbench"
