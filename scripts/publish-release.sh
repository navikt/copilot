#!/usr/bin/env bash
# Publish a GitHub release so that it is either complete or absent (#2056).
#
# The release is created as a draft, every file in ASSET_DIR is uploaded to it
# with bounded retries, the uploaded names and sizes are checked against the
# local files, and only then is the draft published. Publishing creates the
# tag, so a draft never has one. If anything fails, the draft is deleted and
# the script exits non-zero. A rerun for the same tag deletes a leftover draft
# and starts over; a tag that is already published with every asset is a no-op.
#
# Usage: publish-release.sh TAG TARGET_SHA TITLE NOTES_FILE ASSET_DIR
# Env:   UPLOAD_ATTEMPTS (default 3), RETRY_DELAY seconds (default 10)
set -euo pipefail

tag=$1 target=$2 title=$3 notes=$4 dir=$5
attempts=${UPLOAD_ATTEMPTS:-3}
delay=${RETRY_DELAY:-10}

# "name size" per local file, sorted: what the release must end up holding.
expected=$(cd "$dir" && for f in *; do [ -f "$f" ] && printf '%s %s\n' "$f" "$(wc -c <"$f" | tr -d ' ')"; done | sort)
[ -n "$expected" ] || { echo "::error::no assets in $dir"; exit 1; }

remote_assets() {
  gh release view "$tag" --json assets --jq '.assets[] | "\(.name) \(.size)"' | sort
}

# Prints "draft", "published" or nothing when the release does not exist.
state() {
  gh release view "$tag" --json isDraft --jq 'if .isDraft then "draft" else "published" end' 2>/dev/null || true
}

case "$(state)" in
  published)
    if [ "$(remote_assets)" = "$expected" ]; then
      echo "$tag is already published with every asset; nothing to do"
      exit 0
    fi
    echo "::error::$tag is published but its assets differ from this build; fix it by hand"
    exit 1
    ;;
  draft)
    echo "Deleting leftover draft $tag from an earlier run"
    gh release delete "$tag" --yes
    ;;
esac

cleanup() {
  if [ "$(state)" = draft ]; then
    echo "::error::publishing $tag failed; deleting the draft so no partial release remains"
    gh release delete "$tag" --yes || echo "::error::could not delete draft $tag; delete it by hand"
  fi
}
trap cleanup EXIT

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
    sleep "$delay"
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
