#!/usr/bin/env python3
"""Serve the published skid site on loopback for the Cloudflare tunnel.

Usage: serve.py <port> [root]   (skid-web.service; the tunnel maps skid.nielseriknandal.com to 127.0.0.1:<port>)

Serves /srv/skid-web/current, the release that release.py activated; the link is followed on every request, so
activating a release needs no restart. Static files only: no directory listings, dotfiles or symlinks;
/page -> /page/ redirects; 404.html for anything missing; HTML revalidated on every visit, other files cached for
five minutes; one byte range per request, which Safari needs to play the demo video.
`root` serves another folder the same way, e.g. `serve.py 8000 site` in a checkout.
Adapted from atlas-kit's deploy/devbox/serve.py.
"""
import email.utils
import http.server
import re
import shutil
import sys
import urllib.parse
from pathlib import Path

ROOT = Path("/srv/skid-web/current")
TYPES = {".html": "text/html; charset=utf-8", ".css": "text/css; charset=utf-8", ".js": "text/javascript; charset=utf-8",
         ".svg": "image/svg+xml", ".png": "image/png", ".mp4": "video/mp4", ".woff2": "font/woff2",
         ".txt": "text/plain; charset=utf-8", ".md": "text/plain; charset=utf-8"}
SECURITY = (("X-Content-Type-Options", "nosniff"), ("Referrer-Policy", "strict-origin-when-cross-origin"),
            ("Strict-Transport-Security", "max-age=31536000"))
RANGE = re.compile(r"bytes=(\d*)-(\d*)\Z")


class Handler(http.server.BaseHTTPRequestHandler):
    server_version = "skid-web"
    sys_version = ""
    protocol_version = "HTTP/1.1"
    timeout = 30

    def do_GET(self):
        self.respond(head=False)

    def do_HEAD(self):
        self.respond(head=True)

    def log_request(self, code="-", size="-"):  # path without the query string, to the journal
        sys.stderr.write(f"{self.command} {urllib.parse.urlsplit(self.path).path} {code}\n")

    def respond(self, head):
        url = urllib.parse.urlsplit(self.path)
        try:
            path = urllib.parse.unquote(url.path, errors="strict")
        except UnicodeDecodeError:
            return self.plain(400, "bad request", head)
        parts = [p for p in path.split("/") if p]
        if not path.startswith("/") or "//" in path or any(p.startswith(".") or "\\" in p or "\0" in p for p in parts):
            return self.missing(head)
        try:
            root = ROOT.resolve(strict=True)
        except OSError:
            return self.plain(503, "no release is active", head)
        target = root.joinpath(*parts)
        if target.is_dir() and not target.is_symlink():
            if not path.endswith("/"):
                return self.redirect(url.path + "/" + (f"?{url.query}" if url.query else ""), head)
            target = target / "index.html"
        if target.is_symlink() or not target.is_file() or root not in target.resolve().parents:
            return self.missing(head)
        self.file(200, target, head)

    def file(self, status, f, head):
        st = f.stat()
        etag = f'"{st.st_size:x}-{int(st.st_mtime_ns):x}"'
        cache = "no-cache" if f.suffix == ".html" or status != 200 else "public, max-age=300"
        headers = [("Content-Type", TYPES.get(f.suffix, "application/octet-stream")), ("Cache-Control", cache),
                   ("Last-Modified", email.utils.formatdate(st.st_mtime, usegmt=True)), ("ETag", etag),
                   ("Accept-Ranges", "bytes")]
        if status == 200 and self.headers.get("If-None-Match") == etag:
            self.send_response(304)
            for k, v in headers[1:] + list(SECURITY):
                self.send_header(k, v)
            self.send_header("Content-Length", "0")
            return self.end_headers()
        start, end = 0, st.st_size - 1
        wanted = self.headers.get("Range") if status == 200 else None
        if wanted:
            match = RANGE.match(wanted.strip())
            if match and match.group(1):  # bytes=a- or bytes=a-b
                start = int(match.group(1))
                end = min(int(match.group(2)), end) if match.group(2) else end
            elif match and match.group(2):  # bytes=-n: the last n bytes
                start = max(st.st_size - int(match.group(2)), 0)
            if not match or start > end:
                self.send_response(416)
                self.send_header("Content-Range", f"bytes */{st.st_size}")
                self.send_header("Content-Length", "0")
                return self.end_headers()
            status = 206
            headers.append(("Content-Range", f"bytes {start}-{end}/{st.st_size}"))
        self.send_response(status)
        for k, v in headers + list(SECURITY):
            self.send_header(k, v)
        self.send_header("Content-Length", str(end - start + 1))
        self.end_headers()
        if not head:
            with open(f, "rb") as fh:
                fh.seek(start)
                remaining = end - start + 1
                while remaining > 0 and (chunk := fh.read(min(1 << 16, remaining))):
                    self.wfile.write(chunk)
                    remaining -= len(chunk)

    def missing(self, head):
        try:
            page = ROOT.resolve(strict=True) / "404.html"
        except OSError:
            page = None
        if page and page.is_file():
            return self.file(404, page, head)
        self.plain(404, "not found", head)

    def redirect(self, location, head):
        self.send_response(308)
        self.send_header("Location", location)
        self.send_header("Content-Length", "0")
        for k, v in SECURITY:
            self.send_header(k, v)
        self.end_headers()

    def plain(self, status, text, head):
        body = (text + "\n").encode()
        self.send_response(status)
        self.send_header("Content-Type", "text/plain; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.send_header("Cache-Control", "no-store")
        self.end_headers()
        if not head:
            self.wfile.write(body)


def main():
    global ROOT
    if len(sys.argv) not in (2, 3) or not sys.argv[1].isdigit():
        sys.exit("usage: serve.py <port> [root]")
    if len(sys.argv) == 3:
        ROOT = Path(sys.argv[2]).absolute()
    server = http.server.ThreadingHTTPServer(("127.0.0.1", int(sys.argv[1])), Handler)
    server.daemon_threads = True
    server.serve_forever()


if __name__ == "__main__":
    main()
