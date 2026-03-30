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

source_count="$("$SQLITE_BIN" "$DB" "SELECT count(*) FROM sources;")"
article_count="$("$SQLITE_BIN" "$DB" "SELECT count(*) FROM articles;")"

echo ""
echo "Cross-ZIM Search ($source_count sources, $article_count articles)"
echo "────────────────────────────────"

# Search loop
query="${1:-}"
while true; do
    if [ -z "$query" ]; then
        echo ""
        printf "  Search (q to quit): "
        read -r query
    fi
    [[ "$query" = q || "$query" = Q || -z "$query" ]] && exit 0

    safe_query="${query//\'/\'\'}"

    results="$("$SQLITE_BIN" -separator $'\t' "$DB" \
        "SELECT a.id, s.filename, a.path, a.title,
                snippet(articles_fts, 1, '»', '«', '...', 12)
         FROM articles_fts
         JOIN articles a ON a.id = articles_fts.rowid
         JOIN sources  s ON s.id = a.source_id
         WHERE articles_fts MATCH '${safe_query}'
         ORDER BY rank
         LIMIT 20;" 2>/dev/null || true)"

    if [ -z "$results" ]; then
        echo "  No results for: $query"
        query=""
        continue
    fi

    # Parse results
    ids=(); filenames=(); paths=(); num=0
    echo ""
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

    # If it's a number, open that result
    if [[ "$choice" =~ ^[0-9]+$ ]] && (( choice >= 1 && choice <= num )); then
        idx=$((choice - 1))
        filename="${filenames[$idx]}"
        book="${filename%.zim}"
        url="http://localhost:8080/viewer#/search?books=${book}&pattern=$(printf '%s' "$query" | sed 's/ /+/g')"
        echo "  Opening: $url"
        open_browser "$url"
    elif [ -n "$choice" ]; then
        # Treat as a new search query
        query="$choice"
        continue
    fi

    query=""
done
