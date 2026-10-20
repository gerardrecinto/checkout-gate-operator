#!/usr/bin/env bash
# Fails when a package's statement coverage drops below its floor. Floors sit
# just under current coverage, so a change that lands untested or deletes tests
# fails the PR. Raise a floor when coverage rises; do not lower one to merge.
set -euo pipefail

floors=(
  "./internal/apiserver:55"
  "./internal/controller:77"
  "./internal/gate:100"
  "./internal/notify:93"
  "./internal/webhook:86"
)

fail=0
for entry in "${floors[@]}"; do
  pkg="${entry%%:*}"
  floor="${entry##*:}"
  out=$(go test -count=1 -cover "$pkg")
  pct=$(printf '%s\n' "$out" | sed -n 's/.*coverage: \([0-9.]*\)%.*/\1/p' | tail -1)
  if [ -z "$pct" ]; then
    echo "FAIL $pkg: no coverage reported"
    printf '%s\n' "$out"
    fail=1
  elif awk -v p="$pct" -v f="$floor" 'BEGIN { exit !(p + 0 < f + 0) }'; then
    echo "FAIL $pkg: ${pct}% < floor ${floor}%"
    fail=1
  else
    echo "ok   $pkg: ${pct}% >= floor ${floor}%"
  fi
done
exit "$fail"
