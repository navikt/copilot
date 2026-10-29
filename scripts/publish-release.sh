#!/usr/bin/env bash
# Publish a GitHub release so that it is either complete or absent (#2056).
#
# The release is created as a draft, every file in ASSET_DIR is uploaded to it
# with bounded retries, the uploaded names and SHA-256 digests are checked
# against the local files, and only then is the draft published. Publishing
# creates the tag, so a draft never has one. If anything fails, the draft is
# deleted and the script exits non-zero.
#
# A run that is killed (cancel, timeout) cannot clean up after itself, and the
# next run has a new timestamp tag. So every run first deletes all nav-pilot/
# drafts. That is safe because the release workflow's concurrency group
# (nav-pilot-release, no cancel-in-progress) lets only one run publish at once.
# A tag that is already published with every asset is a no-op.
#
# Usage: publish-release.sh TAG TARGET_SHA TITLE NOTES_FILE ASSET_DIR
# Env:   UPLOAD_ATTEMPTS (default 3), RETRY_DELAY seconds (default 10)
set -euo pipefail

tag=$1 target=$2 title=$3 notes=$4 dir=$5
attempts=${UPLOAD_ATTEMPTS:-3}
delay=${RETRY_DELAY:-10}

sha256() {
  if command -v sha256sum >/dev/null; then sha256sum "$1"; else shasum -a 256 "$1"; fi | cut -d' ' -f1
}

# "name sha256:digest uploaded" per local file, sorted: what the release must hold.
expected=$(for f in "$dir"/*; do [ -f "$f" ] && printf '%s sha256:%s uploaded\n' "$(basename "$f")" "$(sha256 "$f")"; done | sort)
[ -n "$expected" ] || { echo "::error::no assets in $dir"; exit 1; }

remote_assets() {
  gh release view "$tag" --json assets --jq '.assets[] | "\(.name) \(.digest) \(.state)"' | sort
}

# Prints "draft", "published" or "absent". Any other gh error fails.
state() {
  local out err
  if out=$(gh release view "$tag" --json isDraft --jq 'if .isDraft then "draft" else "published" end' 2>"$errf"); then
    echo "$out"
  elif grep -qx 'release not found' "$errf"; then
    echo absent
  else
    err=$(cat "$errf")
    echo "::error::could not look up release $tag: $err" >&2
    return 1
  fi
}
errf=$(mktemp)

for draft in $(gh release list --limit 100 --json tagName,isDraft --jq '.[] | select(.isDraft) | .tagName'); do
  case "$draft" in
    nav-pilot/*)
      echo "Deleting leftover draft $draft from an earlier run"
      gh release delete "$draft" --yes
      ;;
  esac
done

s=$(state)
if [ "$s" = published ]; then
  if [ "$(remote_assets)" = "$expected" ]; then
    echo "$tag is already published with every asset; nothing to do"
    exit 0
  fi
  echo "::error::$tag is published but its assets differ from this build; fix it by hand"
  exit 1
fi

cleanup() {
  local s
  if ! s=$(state); then
    echo "::error::publishing $tag failed and its state is unknown; a draft may remain, delete it by hand"
    return
  fi
  if [ "$s" = draft ]; then
    echo "::error::publishing $tag failed; deleting the draft so no partial release remains"
    gh release delete "$tag" --yes || echo "::error::could not delete draft $tag; delete it by hand"
  fi
}
trap cleanup EXIT
trap 'exit 143' INT TERM

gh release create "$tag" --draft --target "$target" --title "$title" --notes-file "$notes"

for f in "$dir"/*; do
  [ -f "$f" ] || continue
  for ((i = 1; ; i++)); do
    if gh release upload "$tag" "$f" --clobber; then break; fi
    if ((i >= attempts)); then
      echo "::error::upload of $(basename "$f") failed $attempts times"
      exit 1
    fi
    echo "::warning::upload of $(basename "$f") failed (attempt $i/$attempts); retrying in ${delay}s"
    sleep "$delay" & wait $! # background + wait, so a SIGTERM is handled at once
  done
done

actual=$(remote_assets)
if [ "$actual" != "$expected" ]; then
  echo "::error::draft $tag does not hold exactly the built assets"
  diff <(echo "$expected") <(echo "$actual") || true
  exit 1
fi

gh release edit "$tag" --draft=false --latest
trap - EXIT
echo "Published $tag"
