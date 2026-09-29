#!/bin/bash
# Stress the timing tests from #1335 under load.
#
#   hack/probes/load-stress.sh TREE OUT [PROCS]
#
# Builds the nav-pilot test binaries from the checkout TREE, runs
# `mise run check` in TREE in a loop as background load, and then runs PROCS
# copies (default 9) of each test binary at once, each with -count=20:
# TestResolveForLaunchWithinMaxAge, the fetch_typeahead_stderr_redirected
# script, and the three cli tests that probe a fake cplt. Logs and a summary
# with load averages go to OUT.
set -u
tree=$(cd "$1" && pwd); out=$2; procs=${3:-9}
mkdir -p "$out"; out=$(cd "$out" && pwd)
nav=$tree/cli/nav-pilot
(cd "$nav" && go test -c -o "$out/source.test" ./internal/source && go test -c -o "$out/e2e.test" ./e2e && go test -c -o "$out/cli.test" ./internal/cli) || exit 1

killtree() { for c in $(pgrep -P "$1"); do killtree "$c"; done; kill "$1" 2>/dev/null; }

echo "tree: $(git -C "$tree" rev-parse --short HEAD)" > "$out/summary.txt"
echo "before: $(uptime)" >> "$out/summary.txt"
(cd "$tree" && while :; do mise run check >/dev/null 2>&1; done) & load=$!
trap 'kill ${sampler:-} 2>/dev/null; killtree "$load"' EXIT
sleep 45
echo "load running: $(uptime)" >> "$out/summary.txt"
(while :; do uptime >> "$out/uptime.log"; sleep 30; done) & sampler=$!

cplt='TestCpltBuiltinDomainsComeFromCplt|TestDoctorReportsCpltVersion|TestUpgradeChecksCpltWhenNavPilotIsCurrent'
pids=()
for i in $(seq "$procs"); do
	(cd "$nav/internal/source" && "$out/source.test" -test.run '^TestResolveForLaunchWithinMaxAge$' -test.count=20 -test.v > "$out/source.$i.log" 2>&1) & pids+=($!)
	(cd "$nav/e2e" && "$out/e2e.test" -test.run 'TestScripts/^fetch_typeahead_stderr_redirected$' -test.count=20 -test.v -test.timeout=90m > "$out/e2e.$i.log" 2>&1) & pids+=($!)
	(cd "$nav/internal/cli" && "$out/cli.test" -test.run "^($cplt)\$" -test.count=20 -test.v > "$out/cli.$i.log" 2>&1) & pids+=($!)
done
wait "${pids[@]}"
echo "after: $(uptime)" >> "$out/summary.txt"
kill "$sampler"; killtree "$load"

echo "peak 1m load: $(awk -F'load averages: ' '{split($2,a," "); print a[1]}' "$out/uptime.log" | sort -n | tail -1)" >> "$out/summary.txt"
count() { cat "$out"/"$1".*.log | grep -c -- "--- $2: $3"; }
echo "TestResolveForLaunchWithinMaxAge: pass $(count source PASS TestResolveForLaunchWithinMaxAge), fail $(count source FAIL TestResolveForLaunchWithinMaxAge)" >> "$out/summary.txt"
echo "fetch_typeahead_stderr_redirected: pass $(count e2e PASS TestScripts/fetch), fail $(count e2e FAIL TestScripts/fetch)" >> "$out/summary.txt"
for t in ${cplt//|/ }; do echo "$t: pass $(count cli PASS "$t "), fail $(count cli FAIL "$t ")" >> "$out/summary.txt"; done
cat "$out/summary.txt"
