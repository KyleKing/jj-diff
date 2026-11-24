#!/bin/sh
# Open jj-diff on a throwaway jj repository built from invented content, so a
# README recording shows a real screen and publishes nothing but this script.
#
#   demo/scene.sh squash     -i on the working copy, to move hunks with s
#   demo/scene.sh split      --split on the working copy, the column view
#   demo/scene.sh conflicts  --resolve on a merge whose sides disagree
#   demo/scene.sh palette    -i on the working copy, for the palette and legend
#
# Everything the run reads or writes lives under demo/.work/<scene>: the
# repository, and a HOME, XDG directories, and jj config of its own, so the
# recording neither reads the user's jj settings nor files the repository in
# their jj state. The directory is rebuilt every run and kept afterwards.
set -eu

here=$(cd "$(dirname "$0")" && pwd)
name=${1:-}

case "$name" in
  squash) args="-i" ;;
  split) args="--split" ;;
  conflicts) args="--resolve -r conflicted" ;;
  palette) args="-i" ;;
  *)
    echo "usage: $(basename "$0") squash|split|conflicts|palette" >&2
    exit 2
    ;;
esac

go build -o "$here/../jj-diff" "$here/../cmd/jj-diff"

work=$here/.work/$name
rm -rf "$work"
mkdir -p "$work/home" "$work/config" "$work/state/jj-diff" "$work/cache"

export HOME="$work/home"
export XDG_CONFIG_HOME="$work/config"
export XDG_STATE_HOME="$work/state"
export XDG_CACHE_HOME="$work/cache"
export JJ_CONFIG="$work/jj.toml"

cat > "$JJ_CONFIG" <<'CONF'
[user]
name = "Demo Author"
email = "demo@example.invalid"

[ui]
paginate = "never"
CONF

# A fresh state directory is a first launch, which opens on the welcome card
# until a key closes it. The coach ledger marks this reader as one who has been
# here before, so the recording's first key does what the tape means it to.
printf '{"first":"2026-01-01T00:00:00Z","welcomed":"2026-01-01T00:00:00Z"}\n' \
  > "$XDG_STATE_HOME/jj-diff/coach.json"

repo=$work/pantry
jj --quiet git init "$repo"
cd "$repo"

"$here/fixture.sh" "$name"

# shellcheck disable=SC2086 # args is a command line, so it has to split
exec "$here/../jj-diff" $args
