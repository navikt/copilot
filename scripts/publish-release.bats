#!/usr/bin/env bats
#
# publish-release.sh must leave a release either complete or absent (#2056).
# A real release cannot be published from a test, so gh is replaced by a fake
# that keeps releases per tag in a state directory and can fail lookups, fail
# or corrupt uploads, and fail after publishing.

SCRIPT="${BATS_TEST_DIRNAME}/publish-release.sh"
TAG=nav-pilot/v2

setup() {
  export STATE="$BATS_TEST_TMPDIR/state"
  mkdir -p "$STATE/r" "$BATS_TEST_TMPDIR/bin" "$BATS_TEST_TMPDIR/assets"
  printf 'one' >"$BATS_TEST_TMPDIR/assets/nav-pilot-linux-amd64"
  printf 'two' >"$BATS_TEST_TMPDIR/assets/nav-pilot-darwin-arm64"
  printf 'sums' >"$BATS_TEST_TMPDIR/assets/SHA256SUMS"
  echo notes >"$BATS_TEST_TMPDIR/notes.md"
  export RETRY_DELAY=0 FAIL_UPLOADS=0 CORRUPT="" FAIL_VIEW="" FAIL_EDIT=""

  cat >"$BATS_TEST_TMPDIR/bin/gh" <<'GH'
#!/usr/bin/env bash
echo "$*" >>"$STATE/log"
[ "$1" = release ] || exit 2
d="$STATE/r/$(echo "$3" | tr / _)"
digest() { if command -v sha256sum >/dev/null; then sha256sum "$1"; else shasum -a 256 "$1"; fi | cut -d' ' -f1; }
case "$2" in
  list)
    for r in "$STATE"/r/*; do
      [ "$(cat "$r/kind" 2>/dev/null)" = draft ] && basename "$r" | tr _ /
    done
    exit 0 ;;
  view)
    [ -n "$FAIL_VIEW" ] && { echo "HTTP 502: Bad Gateway" >&2; exit 1; }
    [ -f "$d/kind" ] || { echo "release not found" >&2; exit 1; }
    case "$*" in
      *isDraft*) cat "$d/kind" ;;
      *assets*) cat "$d/assets" ;;
    esac ;;
  create) mkdir -p "$d"; echo draft >"$d/kind"; : >"$d/assets" ;;
  upload)
    n=$(cat "$STATE/fails" 2>/dev/null || echo 0)
    if [ "$n" -lt "$FAIL_UPLOADS" ]; then echo $((n + 1)) >"$STATE/fails"; echo "other side closed" >&2; exit 1; fi
    name=$(basename "$4") sum=$(digest "$4")
    [ "$name" = "$CORRUPT" ] && sum=0000
    { grep -v "^$name " "$d/assets" || true; echo "$name sha256:$sum uploaded"; } >"$d/a.tmp"
    mv "$d/a.tmp" "$d/assets" ;;
  edit) echo published >"$d/kind"; [ -z "$FAIL_EDIT" ] || exit 1 ;;
  delete) rm -rf "$d" ;;
esac
GH
  chmod +x "$BATS_TEST_TMPDIR/bin/gh"
  export PATH="$BATS_TEST_TMPDIR/bin:$PATH"
}

release() { # $1 tag, $2 kind, then asset lines
  local d="$STATE/r/$(echo "$1" | tr / _)"
  mkdir -p "$d"; echo "$2" >"$d/kind"; shift 2
  printf '%s\n' "$@" >"$d/assets"
}
kind() { cat "$STATE/r/$(echo "$1" | tr / _)/kind" 2>/dev/null || echo absent; }
assets() { wc -l <"$STATE/r/$(echo "$TAG" | tr / _)/assets" | tr -d ' '; }

publish() {
  run "$SCRIPT" "$TAG" abc123 "nav-pilot v2" "$BATS_TEST_TMPDIR/notes.md" "$BATS_TEST_TMPDIR/assets"
}

@test "publishes only after every asset is uploaded and verified" {
  publish
  [ "$status" -eq 0 ]
  [ "$(kind $TAG)" = published ]
  [ "$(assets)" -eq 3 ]
  [ "$(tail -n1 "$STATE/log")" = "release edit $TAG --draft=false --latest" ]
}

@test "a transient upload failure is retried and the release still publishes" {
  FAIL_UPLOADS=2 publish
  [ "$status" -eq 0 ]
  [ "$(kind $TAG)" = published ]
  [[ "$output" == *"attempt 2/3"* ]]
}

@test "an upload that keeps failing deletes the draft and fails the job" {
  FAIL_UPLOADS=99 publish
  [ "$status" -ne 0 ]
  [ "$(kind $TAG)" = absent ]
  ! grep -q "release edit" "$STATE/log"
  [[ "$output" == *"failed 3 times"* ]]
}

# Control for the verification step: the uploads "succeed", but one asset
# lands with the wrong digest. Without the digest check this would publish.
@test "control: a corrupt asset is caught, nothing is published" {
  CORRUPT=nav-pilot-darwin-arm64 publish
  [ "$status" -ne 0 ]
  [ "$(kind $TAG)" = absent ]
  ! grep -q "release edit" "$STATE/log"
}

@test "orphan nav-pilot drafts from killed runs are swept, other drafts and releases are kept" {
  release nav-pilot/v1 draft "x sha256:1 uploaded"
  release nav-pilot/v0 published "x sha256:1 uploaded"
  release other/v1 draft
  publish
  [ "$status" -eq 0 ]
  [ "$(kind nav-pilot/v1)" = absent ]
  [ "$(kind nav-pilot/v0)" = published ]
  [ "$(kind other/v1)" = draft ]
  [ "$(kind $TAG)" = published ]
}

@test "a rerun deletes its own leftover draft and publishes cleanly" {
  release $TAG draft "nav-pilot-linux-amd64 sha256:1 uploaded"
  publish
  [ "$status" -eq 0 ]
  grep -qx "release delete $TAG --yes" "$STATE/log"
  [ "$(kind $TAG)" = published ]
  [ "$(assets)" -eq 3 ]
}

@test "a rerun after a complete publish does nothing" {
  publish
  : >"$STATE/log"
  publish
  [ "$status" -eq 0 ]
  [[ "$output" == *"nothing to do"* ]]
  ! grep -qE "release (create|upload|edit|delete)" "$STATE/log"
}

@test "a published release with different assets is not touched" {
  release $TAG published "nav-pilot-linux-amd64 sha256:1 uploaded"
  publish
  [ "$status" -ne 0 ]
  ! grep -qE "release (create|upload|edit|delete)" "$STATE/log"
}

@test "a lookup error that is not 'release not found' aborts before anything is created" {
  FAIL_VIEW=1 publish
  [ "$status" -ne 0 ]
  [[ "$output" == *"could not look up release"* ]]
  ! grep -qE "release (create|upload|edit)" "$STATE/log"
}

@test "an edit that publishes and then errors fails the job but does not delete" {
  FAIL_EDIT=1 publish
  [ "$status" -ne 0 ]
  [ "$(kind $TAG)" = published ]
  ! grep -q "release delete" "$STATE/log"
}

@test "SIGTERM runs the cleanup and deletes the draft" {
  cat >"$BATS_TEST_TMPDIR/bin/sleep" <<'SL'
#!/usr/bin/env bash
touch "$STATE/sleeping"; exec /bin/sleep "$1"
SL
  chmod +x "$BATS_TEST_TMPDIR/bin/sleep"
  export FAIL_UPLOADS=99 RETRY_DELAY=5
  "$SCRIPT" "$TAG" abc123 t "$BATS_TEST_TMPDIR/notes.md" "$BATS_TEST_TMPDIR/assets" >/dev/null 2>&1 &
  pid=$!
  for _ in $(seq 50); do [ -f "$STATE/sleeping" ] && break; /bin/sleep 0.1; done
  [ "$(kind $TAG)" = draft ]
  kill -TERM "$pid"
  st=0; wait "$pid" || st=$?
  [ "$st" -eq 143 ]
  [ "$(kind $TAG)" = absent ]
}
