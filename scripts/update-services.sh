#!/bin/bash
# Update go-common dependency in all meepo-*-service repos in the same parent directory.
# Usage: ./scripts/update-services.sh [commit-hash]
#   commit-hash: optional, defaults to latest commit on current branch

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
GO_COMMON_DIR="$(dirname "$SCRIPT_DIR")"
PARENT_DIR="$(dirname "$GO_COMMON_DIR")"

# Determine the commit hash to update to
if [ -n "$1" ]; then
    HASH="$1"
else
    HASH="$(cd "$GO_COMMON_DIR" && git rev-parse HEAD)"
fi

echo "Updating go-common to: $HASH"
echo "Scanning for services in: $PARENT_DIR"
echo ""

for dir in "$PARENT_DIR"/meepo-*-service; do
    [ -d "$dir" ] || continue
    [ -f "$dir/go.mod" ] || continue

    # Check if this service depends on go-common
    if ! grep -q "github.com/infigaming-com/go-common" "$dir/go.mod"; then
        continue
    fi

    echo "==> $(basename "$dir")"
    (
        cd "$dir"
        go get "github.com/infigaming-com/go-common@$HASH"
        go mod tidy
    )
    echo ""
done

echo "Done."
