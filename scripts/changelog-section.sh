#!/bin/sh
# Prints the body of CHANGELOG.md's section for <version> (`## [X.Y.Z] -
# date`, with or without the brackets or a v), without its heading or the
# blank lines around it. Fails when the file has no such section, or only
# an empty one.
#
# Usage: scripts/changelog-section.sh <version> [changelog]   (default: CHANGELOG.md)
set -eu

version=${1#v}
file=${2:-CHANGELOG.md}

awk -v want="$version" '
	/^## / {
		if (inside) exit
		v = $2
		gsub(/^\[|\]$/, "", v)
		sub(/^v/, "", v)
		if (v == want) inside = 1
		next
	}
	inside && /^\[[^]]+\]: / { next } # link references
	inside {
		if ($0 ~ /^[[:space:]]*$/) { blank++; next }
		if (printed) while (blank-- > 0) print ""
		blank = 0
		print
		printed = 1
	}
	END { if (!printed) exit 1 }
' "$file"
