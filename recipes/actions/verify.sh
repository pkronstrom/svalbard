#!/usr/bin/env bash
set -euo pipefail
source "$DRIVE_ROOT/.svalbard/lib/ui.sh"
source "$DRIVE_ROOT/.svalbard/lib/platform.sh"
source "$DRIVE_ROOT/.svalbard/lib/binaries.sh"

ui_header "Verifying drive integrity"

SHA_CMD=""
SHA_BIN="$(find_binary sha256sum 2>/dev/null || true)"
if [ -n "$SHA_BIN" ]; then
    SHA_CMD="$SHA_BIN"
elif command -v sha256sum >/dev/null 2>&1; then
    SHA_CMD="sha256sum"
elif command -v shasum >/dev/null 2>&1; then
    SHA_CMD="shasum -a 256"
else
    ui_error "No SHA-256 tool found."
    exit 1
fi

if [ ! -f "$DRIVE_ROOT/.svalbard/checksums.sha256" ]; then
    ui_error "No checksum file found. Verification not available for this drive."
    exit 1
fi

passed=0
failed=0
missing=0

while IFS='  ' read -r expected_hash filepath; do
    [ -z "$expected_hash" ] && continue
    [[ "$expected_hash" == \#* ]] && continue
    full_path="$DRIVE_ROOT/$filepath"
    if [ ! -f "$full_path" ]; then
        echo "  ${YELLOW}MISSING${NC}  $filepath"
        missing=$((missing + 1))
        continue
    fi
    actual_hash="$($SHA_CMD "$full_path" | awk '{print $1}')"
    if [ "$actual_hash" = "$expected_hash" ]; then
        echo "  ${GREEN}OK${NC}       $filepath"
        passed=$((passed + 1))
    else
        echo "  ${RED}FAIL${NC}     $filepath"
        failed=$((failed + 1))
    fi
done < "$DRIVE_ROOT/.svalbard/checksums.sha256"

echo ""
echo "  Passed: $passed  Failed: $failed  Missing: $missing"
