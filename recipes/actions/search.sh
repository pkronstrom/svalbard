#!/usr/bin/env bash
set -uo pipefail
source "$DRIVE_ROOT/.svalbard/lib/ui.sh"
source "$DRIVE_ROOT/.svalbard/lib/platform.sh"
source "$DRIVE_ROOT/.svalbard/lib/binaries.sh"

DB="$DRIVE_ROOT/data/search.db"
if [ ! -f "$DB" ]; then
    ui_error "Search index not found. Run 'svalbard index' to build it."
    exit 1
fi

SQLITE_BIN="$(find_binary sqlite3 2>/dev/null || true)"
if [ -z "$SQLITE_BIN" ]; then
    ui_error "sqlite3 not found."
    exit 1
fi

# Check if semantic search is available
has_embeddings="$("$SQLITE_BIN" "$DB" "SELECT count(*) FROM sqlite_master WHERE name='embeddings';" 2>/dev/null)"
LLAMA_BIN=""
EMBED_MODEL=""
EMBED_PORT=8085
if [ "$has_embeddings" = "1" ]; then
    embed_count="$("$SQLITE_BIN" "$DB" "SELECT count(*) FROM embeddings;" 2>/dev/null)"
    if [ "$embed_count" -gt 0 ] 2>/dev/null; then
        LLAMA_BIN="$(find_binary llama-server 2>/dev/null || true)"
        # Find embedding model
        for f in "$DRIVE_ROOT"/models/*nomic*embed* "$DRIVE_ROOT"/models/*embed* "$DRIVE_ROOT"/models/*bge*; do
            [ -f "$f" ] && { EMBED_MODEL="$f"; break; }
        done
    fi
fi

source_count="$("$SQLITE_BIN" "$DB" "SELECT count(*) FROM sources;")"
article_count="$("$SQLITE_BIN" "$DB" "SELECT count(*) FROM articles;")"
mode="keyword"
[ -n "$LLAMA_BIN" ] && [ -n "$EMBED_MODEL" ] && mode="semantic"

echo ""
echo "Cross-ZIM Search ($source_count sources, $article_count articles, $mode)"
echo "────────────────────────────────"

# Semantic helpers
_embed_server_running() {
    curl -s "http://127.0.0.1:$EMBED_PORT/health" 2>/dev/null | grep -q "ok"
}

_start_embed_server() {
    echo "  Starting embedding server..."
    "$LLAMA_BIN" --model "$EMBED_MODEL" --embedding --port "$EMBED_PORT" --host 127.0.0.1 >/dev/null 2>&1 &
    EMBED_PID=$!
    for i in $(seq 1 30); do
        _embed_server_running && return 0
        sleep 1
    done
    kill "$EMBED_PID" 2>/dev/null
    EMBED_PID=""
    return 1
}

_semantic_rerank() {
    # $1 = query, $2 = candidate IDs (comma-separated)
    local q="$1" candidate_ids="$2"
    # Embed query
    local q_json
    q_json=$(printf '%s' "$q" | sed 's/"/\\"/g')
    local q_vec
    q_vec=$(curl -s "http://127.0.0.1:$EMBED_PORT/embedding" \
        -H "Content-Type: application/json" \
        -d "{\"content\": [\"search_query: $q_json\"]}" 2>/dev/null)
    [ -z "$q_vec" ] && return 1

    # Use Python for the dot product reranking (too complex for awk with BLOBs)
    python3 -c "
import json, struct, sqlite3, sys
q_data = json.loads('''$q_vec''')
q_emb = q_data[0]['embedding']
if isinstance(q_emb[0], list): q_emb = q_emb[0]
conn = sqlite3.connect('$DB')
ids = [$candidate_ids]
scores = []
for aid in ids:
    row = conn.execute('SELECT vector FROM embeddings WHERE article_id=?', (aid,)).fetchone()
    if not row: continue
    vec = struct.unpack(f'<{len(row[0])//4}f', row[0])
    dot = sum(a*b for a,b in zip(q_emb, vec))
    scores.append((aid, dot))
scores.sort(key=lambda x: -x[1])
for aid, score in scores[:20]:
    print(aid)
" 2>/dev/null
}

EMBED_PID=""
KIWIX_PID=""
KIWIX_PORT=8080

# Start kiwix-serve if not already running
_ensure_kiwix() {
    # Already running?
    curl -s "http://127.0.0.1:$KIWIX_PORT/" >/dev/null 2>&1 && return 0

    KIWIX_BIN="$(find_binary kiwix-serve 2>/dev/null || true)"
    [ -z "$KIWIX_BIN" ] && return 1

    zim_files=()
    if [ -d "$DRIVE_ROOT/zim" ]; then
        while IFS= read -r f; do
            [ -n "$f" ] && zim_files+=("$f")
        done < <(find "$DRIVE_ROOT/zim" -name "*.zim" -type f 2>/dev/null | sort)
    fi
    [ ${#zim_files[@]} -eq 0 ] && return 1

    source "$DRIVE_ROOT/.svalbard/lib/ports.sh"
    KIWIX_PORT="$(find_free_port 8080)"
    echo "  Starting kiwix-serve on port $KIWIX_PORT..."
    "$KIWIX_BIN" --port "$KIWIX_PORT" "${zim_files[@]}" >/dev/null 2>&1 &
    KIWIX_PID=$!
    sleep 2
    curl -s "http://127.0.0.1:$KIWIX_PORT/" >/dev/null 2>&1
}

cleanup() {
    [ -n "$EMBED_PID" ] && kill "$EMBED_PID" 2>/dev/null
    [ -n "$KIWIX_PID" ] && kill "$KIWIX_PID" 2>/dev/null
}
trap cleanup EXIT

# Search loop
query="${1:-}"
while true; do
    if [ -z "$query" ]; then
        printf "\n  Search (q to quit): "
        read -r query
    fi
    [[ "$query" = q || "$query" = Q || -z "$query" ]] && exit 0
    clear 2>/dev/null || true
    echo "Searching: $query"

    results=""

    if [ "$mode" = "semantic" ] && [ "$article_count" -lt 500000 ]; then
        # Pure semantic: embed query, dot-product against all articles
        if ! _embed_server_running; then
            _start_embed_server || mode="keyword"
        fi
        if _embed_server_running; then
            echo "  Semantic search..."
            all_ids=$("$SQLITE_BIN" "$DB" "SELECT id FROM articles;" | tr '\n' ',' | sed 's/,$//')
            ranked_ids=$(_semantic_rerank "$query" "$all_ids" || true)
            if [ -n "$ranked_ids" ]; then
                results="$("$SQLITE_BIN" -separator $'\t' "$DB" \
                    "WITH ranked(aid, pos) AS (VALUES $(
                        i=0; echo "$ranked_ids" | head -20 | while read -r rid; do
                            [ $i -gt 0 ] && printf ","
                            printf "(%s,%s)" "$rid" "$i"
                            i=$((i+1))
                        done
                    ))
                     SELECT a.id, s.filename, a.path, a.title,
                            substr(a.body, 1, 120)
                     FROM ranked r
                     JOIN articles a ON a.id = r.aid
                     JOIN sources  s ON s.id = a.source_id
                     ORDER BY r.pos;" 2>/dev/null || true)"
            fi
        fi
    fi

    if [ "$mode" = "semantic" ] && [ "$article_count" -ge 500000 ]; then
        # Large archive: FTS prefilter + semantic rerank
        safe_query="${query//\'/\'\'}"
        fts_query=""
        for word in $safe_query; do
            [ -n "$fts_query" ] && fts_query="$fts_query "
            fts_query="${fts_query}${word}*"
        done
        results="$("$SQLITE_BIN" -separator $'\t' "$DB" \
            "SELECT a.id, s.filename, a.path, a.title,
                    snippet(articles_fts, 1, '»', '«', '...', 12)
             FROM articles_fts
             JOIN articles a ON a.id = articles_fts.rowid
             JOIN sources  s ON s.id = a.source_id
             WHERE articles_fts MATCH '${fts_query}'
             ORDER BY rank LIMIT 200;" 2>/dev/null || true)"
        if [ -n "$results" ] && _embed_server_running; then
            echo "  Reranking..."
            candidate_ids=$(echo "$results" | cut -f1 | tr '\n' ',' | sed 's/,$//')
            ranked_ids=$(_semantic_rerank "$query" "$candidate_ids" || true)
            if [ -n "$ranked_ids" ]; then
                results="$("$SQLITE_BIN" -separator $'\t' "$DB" \
                    "WITH ranked(aid, pos) AS (VALUES $(
                        i=0; echo "$ranked_ids" | head -20 | while read -r rid; do
                            [ $i -gt 0 ] && printf ","
                            printf "(%s,%s)" "$rid" "$i"
                            i=$((i+1))
                        done
                    ))
                     SELECT a.id, s.filename, a.path, a.title,
                            substr(a.body, 1, 120)
                     FROM ranked r
                     JOIN articles a ON a.id = r.aid
                     JOIN sources  s ON s.id = a.source_id
                     ORDER BY r.pos;" 2>/dev/null || true)"
            fi
        fi
    fi

    if [ "$mode" = "keyword" ] && [ -z "$results" ]; then
        # FTS only fallback
        safe_query="${query//\'/\'\'}"
        fts_query=""
        for word in $safe_query; do
            [ -n "$fts_query" ] && fts_query="$fts_query "
            fts_query="${fts_query}${word}*"
        done
        results="$("$SQLITE_BIN" -separator $'\t' "$DB" \
            "SELECT a.id, s.filename, a.path, a.title,
                    snippet(articles_fts, 1, '»', '«', '...', 12)
             FROM articles_fts
             JOIN articles a ON a.id = articles_fts.rowid
             JOIN sources  s ON s.id = a.source_id
             WHERE articles_fts MATCH '${fts_query}'
             ORDER BY rank LIMIT 20;" 2>/dev/null || true)"
    fi

    if [ -z "$results" ]; then
        echo "  No results for: $query"
        query=""
        continue
    fi

    # Display results
    ids=(); filenames=(); paths=(); num=0
    echo ""
    echo "────────────────────────────────"
    while IFS=$'\t' read -r id filename path title snippet; do
        num=$((num + 1))
        ids+=("$id"); filenames+=("$filename"); paths+=("$path")
        label="${filename%.zim}"
        echo "  ${num}. [$label] $title"
        [ -n "$snippet" ] && echo "     $snippet"
    done <<< "$results"

    echo ""
    printf "  Open # (or new search, q to quit): "
    read -r choice

    [[ "$choice" = q || "$choice" = Q ]] && exit 0

    if [[ "$choice" =~ ^[0-9]+$ ]] && (( choice >= 1 && choice <= num )); then
        idx=$((choice - 1))
        filename="${filenames[$idx]}"
        book="${filename%.zim}"
        if _ensure_kiwix; then
            article_path="${paths[$idx]}"
            url="http://localhost:${KIWIX_PORT}/content/${book}/${article_path}"
            echo "  Opening: $url"
            open_browser "$url"
        else
            echo "  kiwix-serve not available. Article: $book / ${paths[$idx]}"
        fi
    elif [ -n "$choice" ]; then
        query="$choice"
        continue
    fi

    query=""
done
