#!/usr/bin/env bash

set -euo pipefail

HEADER="// Copyright (c) $(date +%Y) Thomas Pham. All rights reserved."


find . -type f -name "*.go" ! -path "./vendor/*" ! -path "./testdata/*" | while read -r file; do
    if head -n 1 "$file" | grep -qF "$HEADER"; then
        echo "Skipping $file (header already present)"
        continue
    fi

    echo "Adding header to $file"
    tmp=$(mktemp)
    {
        echo "$HEADER"
        cat "$file"
    } > "$tmp"
    mv "$tmp" "$file"
done
