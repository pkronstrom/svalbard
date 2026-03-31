#!/usr/bin/env python3
"""Minimal search REST API server using Python stdlib.

Replaces the socat + search-cgi.sh approach with a single Python process.
Expects environment variable: DB (path to search.db).
Arguments: PORT BIND [KIWIX_PORT]
"""
import json
import os
import re
import sqlite3
import sys
from http.server import BaseHTTPRequestHandler, HTTPServer
from urllib.parse import parse_qs, urlparse


DB_PATH = os.environ.get("DB", "")
KIWIX_PORT = "8080"
_conn: sqlite3.Connection | None = None


def _get_conn() -> sqlite3.Connection:
    global _conn
    if _conn is None:
        _conn = sqlite3.connect(DB_PATH)
        _conn.row_factory = sqlite3.Row
        _conn.execute("PRAGMA journal_mode=WAL")
    return _conn


class SearchHandler(BaseHTTPRequestHandler):
    def log_message(self, format, *args):
        pass  # suppress request logging

    def _send_json(self, code, body):
        data = json.dumps(body).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.send_header("Access-Control-Allow-Origin", "*")
        self.send_header("Connection", "close")
        self.end_headers()
        self.wfile.write(data)

    def _send_error(self, code, message):
        self._send_json(code, {"error": message})

    def do_GET(self):
        parsed = urlparse(self.path)
        path = parsed.path.rstrip("/")
        params = parse_qs(parsed.query)

        if path == "/health":
            self._handle_health()
        elif path == "/search":
            self._handle_search(params)
        elif path.startswith("/article/"):
            self._handle_article(path)
        else:
            self._send_error(404, f"Unknown route: {path}")

    def _handle_health(self):
        try:
            conn = _get_conn()
            source_count = conn.execute("SELECT count(*) FROM sources").fetchone()[0]
            article_count = conn.execute("SELECT count(*) FROM articles").fetchone()[0]
        except Exception:
            source_count = 0
            article_count = 0
        self._send_json(200, {
            "tier": "sqlite",
            "sources": source_count,
            "articles": article_count,
        })

    def _handle_search(self, params):
        q = params.get("q", [None])[0]
        if not q:
            self._send_error(400, "Missing q parameter")
            return

        # Build FTS query with prefix matching
        words = q.split()
        fts_query = " ".join(f"{w}*" for w in words)

        try:
            conn = _get_conn()
            rows = conn.execute(
                """SELECT a.id, s.filename, a.path, a.title,
                          snippet(articles_fts, 1, '>', '<', '...', 12) AS snippet
                   FROM articles_fts
                   JOIN articles a ON a.id = articles_fts.rowid
                   JOIN sources  s ON s.id = a.source_id
                   WHERE articles_fts MATCH ?
                   ORDER BY rank
                   LIMIT 20""",
                (fts_query,),
            ).fetchall()
            results = [dict(r) for r in rows]
        except Exception:
            results = []

        self._send_json(200, results)

    def _handle_article(self, path):
        match = re.match(r"^/article/(\d+)$", path)
        if not match:
            self._send_error(400, "Invalid article ID")
            return

        article_id = int(match.group(1))
        try:
            conn = _get_conn()
            row = conn.execute(
                """SELECT s.filename, a.path
                   FROM articles a
                   JOIN sources s ON s.id = a.source_id
                   WHERE a.id = ?
                   LIMIT 1""",
                (article_id,),
            ).fetchone()
        except Exception:
            row = None

        if not row:
            self._send_error(404, "Article not found")
            return

        filename, article_path = row
        book = filename.rsplit(".zim", 1)[0]
        location = f"http://localhost:{KIWIX_PORT}/content/{book}/{article_path}"
        self.send_response(302)
        self.send_header("Location", location)
        self.send_header("Access-Control-Allow-Origin", "*")
        self.send_header("Content-Length", "0")
        self.send_header("Connection", "close")
        self.end_headers()


def main():
    global DB_PATH, KIWIX_PORT
    if len(sys.argv) < 3:
        print(f"Usage: {sys.argv[0]} PORT BIND [KIWIX_PORT]", file=sys.stderr)
        sys.exit(1)

    port = int(sys.argv[1])
    bind = sys.argv[2]
    if len(sys.argv) > 3:
        KIWIX_PORT = sys.argv[3]

    if not DB_PATH:
        print("DB environment variable not set", file=sys.stderr)
        sys.exit(1)

    server = HTTPServer((bind, port), SearchHandler)
    server.serve_forever()


if __name__ == "__main__":
    main()
