#!/usr/bin/env bash
# Build the wasm binary and stage Go's wasm runtime alongside index.html.
# Run from the project root: ./web/build.sh
set -euo pipefail

cd "$(dirname "$0")/.."

GOROOT="$(go env GOROOT)"
WASM_EXEC=""
for candidate in "$GOROOT/lib/wasm/wasm_exec.js" "$GOROOT/misc/wasm/wasm_exec.js"; do
  if [[ -f "$candidate" ]]; then
    WASM_EXEC="$candidate"
    break
  fi
done
if [[ -z "$WASM_EXEC" ]]; then
  echo "could not find wasm_exec.js under \$GOROOT ($GOROOT)" >&2
  exit 1
fi

GOOS=js GOARCH=wasm go build -o web/protozoa.wasm .
cp "$WASM_EXEC" web/wasm_exec.js

echo "built web/protozoa.wasm ($(du -h web/protozoa.wasm | cut -f1))"
echo "to run: serve the web/ directory and open in a browser, e.g.:"
echo "  go run github.com/jpillora/serve-static@latest -path ./web -port 8080"
echo "  python3 -m http.server 8080 --directory web"
