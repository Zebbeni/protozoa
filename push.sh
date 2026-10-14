#!/usr/bin/env bash
# Uploads what dist.sh staged to itch.io with butler.
#
#   ./push.sh zebbeni/protozoa
#
# Two channels: "html5", which itch serves in the browser because the channel
# name contains "html", and "windows", which it offers as a download. The
# version is the short commit, so a build on the page can be traced back to
# the source it came from.
set -euo pipefail

cd "$(dirname "$0")"

if [[ $# -lt 1 ]]; then
  echo "usage: ./push.sh <user>/<game>   (e.g. ./push.sh zebbeni/protozoa)" >&2
  exit 1
fi
TARGET="$1"

if [[ ! -f dist/web/protozoa.wasm || ! -f dist/windows/protozoa.exe ]]; then
  echo "dist/ is missing or incomplete; run ./dist.sh first" >&2
  exit 1
fi

VERSION="$(git rev-parse --short HEAD)"
if ! git diff --quiet HEAD -- . ':(exclude)dist'; then
  VERSION="$VERSION-dirty"
fi

echo "pushing $VERSION to $TARGET"
butler push dist/web "$TARGET:html5" --userversion "$VERSION"
butler push dist/windows "$TARGET:windows" --userversion "$VERSION"

echo
butler status "$TARGET"
