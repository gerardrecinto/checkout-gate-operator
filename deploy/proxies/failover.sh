#!/usr/bin/env bash
# Usage: failover.sh [stop|fail]. stop kills the backend1 container, fail makes it
# answer 503 (POST /fail). Takes backend1 down behind each proxy, fires a request every 50ms for 10s, and
# reports failed requests plus how long until backend1 serves again after restart.
set -u
MODE=${1:-stop}
net=$(docker compose ps -q backend2 | xargs docker inspect -f '{{range $k,$v := .NetworkSettings.Networks}}{{$k}}{{end}}')
backend() { docker run --rm --network "$net" curlimages/curl:8.10.1 -s -o /dev/null -X POST "http://backend1:8080/$1"; }
down() { if [ "$MODE" = fail ]; then backend fail; else docker compose stop -t 0 backend1 >/dev/null; fi; }
up() { if [ "$MODE" = fail ]; then backend recover; else docker compose start backend1 >/dev/null; fi; }
cd "$(dirname "$0")"
port() { case $1 in nginx) echo 8441;; haproxy) echo 8442;; apache) echo 8443;; esac; }

now_ms() { python3 -c 'import time; print(int(time.time()*1000))'; }

for p in nginx haproxy apache; do
  url="https://localhost:$(port $p)/"
  up; sleep 6
  seen=$(for _ in $(seq 20); do curl -ks --max-time 2 "$url"; done | sort | uniq -c | tr '\n' ' ')
  echo "== $p baseline: $seen"
  echo "   tls: $(curl -ksv --max-time 2 "$url" 2>&1 | grep -o 'SSL connection using [A-Za-z0-9._ -]*' | head -1)"

  down
  t0=$(now_ms); fail=0; total=0; last_fail=0
  while [ $(( $(now_ms) - t0 )) -lt 10000 ]; do
    code=$(curl -ks -o /dev/null -w '%{http_code}' --max-time 2 "$url")
    total=$((total+1))
    if [ "$code" != 200 ]; then fail=$((fail+1)); last_fail=$(( $(now_ms) - t0 )); fi
    sleep 0.05
  done
  echo "   backend1 down ($MODE): $fail failed of $total requests, last failure at ${last_fail}ms"

  up; t1=$(now_ms); back=-1
  while [ $(( $(now_ms) - t1 )) -lt 15000 ]; do
    if [ "$(curl -ks --max-time 2 "$url" | tr -d '\n')" = backend1 ]; then back=$(( $(now_ms) - t1 )); break; fi
    sleep 0.1
  done
  echo "   backend1 serving again after ${back}ms"
done
