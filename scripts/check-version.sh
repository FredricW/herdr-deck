#!/bin/sh
# Fails when a release tag (vX.Y.Z) disagrees with herdr-plugin.toml's
# version. Skips when there is no tag or no manifest yet.
#
# Usage: scripts/check-version.sh [tag]   (default: $GITHUB_REF_NAME on a tag push)
set -eu

manifest=herdr-plugin.toml
tag=${1:-}
if [ -z "$tag" ] && [ "${GITHUB_REF_TYPE:-}" = tag ]; then
	tag=${GITHUB_REF_NAME:-}
fi

if [ -z "$tag" ]; then
	echo "no release tag; skipping the version check"
	exit 0
fi
if [ ! -f "$manifest" ]; then
	echo "$manifest not found; skipping the version check"
	exit 0
fi

# The first top-level `version = "…"` line, before any [table].
version=$(awk '
	/^[[:space:]]*\[/ { exit }
	/^[[:space:]]*version[[:space:]]*=/ {
		sub(/^[^=]*=[[:space:]]*"/, ""); sub(/".*$/, ""); print; exit
	}' "$manifest")
if [ -z "$version" ]; then
	echo "$manifest has no top-level version" >&2
	exit 1
fi

if [ "${tag#v}" != "${version#v}" ]; then
	echo "tag $tag does not match $manifest version $version" >&2
	exit 1
fi
echo "tag $tag matches $manifest version $version"
