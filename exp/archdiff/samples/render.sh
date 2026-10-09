#!/bin/sh
# Re-renders the canvas samples. Run from the repository root:
#
#   sh exp/archdiff/samples/render.sh
#
# The synthetic sample needs a scratch clone with synthetic.patch applied
# on top of 0937435 as one commit; pass its path as $1 to include it.
set -eu
out=exp/archdiff/samples
layers=exp/archdiff/testdata/herdr-deck-layers.json
bin=$(mktemp -d)/archdiff
go build -o "$bin" ./exp/archdiff

render() { # name repo base head
	for w in 60 80 120; do
		for e in ports lines focus; do
			[ "$w" = 60 ] && [ "$e" = focus ] && continue
			"$bin" -repo "$2" -base "$3" -head "$4" -layers "$layers" -view canvas -edges "$e" -width "$w" >"$out/$1-$w-$e.txt"
			# colour copies only where they add something: 80 and 120, not dense
			if [ "$w" != 60 ] && [ "$1" != dense-history ]; then
				"$bin" -repo "$2" -base "$3" -head "$4" -layers "$layers" -view canvas -edges "$e" -width "$w" -color >"$out/$1-$w-$e.ansi"
			fi
		done
	done
}

render pr52-dev-manifest . 9fc47b3^1 9fc47b3^2
render pr53-pr-tab . 0937435^1 0937435^2
render pr43-github . 9e8c77a^1 9e8c77a^2
# Dense: the whole history up to PR #53, to see where each mode breaks.
render dense-history . "$(git rev-list --max-parents=0 0937435)" 0937435
if [ $# -gt 0 ]; then
	render synthetic "$1" HEAD~1 HEAD
fi
