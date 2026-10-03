#!/bin/sh
# Prints the GitHub Release notes for <tag>: CHANGELOG.md's section for the
# tag's version, else (a tag from before the changelog, or one it misses)
# scripts/changelog.sh's list of PRs and commits up to <rev>.
#
# Usage: scripts/release-notes.sh <tag> [rev]   (rev defaults to the tag)
set -eu

tag=$1
rev=${2:-$tag}
dir=$(dirname "$0")

if [ -f CHANGELOG.md ] && notes=$("$dir/changelog-section.sh" "$tag" CHANGELOG.md); then
	printf '%s\n' "$notes"
else
	"$dir/changelog.sh" "$rev"
fi
