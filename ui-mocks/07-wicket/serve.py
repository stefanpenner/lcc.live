#!/usr/bin/env python3
"""Serve the wicket mock + proxy live lcc.live data (CORS bypass)."""
from __future__ import annotations

import http.client
import http.server
import os
import ssl
import sys
import urllib.error
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]  # ui-mocks/
UPSTREAM = "https://lcc.live"
PORT = int(os.environ.get("PORT", "8765"))

# Allow HTTPS without fuss on older Python
CTX = ssl.create_default_context()

PROXY_PREFIXES = (
    "/lcc.json",
    "/bcc.json",
    "/.json",
    "/api/",
    "/image/",
)


class Handler(http.server.SimpleHTTPRequestHandler):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, directory=str(ROOT), **kwargs)

    def log_message(self, fmt, *args):
        sys.stderr.write("%s - %s\n" % (self.address_string(), fmt % args))

    def do_OPTIONS(self):
        self.send_response(204)
        self.send_header("Access-Control-Allow-Origin", "*")
        self.send_header("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
        self.send_header("Access-Control-Allow-Headers", "*")
        self.end_headers()

    def do_HEAD(self):
        if self._is_proxy():
            self._proxy(head=True)
        else:
            super().do_HEAD()

    def do_GET(self):
        if self._is_proxy():
            self._proxy(head=False)
        else:
            super().do_GET()

    def _is_proxy(self) -> bool:
        path = self.path.split("?", 1)[0]
        return any(path == p or path.startswith(p) for p in PROXY_PREFIXES)

    def _proxy(self, head: bool) -> None:
        url = UPSTREAM + self.path
        req = urllib.request.Request(
            url,
            method="HEAD" if head else "GET",
            headers={
                "User-Agent": "lcc-wicket-mock/1.0",
                "Accept": self.headers.get("Accept", "*/*"),
            },
        )
        try:
            with urllib.request.urlopen(req, context=CTX, timeout=30) as resp:
                body = b"" if head else resp.read()
                self.send_response(resp.status)
                # Pass through useful headers
                for h in ("Content-Type", "ETag", "Cache-Control", "Last-Modified"):
                    v = resp.headers.get(h)
                    if v:
                        self.send_header(h, v)
                if not head:
                    self.send_header("Content-Length", str(len(body)))
                self.send_header("Access-Control-Allow-Origin", "*")
                self.end_headers()
                if not head and body:
                    self.wfile.write(body)
        except urllib.error.HTTPError as e:
            body = e.read() if not head else b""
            self.send_response(e.code)
            self.send_header("Content-Type", e.headers.get("Content-Type", "text/plain"))
            self.send_header("Access-Control-Allow-Origin", "*")
            if body:
                self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            if body:
                self.wfile.write(body)
        except Exception as e:
            msg = ("proxy error: %s\n" % e).encode()
            self.send_response(502)
            self.send_header("Content-Type", "text/plain")
            self.send_header("Content-Length", str(len(msg)))
            self.send_header("Access-Control-Allow-Origin", "*")
            self.end_headers()
            self.wfile.write(msg)


def main() -> None:
    http.server.ThreadingHTTPServer.allow_reuse_address = True
    server = http.server.ThreadingHTTPServer(("127.0.0.1", PORT), Handler)
    url = "http://127.0.0.1:%d/07-wicket/" % PORT
    print("wicket mock → %s" % url, flush=True)
    print("proxying %s {/lcc.json,/bcc.json,/api/*,/image/*}" % UPSTREAM, flush=True)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\nbye", flush=True)


if __name__ == "__main__":
    main()
