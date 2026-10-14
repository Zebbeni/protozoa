#!/usr/bin/env bash
# Stages the files that go to itch.io, under dist/.
#
#   dist/web/      index.html, wasm_exec.js, protozoa.wasm — the browser build
#   dist/windows/  protozoa.exe — the native build
#
# It does not upload anything. ./push.sh does that, and takes the itch target.
set -euo pipefail

cd "$(dirname "$0")"

rm -rf dist
mkdir -p dist/web dist/windows

echo "building the browser version..."
./web/build.sh >/dev/null
# Only the three files the page needs: build.sh, serve.sh and serve.py are
# development tools and have no business in a release.
cp web/index.html web/wasm_exec.js web/protozoa.wasm dist/web/

echo "building the windows version..."
# -s -w strips the symbol table and DWARF; -H=windowsgui keeps a console
# window from opening behind the game.
go build -ldflags "-s -w -H=windowsgui" -o dist/windows/protozoa.exe .

echo
echo "staged:"
for f in dist/web/* dist/windows/*; do
  printf "  %-28s %s\n" "$f" "$(du -h "$f" | cut -f1)"
done
echo
echo "to upload, run: ./push.sh <user>/<game>"
