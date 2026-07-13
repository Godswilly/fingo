#!/bin/sh
set -eu

module_path=$(go list -m)
violations=""

add_violation() {
	violations="${violations}
- $1 imports $2 ($3)"
}

check_import() {
	pkg="$1"
	imp="$2"

	case "$pkg" in
	"$module_path/internal/domain" | "$module_path/internal/domain/"*)
		case "$imp" in
		"$module_path/internal/app" | "$module_path/internal/app/"* | \
			"$module_path/internal/transport" | "$module_path/internal/transport/"* | \
			"$module_path/internal/persistence" | "$module_path/internal/persistence/"* | \
			"$module_path/internal/messaging" | "$module_path/internal/messaging/"* | \
			"$module_path/internal/platform" | "$module_path/internal/platform/"*)
			add_violation "$pkg" "$imp" "domain packages must not import outer layers"
			;;
		esac
		;;
	"$module_path/internal/app" | "$module_path/internal/app/"*)
		case "$imp" in
		"$module_path/internal/transport" | "$module_path/internal/transport/"* | \
			"$module_path/internal/persistence" | "$module_path/internal/persistence/"* | \
			"$module_path/internal/messaging" | "$module_path/internal/messaging/"* | \
			"$module_path/internal/platform" | "$module_path/internal/platform/"*)
			add_violation "$pkg" "$imp" "app packages must depend inward only"
			;;
		esac
		;;
	"$module_path/internal/transport" | "$module_path/internal/transport/"*)
		case "$imp" in
		"$module_path/internal/persistence" | "$module_path/internal/persistence/"* | \
			"$module_path/internal/messaging" | "$module_path/internal/messaging/"*)
			add_violation "$pkg" "$imp" "transport adapters must not depend on outbound adapters"
			;;
		esac
		;;
	"$module_path/internal/persistence" | "$module_path/internal/persistence/"*)
		case "$imp" in
		"$module_path/internal/transport" | "$module_path/internal/transport/"* | \
			"$module_path/internal/messaging" | "$module_path/internal/messaging/"*)
			add_violation "$pkg" "$imp" "persistence adapters must not depend on transport or messaging adapters"
			;;
		esac
		;;
	"$module_path/internal/messaging" | "$module_path/internal/messaging/"*)
		case "$imp" in
		"$module_path/internal/transport" | "$module_path/internal/transport/"* | \
			"$module_path/internal/persistence" | "$module_path/internal/persistence/"*)
			add_violation "$pkg" "$imp" "messaging adapters must not depend on transport or persistence adapters"
			;;
		esac
		;;
	"$module_path/cmd" | "$module_path/cmd/"*)
		case "$imp" in
		"$module_path/internal/domain" | "$module_path/internal/domain/"*)
			add_violation "$pkg" "$imp" "cmd packages must not import domain directly"
			;;
		esac
		;;
	esac
}

for pkg in $(go list ./...); do
	imports=$(go list -f '{{range .Imports}}{{.}}{{"\n"}}{{end}}' "$pkg")
	for imp in $imports; do
		check_import "$pkg" "$imp"
	done
done

if [ -n "$violations" ]; then
	echo "Package boundary violations found:"
	echo "$violations"
	exit 1
fi

echo "Package boundaries OK."
