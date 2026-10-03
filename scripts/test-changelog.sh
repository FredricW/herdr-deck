#!/bin/sh
# Tests scripts/changelog-section.sh, the changelog check in
# scripts/check-version.sh and scripts/release-notes.sh, in scratch
# repositories. Needs git and awk; not Go.
#
# Usage: scripts/test-changelog.sh
set -eu

repo=$(cd "$(dirname "$0")/.." && pwd)
work=$(mktemp -d "${TMPDIR:-/tmp}/herdr-deck-changelog-test.XXXXXX")
trap 'rm -rf "$work"' EXIT
failures=0

pass() { echo "ok   $1"; }
fail() {
	echo "FAIL $1"
	failures=$((failures + 1))
}
# expect <name> <want> <got>
expect() {
	if [ "$2" = "$3" ]; then pass "$1"; else
		fail "$1"
		printf '  want: %s\n  got:  %s\n' "$2" "$3"
	fi
}

cat >"$work/CHANGELOG.md" <<'MD'
# Changelog

## [Unreleased]

### Added

- Something new.

## [0.2.0] - 2026-10-09

### Added

- Two.


### Fixed

- A fix.

## v0.1.1 - 2026-10-05

- Plain heading.

## [0.1.0] - 2026-10-01

## [0.0.1] - 2026-09-01

- Last one.

[0.2.0]: https://example.com/v0.2.0
MD

section() { "$repo/scripts/changelog-section.sh" "$@" "$work/CHANGELOG.md" 2>&1; }

expect "a bracketed section, blank lines kept inside, trimmed outside" \
	"$(printf '### Added\n\n- Two.\n\n\n### Fixed\n\n- A fix.')" "$(section 0.2.0)"
expect "a v on the tag" "$(section 0.2.0)" "$(section v0.2.0)"
expect "a heading without brackets" "- Plain heading." "$(section 0.1.1)"
expect "the last section, without link references" "- Last one." "$(section 0.0.1)"
expect "Unreleased" "$(printf '### Added\n\n- Something new.')" "$(section Unreleased)"
if section 0.1.0 >/dev/null; then fail "an empty section fails"; else pass "an empty section fails"; fi
if section 0.3.0 >/dev/null; then fail "a missing section fails"; else pass "a missing section fails"; fi
if section 0.2 >/dev/null; then fail "a prefix is not a match"; else pass "a prefix is not a match"; fi

# check-version.sh in a folder with a manifest at 0.2.0.
mkdir -p "$work/check/scripts"
cp "$repo/scripts/check-version.sh" "$repo/scripts/changelog-section.sh" "$work/check/scripts/"
printf 'id = "herdr-deck"\nversion = "0.2.0"\n' >"$work/check/herdr-plugin.toml"
check() { (cd "$work/check" && sh scripts/check-version.sh "$@" >/dev/null 2>&1); }
if check v0.2.0; then pass "check: no CHANGELOG.md skips the changelog"; else fail "check: no CHANGELOG.md skips the changelog"; fi
cp "$work/CHANGELOG.md" "$work/check/"
if check v0.2.0; then pass "check: a tag with a section passes"; else fail "check: a tag with a section passes"; fi
printf 'id = "herdr-deck"\nversion = "0.3.0"\n' >"$work/check/herdr-plugin.toml"
if check v0.3.0; then fail "check: a tag without a section fails"; else pass "check: a tag without a section fails"; fi
if check v0.2.0; then fail "check: a tag that disagrees with the manifest fails"; else pass "check: a tag that disagrees with the manifest fails"; fi
if check; then pass "check: no tag skips"; else fail "check: no tag skips"; fi

# release-notes.sh in a git repository with two tagged PR merges.
git init -q "$work/notes"
mkdir -p "$work/notes/scripts"
cp "$repo/scripts/release-notes.sh" "$repo/scripts/changelog-section.sh" "$repo/scripts/changelog.sh" "$work/notes/scripts/"
g() { git -C "$work/notes" -c user.name=test -c user.email=test@example.com "$@"; }
g add -A
g commit -qm "scripts"
g tag v0.1.0
g commit -q --allow-empty -m "Merge pull request #2 from acme/feature" -m "Add a feature"
g tag v0.1.1
notes() { (cd "$work/notes" && sh scripts/release-notes.sh "$@" 2>&1); }
expect "notes: no CHANGELOG.md lists the PRs" \
	"$(printf 'Changes since v0.1.0:\n\n- Add a feature (#2)')" "$(notes v0.1.1)"
cp "$work/CHANGELOG.md" "$work/notes/"
expect "notes: the tag's section" "- Plain heading." "$(notes v0.1.1)"
expect "notes: a tag the changelog misses lists the PRs up to rev" \
	"$(printf 'Changes since v0.1.0:\n\n- Add a feature (#2)')" "$(notes v0.3.0 v0.1.1)"

if [ "$failures" -gt 0 ]; then
	echo "$failures failed"
	exit 1
fi
echo "all passed"
