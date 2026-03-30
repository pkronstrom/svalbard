#!/usr/bin/env bash
set -euo pipefail
source "$DRIVE_ROOT/.svalbard/lib/ui.sh"
source "$DRIVE_ROOT/.svalbard/lib/platform.sh"
source "$DRIVE_ROOT/.svalbard/lib/binaries.sh"

DB="$DRIVE_ROOT/data/search.db"
if [ ! -f "$DB" ]; then
    ui_error "Search index not found at data/search.db"
    ui_error "Run 'Build search index' first."
    exit 1
fi

SQLITE_BIN="$(find_binary sqlite3 2>/dev/null || true)"
if [ -z "$SQLITE_BIN" ]; then
    ui_error "sqlite3 not found. Run 'Provision tools' from the menu."
    exit 1
fi

# Show index stats
source_count="$("$SQLITE_BIN" "$DB" "SELECT count(*) FROM sources;")"
article_count="$("$SQLITE_BIN" "$DB" "SELECT count(*) FROM articles;")"
ui_header "Svalbard Cross-ZIM Search"
ui_status "Index: $source_count sources, $article_count articles"
echo ""

# Get query from argument or prompt
query="${1:-}"
if [ -z "$query" ]; then
    printf "  Search: "
    read -r query
fi
if [ -z "$query" ]; then
    ui_error "No query provided."
    exit 1
fi

# Escape single quotes for SQL safety
safe_query="${query//\'/\'\'}"

# Run FTS5 query
results="$("$SQLITE_BIN" -separator $'\t' "$DB" \
    "SELECT a.id, s.filename, a.path, a.title,
            snippet(articles_fts, 1, '>', '<', '...', 12)
     FROM articles_fts
     JOIN articles a ON a.id = articles_fts.rowid
     JOIN sources  s ON s.id = a.source_id
     WHERE articles_fts MATCH '${safe_query}'
     ORDER BY rank
     LIMIT 20;" 2>/dev/null || true)"

if [ -z "$results" ]; then
    echo "  No results found for: $query"
    exit 0
fi

# Display numbered results
echo ""
ui_header "Results for: $query"
declare -a result_ids=()
declare -a result_filenames=()
declare -a result_paths=()
num=0
while IFS=$'\t' read -r id filename path title snippet; do
    num=$((num + 1))
    result_ids+=("$id")
    result_filenames+=("$filename")
    result_paths+=("$path")
    # Derive a short label from the filename (strip .zim extension)
    label="${filename%.zim}"
    echo "  ${BOLD}${num}.${NC} [${CYAN}${label}${NC}] ${title}"
    if [ -n "$snippet" ]; then
        echo "     ${DIM}${snippet}${NC}"
    fi
done <<< "$results"

if [ "$num" -eq 0 ]; then
    echo "  No results found for: $query"
    exit 0
fi

# Prompt to open a result
echo ""
printf "  Open result # (or Enter to quit): "
read -r choice
if [ -z "$choice" ]; then
    exit 0
fi

if ! [[ "$choice" =~ ^[0-9]+$ ]] || [ "$choice" -lt 1 ] || [ "$choice" -gt "$num" ]; then
    ui_error "Invalid selection."
    exit 1
fi

idx=$((choice - 1))
filename="${result_filenames[$idx]}"
path="${result_paths[$idx]}"

# Attempt to construct kiwix-serve URL
KIWIX_BIN="$(find_binary kiwix-serve 2>/dev/null || true)"
if [ -z "$KIWIX_BIN" ]; then
    ui_error "kiwix-serve not found. Cannot open article in browser."
    echo "  ZIM: $filename"
    echo "  Path: $path"
    exit 1
fi

# Derive the kiwix book name from the ZIM filename (without .zim extension)
book="${filename%.zim}"
url="http://localhost:8080/viewer#/search?books=${book}&pattern=$(printf '%s' "$query" | sed 's/ /+/g')"
ui_status "Opening: $url"
open_browser "$url"
