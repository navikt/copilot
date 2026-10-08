#!/bin/bash
# Paired A/B of the nav-pilot suite: each round runs `go test -race` in two
# checkouts at the same time, so both see the same load, with SPIN extra
# `yes` processes underneath. Prints one line per run and the failed tests.
#
#   git worktree add --detach /tmp/np-main origin/main
#   bash hack/probes/ab-suite.sh /tmp/np-main . 8 12
#
# usage: ab-suite.sh A_CHECKOUT B_CHECKOUT ROUNDS SPIN
set -u
A=$(cd "$1/cli/nav-pilot" && pwd) B=$(cd "$2/cli/nav-pilot" && pwd)
logs=$(mktemp -d)
spin=()
trap 'kill "${spin[@]}" 2>/dev/null' EXIT
for _ in $(seq 1 "$4"); do yes > /dev/null & spin+=($!); done
run() { # dir label round
  (cd "$1" && go test -race -skip TestLaunchBudget -count=1 ./... > "$logs/$2-$3.log" 2>&1)
  echo "$2 round $3 rc=$? fails: $(grep -oE -- '--- FAIL: [A-Za-z_0-9/]+' "$logs/$2-$3.log" | sed 's/--- FAIL: //' | tr '\n' ' ')"
}
for i in $(seq 1 "$3"); do
  echo "round $i load=$(uptime | sed 's/.*load averages*: //')"
  run "$A" A "$i" & a=$!
  run "$B" B "$i"
  wait $a
done
echo "logs: $logs"
