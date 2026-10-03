#!/bin/sh
# Prints release notes for <tag>: the pull requests and commits on its
# first-parent history since the previous vX.Y.Z tag (or since the start).
#
# Usage: scripts/changelog.sh <tag>
set -eu

tag=$1
prev=$(git describe --tags --abbrev=0 --match 'v[0-9]*' "$tag^" 2>/dev/null) || prev=

if [ -n "$prev" ]; then
	echo "Changes since $prev:"
else
	echo "Changes:"
fi
echo
git log --first-parent --format=%H "${prev:+$prev..}$tag" | while read -r sha; do
	subject=$(git log -1 --format=%s "$sha")
	case $subject in
	"Merge pull request #"*)
		# The PR's title is the first line of the merge commit's body.
		pr=${subject#Merge pull request #}
		pr=${pr%% *}
		title=$(git log -1 --format=%b "$sha" | sed -n '/./{p;q;}')
		echo "- ${title:-$subject} (#$pr)"
		;;
	*) echo "- $subject" ;;
	esac
done
