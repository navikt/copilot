#!/usr/bin/env bash
# Sourced by check.sh and build.sh. `section NAME CMD...` starts CMD in the
# background with its output in a log; `wait_sections` waits for them in the
# order they were started, prints each log whole (no interleaving) and adds
# the names of failed sections to $failed.
failed=()
_logs=$(mktemp -d)
trap 'rm -rf "$_logs"' EXIT
_names=()
_pids=()

section() {
  local name=$1
  shift
  (
    start=$SECONDS
    "$@"
    rc=$?
    echo "⏱  $name: $((SECONDS - start))s"
    exit $rc
  ) >"$_logs/${#_names[@]}.log" 2>&1 </dev/null &
  _names+=("$name")
  _pids+=($!)
}

wait_sections() {
  local i
  for i in "${!_names[@]}"; do
    wait "${_pids[$i]}" || failed+=("${_names[$i]}")
    echo "── ${_names[$i]}"
    cat "$_logs/$i.log"
    echo ""
  done
  _names=()
  _pids=()
}
