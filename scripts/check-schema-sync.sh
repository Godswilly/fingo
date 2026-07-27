#!/bin/sh
set -eu

tmpdir="${TMPDIR:-/tmp}/fingo-schema-sync.$$"
mkdir -p "$tmpdir"
trap 'rm -rf "$tmpdir"' EXIT

# This is a lightweight name-only guard for tables, indexes, and constraints.
# It is not a semantic schema diff for column types, expressions, or options.
extract_migrations() {
	files=$(find migrations -maxdepth 1 -type f -name '*.sql' | sort)
	for file in $files; do
		sed -n \
			-e 's/^[[:space:]]*CREATE[[:space:]]\+TABLE[[:space:]]\+\([a-zA-Z_][a-zA-Z0-9_]*\).*/table:\1/ip' \
			-e 's/^[[:space:]]*CREATE[[:space:]]\+INDEX[[:space:]]\+\([a-zA-Z_][a-zA-Z0-9_]*\).*/index:\1/ip' \
			-e 's/.*CONSTRAINT[[:space:]]\+\([a-zA-Z_][a-zA-Z0-9_]*\).*/constraint:\1/ip' \
			"$file"
	done | sort -u
}

extract_schema() {
	files=$(find sql/schema -maxdepth 1 -type f -name '*.sql' | sort)
	for file in $files; do
		sed -n \
			-e 's/^[[:space:]]*CREATE[[:space:]]\+TABLE[[:space:]]\+\([a-zA-Z_][a-zA-Z0-9_]*\).*/table:\1/ip' \
			-e 's/^[[:space:]]*CREATE[[:space:]]\+INDEX[[:space:]]\+\([a-zA-Z_][a-zA-Z0-9_]*\).*/index:\1/ip' \
			-e 's/.*CONSTRAINT[[:space:]]\+\([a-zA-Z_][a-zA-Z0-9_]*\).*/constraint:\1/ip' \
			"$file"
	done | sort -u
}

extract_migrations > "$tmpdir/migrations"
extract_schema > "$tmpdir/schema"

if ! cmp -s "$tmpdir/migrations" "$tmpdir/schema"; then
	echo "Schema snapshot drift detected between migrations/ and sql/schema/." >&2
	echo "Update sql/schema after migration changes, then run make sqlc-generate." >&2
	echo >&2
	echo "Only in migrations:" >&2
	comm -23 "$tmpdir/migrations" "$tmpdir/schema" >&2 || true
	echo >&2
	echo "Only in sql/schema:" >&2
	comm -13 "$tmpdir/migrations" "$tmpdir/schema" >&2 || true
	exit 1
fi

echo "Schema snapshot matches migration tables, indexes, and constraints."
