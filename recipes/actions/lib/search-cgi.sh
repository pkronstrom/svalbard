#!/usr/bin/env bash
set -euo pipefail

# CGI handler for search API, invoked by socat.
# Expects env: DB, SQLITE_BIN, DRIVE_ROOT, KIWIX_PORT

# --- HTTP helpers ---

send_response() {
    local status="$1" content_type="$2" body="$3"
    local length=${#body}
    printf "HTTP/1.1 %s\r\n" "$status"
    printf "Content-Type: %s\r\n" "$content_type"
    printf "Content-Length: %d\r\n" "$length"
    printf "Access-Control-Allow-Origin: *\r\n"
    printf "Connection: close\r\n"
    printf "\r\n"
    printf "%s" "$body"
}

send_redirect() {
    local location="$1"
    printf "HTTP/1.1 302 Found\r\n"
    printf "Location: %s\r\n" "$location"
    printf "Access-Control-Allow-Origin: *\r\n"
    printf "Content-Length: 0\r\n"
    printf "Connection: close\r\n"
    printf "\r\n"
}

send_error() {
    local status="$1" message="$2"
    local body="{\"error\":\"${message}\"}"
    send_response "$status" "application/json" "$body"
}

# --- URL decoding ---

url_decode() {
    local encoded="$1"
    # Replace + with space, then decode percent-encoding
    encoded="${encoded//+/ }"
    printf '%b' "${encoded//%/\\x}"
}

# --- Read HTTP request ---

read -r request_line || true
# Parse method and path: "GET /path HTTP/1.1"
method="$(echo "$request_line" | cut -d' ' -f1)"
full_path="$(echo "$request_line" | cut -d' ' -f2 | tr -d '\r')"

# Consume remaining headers
while IFS= read -r header; do
    header="$(echo "$header" | tr -d '\r')"
    [ -z "$header" ] && break
done

# Split path and query string
path="${full_path%%\?*}"
query_string=""
if [[ "$full_path" == *"?"* ]]; then
    query_string="${full_path#*\?}"
fi

# --- Routing ---

case "$method $path" in
    "GET /health")
        source_count="$("$SQLITE_BIN" "$DB" "SELECT count(*) FROM sources;" 2>/dev/null || echo "0")"
        article_count="$("$SQLITE_BIN" "$DB" "SELECT count(*) FROM articles;" 2>/dev/null || echo "0")"
        body="{\"tier\":\"sqlite\",\"sources\":${source_count},\"articles\":${article_count}}"
        send_response "200 OK" "application/json" "$body"
        ;;

    "GET /search")
        # Extract q= parameter
        raw_q=""
        IFS='&' read -ra params <<< "$query_string"
        for param in "${params[@]}"; do
            key="${param%%=*}"
            value="${param#*=}"
            if [ "$key" = "q" ]; then
                raw_q="$value"
                break
            fi
        done

        if [ -z "$raw_q" ]; then
            send_error "400 Bad Request" "Missing q parameter"
            exit 0
        fi

        query="$(url_decode "$raw_q")"
        # Escape single quotes for SQL safety
        safe_query="${query//\'/\'\'}"

        results="$("$SQLITE_BIN" -json "$DB" \
            "SELECT a.id, s.filename, a.path, a.title,
                    snippet(articles_fts, 1, '>', '<', '...', 12) AS snippet
             FROM articles_fts
             JOIN articles a ON a.id = articles_fts.rowid
             JOIN sources  s ON s.id = a.source_id
             WHERE articles_fts MATCH '${safe_query}'
             ORDER BY rank
             LIMIT 20;" 2>/dev/null || echo "[]")"

        # sqlite3 -json returns nothing for zero rows
        if [ -z "$results" ]; then
            results="[]"
        fi

        send_response "200 OK" "application/json" "$results"
        ;;

    "GET /article/"*)
        # Extract article ID from path: /article/{id}
        article_id="${path#/article/}"
        if [ -z "$article_id" ] || ! [[ "$article_id" =~ ^[0-9]+$ ]]; then
            send_error "400 Bad Request" "Invalid article ID"
            exit 0
        fi

        row="$("$SQLITE_BIN" -separator $'\t' "$DB" \
            "SELECT s.filename, a.path
             FROM articles a
             JOIN sources s ON s.id = a.source_id
             WHERE a.id = ${article_id}
             LIMIT 1;" 2>/dev/null || true)"

        if [ -z "$row" ]; then
            send_error "404 Not Found" "Article not found"
            exit 0
        fi

        IFS=$'\t' read -r filename article_path <<< "$row"
        book="${filename%.zim}"
        location="http://localhost:${KIWIX_PORT}/${book}/${article_path}"
        send_redirect "$location"
        ;;

    *)
        send_error "404 Not Found" "Unknown route: $method $path"
        ;;
esac
