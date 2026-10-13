#!/bin/sh
set -eu
repo=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
original=$(mktemp -d)
trap 'rm -rf "$original"' EXIT HUP INT TERM
cp "$repo"/backend/internal/catalog/catalogdb/*.go "$original/"
(cd "$repo/backend" && "$repo/.tools/sqlc" generate)
diff -ru "$original" "$repo/backend/internal/catalog/catalogdb"
