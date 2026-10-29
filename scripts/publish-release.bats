#!/usr/bin/env bats
#
# publish-release.sh must leave a release either complete or absent (#2056).
# A real release cannot be published from a test, so gh is replaced by a fake
# that keeps one release in a state directory and can fail or corrupt uploads.

SCRIPT="${BATS_TEST_DIRNAME}/publish-release.sh"

setup() {
  export STATE="$BATS_TEST_TMPDIR/state"
  mkdir -p "$STATE" "$BATS_TEST_TMPDIR/bin" "$BATS_TEST_TMPDIR/assets"
  printf 'one' >"$BATS_TEST_TMPDIR/assets/nav-pilot-linux-amd64"
  printf 'twotwo' >"$BATS_TEST_TMPDIR/assets/nav-pilot-darwin-arm64"
  printf 'sums' >"$BATS_TEST_TMPDIR/assets/SHA256SUMS"
  echo notes >"$BATS_TEST_TMPDIR/notes.md"
  export RETRY_DELAY=0 FAIL_UPLOADS=0 CORRUPT=""

  cat >"$BATS_TEST_TMPDIR/bin/gh" <<'GH'
#!/usr/bin/env bash
echo "$*" >>"$STATE/log"
[ "$1" = release ] || exit 2
case "$2" in
  view)
    [ -f "$STATE/kind" ] || exit 1
    case "$*" in
      *isDraft*) cat "$STATE/kind" ;;
      *assets*) cat "$STATE/assets" 2>/dev/null ;;
    esac ;;
  create) echo draft >"$STATE/kind"; : >"$STATE/assets" ;;
  upload)
    n=$(cat "$STATE/fails" 2>/dev/null || echo 0)
    if [ "$n" -lt "$FAIL_UPLOADS" ]; then echo $((n + 1)) >"$STATE/fails"; echo "other side closed" >&2; exit 1; fi
    name=$(basename "$4") size=$(wc -c <"$4" | tr -d ' ')
    [ "$name" = "$CORRUPT" ] && size=0
    grep -v "^$name " "$STATE/assets" >"$STATE/a.tmp" || true
    echo "$name $size" >>"$STATE/a.tmp" && mv "$STATE/a.tmp" "$STATE/assets" ;;
  edit) echo published >"$STATE/kind" ;;
  delete) rm -f "$STATE/kind" "$STATE/assets" ;;
esac
GH
  chmod +x "$BATS_TEST_TMPDIR/bin/gh"
  export PATH="$BATS_TEST_TMPDIR/bin:$PATH"
}

publish() {
  run "$SCRIPT" nav-pilot/v1 abc123 "nav-pilot v1" "$BATS_TEST_TMPDIR/notes.md" "$BATS_TEST_TMPDIR/assets"
}

@test "publishes only after every asset is uploaded" {
  publish
  [ "$status" -eq 0 ]
  [ "$(cat "$STATE/kind")" = published ]
  [ "$(wc -l <"$STATE/assets" | tr -d ' ')" -eq 3 ]
  # The edit that publishes comes after the last upload.
  [ "$(tail -n1 "$STATE/log")" = "release edit nav-pilot/v1 --draft=false --latest" ]
}

@test "a transient upload failure is retried and the release still publishes" {
  FAIL_UPLOADS=2 publish
  [ "$status" -eq 0 ]
  [ "$(cat "$STATE/kind")" = published ]
  [[ "$output" == *"attempt 2/3"* ]]
}

@test "an upload that keeps failing deletes the draft and fails the job" {
  FAIL_UPLOADS=99 publish
  [ "$status" -ne 0 ]
  [ ! -f "$STATE/kind" ]
  ! grep -q "release edit" "$STATE/log"
  [[ "$output" == *"failed 3 times"* ]]
}

# Control for the verification step: the uploads "succeed", but one asset
# lands with the wrong size. Without the name/size check this would publish.
@test "control: a corrupt asset is caught, nothing is published" {
  CORRUPT=nav-pilot-darwin-arm64 publish
  [ "$status" -ne 0 ]
  [ ! -f "$STATE/kind" ]
  ! grep -q "release edit" "$STATE/log"
}

@test "a rerun deletes a leftover draft and publishes cleanly" {
  echo draft >"$STATE/kind"
  echo "nav-pilot-linux-amd64 1" >"$STATE/assets"
  publish
  [ "$status" -eq 0 ]
  [ "$(head -n2 "$STATE/log" | tail -n1)" = "release delete nav-pilot/v1 --yes" ]
  [ "$(cat "$STATE/kind")" = published ]
  [ "$(wc -l <"$STATE/assets" | tr -d ' ')" -eq 3 ]
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
  echo published >"$STATE/kind"
  echo "nav-pilot-linux-amd64 1" >"$STATE/assets"
  publish
  [ "$status" -ne 0 ]
  ! grep -qE "release (create|upload|edit|delete)" "$STATE/log"
}
