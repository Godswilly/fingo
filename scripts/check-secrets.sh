#!/bin/sh
set -eu

scope="${CHECK_SECRETS_SCOPE:-staged}"

case "$scope" in
staged)
	files=$(git diff --cached --name-only --diff-filter=ACMR)
	;;
all)
	files=$(git ls-files)
	;;
*)
	echo "Unknown CHECK_SECRETS_SCOPE: $scope" >&2
	exit 1
	;;
esac

if [ -z "$files" ]; then
	exit 0
fi

pattern='(-----BEGIN (RSA |DSA |EC |OPENSSH |PGP )?PRIVATE KEY-----|AKIA[0-9A-Z]{16}|ghp_[A-Za-z0-9_]{36,}|github_pat_[A-Za-z0-9_]{20,}|xox[baprs]-[A-Za-z0-9-]{10,})'
found=false

for file in $files; do
	if [ ! -f "$file" ]; then
		continue
	fi

	if grep -I -n -E "$pattern" "$file" >/tmp/fingo-secret-scan.$$ 2>/dev/null; then
		echo "Potential secret found in $file:"
		sed 's/^/  /' /tmp/fingo-secret-scan.$$
		found=true
	fi
done

rm -f /tmp/fingo-secret-scan.$$

if [ "$found" = true ]; then
	echo
	echo "Blocked: potential secret detected. Remove the secret or rotate it before committing."
	exit 1
fi
