#!/usr/bin/env bash
# Serves the built web/ directory on the first free port, and prints the URL.
#
# The serving itself is serve.py, which sends Cache-Control: no-store. A
# browser left to its own devices caches protozoa.wasm and keeps running the
# previous build after a rebuild, which looks like the rebuild not working.
#
# The port search is the point: 8080 is a popular default and something else
# is often on it. Windows reports an occupied port as WinError 10013, "an
# attempt was made to access a socket in a way forbidden by its access
# permissions", which reads like a privileges problem and is not one.
#
# Usage: ./web/serve.sh [port]
set -u
cd "$(dirname "$0")"

if [[ ! -f protozoa.wasm ]]; then
  echo "web/protozoa.wasm is missing; run ./web/build.sh first" >&2
  exit 1
fi

PY=""
for candidate in python python3 py; do
  if command -v "$candidate" >/dev/null 2>&1; then
    PY="$candidate"
    break
  fi
done
if [[ -z "$PY" ]]; then
  echo "no python on PATH; serve this directory with any static file server" >&2
  echo "the one requirement is that .wasm is sent as application/wasm" >&2
  exit 1
fi

for port in "${1:-8099}" 8100 8101 8102 8103; do
  if "$PY" -c "import socket,sys; s=socket.socket(); sys.exit(s.connect_ex(('127.0.0.1',$port)) == 0)"; then
    exec "$PY" serve.py "$port"
  fi
  echo "port $port is in use, trying the next one" >&2
done
echo "no free port found in the range tried; pass one: ./web/serve.sh 9000" >&2
exit 1
