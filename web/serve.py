#!/usr/bin/env python3
"""Static file server for the built web/ directory.

Two things it does that `python -m http.server` does not:

  * every response carries Cache-Control: no-store, so a rebuilt
    protozoa.wasm is picked up on a plain reload. Without it the browser
    heuristically caches the binary and keeps running the previous build,
    which looks exactly like the rebuild not having happened.
  * .wasm is sent as application/wasm, which WebAssembly.instantiateStreaming
    requires and which the Windows registry does not always supply.

Usage: python serve.py [port]
"""

import functools
import http.server
import os
import sys


class Handler(http.server.SimpleHTTPRequestHandler):
    extensions_map = {
        **http.server.SimpleHTTPRequestHandler.extensions_map,
        ".wasm": "application/wasm",
        ".js": "text/javascript",
        ".json": "application/json",
    }

    def end_headers(self):
        self.send_header("Cache-Control", "no-store, must-revalidate")
        self.send_header("Pragma", "no-cache")
        self.send_header("Expires", "0")
        super().end_headers()

    def log_message(self, fmt, *args):
        sys.stderr.write("%s %s\n" % (self.address_string(), fmt % args))


def main():
    port = int(sys.argv[1]) if len(sys.argv) > 1 else 8099
    root = os.path.dirname(os.path.abspath(__file__))
    handler = functools.partial(Handler, directory=root)
    server = http.server.ThreadingHTTPServer(("127.0.0.1", port), handler)
    print("serving %s at http://localhost:%d/" % (root, port), flush=True)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass


if __name__ == "__main__":
    main()
