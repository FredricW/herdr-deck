#!/bin/sh
# Puts the herdr-deck binary at bin/herdr-deck. herdr runs this as the
# plugin's build step, in a shallow, detached checkout of the release.
#
# When the checkout is exactly the release named by herdr-plugin.toml's
# `version`, it downloads that release's prebuilt binary for this machine and
# checks it against the release's SHA256SUMS. A checkout on a branch (a
# linked development checkout), with local changes, or on another commit
# builds from source, and so does any failure along the way.
#
#   HERDR_DECK_BUILD=source         always build from source
#   HERDR_DECK_DOWNLOAD_URL=<url>   download from <url>/v<version>/ instead of
#                                   the GitHub release of `origin`
set -u

cd "$(dirname "$0")/.." || exit 1

say() { printf 'herdr-deck build: %s\n' "$*" >&2; }

version=$(awk '
	/^[[:space:]]*\[/ { exit }
	/^[[:space:]]*version[[:space:]]*=/ {
		sub(/^[^=]*=[[:space:]]*"/, ""); sub(/".*$/, ""); print; exit
	}' herdr-plugin.toml 2>/dev/null)
version=${version#v}
tag=v$version

build_from_source() {
	[ -n "${tmp:-}" ] && rm -rf "$tmp"
	say "$1"
	if ! command -v go >/dev/null 2>&1; then
		say "go is not installed: install Go 1.27 or newer (https://go.dev/dl/), then install again"
		exit 1
	fi
	# A development checkout says where it is (v0.1.0-3-gabc1234-dirty);
	# herdr's checkout has no tags, so it gets the manifest's version.
	stamp=$(git describe --tags --dirty 2>/dev/null) || stamp=
	[ -n "$stamp" ] || stamp=$tag
	[ "$stamp" != v ] || stamp=
	say "building from source: go build -ldflags \"-X main.version=$stamp\""
	mkdir -p bin || exit 1
	out=$(mktemp "bin/.herdr-deck.XXXXXX") || exit 1
	if ! go build -ldflags "-X main.version=$stamp" -o "$out" ./cmd/herdr-deck; then
		rm -f "$out"
		exit 1
	fi
	# Rename, never copy over: macOS kills a binary rewritten in place.
	chmod 755 "$out" && mv -f "$out" bin/herdr-deck || { rm -f "$out"; exit 1; }
	say "built bin/herdr-deck from source"
	exit 0
}

if [ "${HERDR_DECK_BUILD:-}" = source ]; then
	build_from_source "HERDR_DECK_BUILD=source is set"
fi
[ -n "$version" ] || build_from_source "herdr-plugin.toml has no version"

case "$(uname -s)" in
Darwin) os=darwin ;;
Linux) os=linux ;;
*) build_from_source "there is no prebuilt binary for $(uname -s)" ;;
esac
case "$(uname -m)" in
arm64 | aarch64) arch=arm64 ;;
x86_64 | amd64) arch=amd64 ;;
*) build_from_source "there is no prebuilt binary for $(uname -m)" ;;
esac
asset="herdr-deck_${version}_${os}_${arch}.tar.gz"

# A prebuilt binary is only the release commit itself, as herdr checks it
# out: detached, with no local changes.
origin=
if git rev-parse --git-dir >/dev/null 2>&1; then
	if git symbolic-ref -q HEAD >/dev/null 2>&1; then
		build_from_source "this checkout is on a branch (a development checkout)"
	fi
	if [ -n "$(git status --porcelain --untracked-files=no 2>/dev/null)" ]; then
		build_from_source "this checkout has uncommitted changes"
	fi
	head=$(git rev-parse HEAD 2>/dev/null)
	release=$(git rev-parse -q --verify "refs/tags/$tag^{commit}" 2>/dev/null)
	if [ -z "$release" ]; then
		# herdr's shallow checkout has no tags, so ask origin. `^{}` is the
		# commit an annotated tag points at; a lightweight tag has none.
		release=$(GIT_TERMINAL_PROMPT=0 git ls-remote origin "refs/tags/$tag" "refs/tags/$tag^{}" 2>/dev/null |
			awk '{ sha = $1 } $2 ~ /\^\{\}$/ { peeled = $1 } END { print (peeled != "" ? peeled : sha) }')
	fi
	if [ -z "$release" ]; then
		build_from_source "could not find the $tag tag here or on origin"
	elif [ "$head" != "$release" ]; then
		build_from_source "this checkout is not the $tag release commit"
	fi
	origin=$(git remote get-url origin 2>/dev/null)
fi

if [ -n "${HERDR_DECK_DOWNLOAD_URL:-}" ]; then
	base="${HERDR_DECK_DOWNLOAD_URL%/}/$tag"
else
	repo=$(printf '%s\n' "$origin" | sed -n 's#^.*github\.com[:/]\([^/]*/[^/]*\)$#\1#p' | sed 's#\.git$##')
	base="https://github.com/${repo:-FredricW/herdr-deck}/releases/download/$tag"
fi

command -v curl >/dev/null 2>&1 || build_from_source "curl is not installed"
command -v tar >/dev/null 2>&1 || build_from_source "tar is not installed"
if command -v sha256sum >/dev/null 2>&1; then
	sha256() { sha256sum "$1" | cut -d ' ' -f 1; }
elif command -v shasum >/dev/null 2>&1; then
	sha256() { shasum -a 256 "$1" | cut -d ' ' -f 1; }
else
	build_from_source "neither sha256sum nor shasum is installed to check the download"
fi
fetch() { curl -fsSL --retry 2 --connect-timeout 10 --max-time 120 -o "$2" "$1"; }

mkdir -p bin || build_from_source "could not create bin"
tmp=$(mktemp -d "bin/.download.XXXXXX") || build_from_source "could not create a download folder"
trap 'rm -rf "$tmp"' EXIT

say "downloading $asset"
fetch "$base/SHA256SUMS" "$tmp/SHA256SUMS" ||
	build_from_source "could not download $base/SHA256SUMS"
expected=$(awk -v name="$asset" '$2 == name || $2 == "*" name { print $1; exit }' "$tmp/SHA256SUMS")
[ -n "$expected" ] || build_from_source "the $tag release has no $asset"
fetch "$base/$asset" "$tmp/$asset" ||
	build_from_source "could not download $base/$asset"
actual=$(sha256 "$tmp/$asset")
[ "$actual" = "$expected" ] ||
	build_from_source "$asset does not match its SHA256SUMS entry (got $actual, expected $expected)"

mkdir "$tmp/x" && tar -xzf "$tmp/$asset" -C "$tmp/x" herdr-deck ||
	build_from_source "could not unpack herdr-deck from $asset"
chmod 755 "$tmp/x/herdr-deck"
reported=$("$tmp/x/herdr-deck" --version 2>/dev/null)
case "$reported" in
"herdr-deck $tag" | "herdr-deck $tag "* | "herdr-deck $version" | "herdr-deck $version "*) ;;
*) build_from_source "the downloaded binary did not run or is not $tag (it said: ${reported:-nothing})" ;;
esac

# Rename, never copy over: macOS kills a binary rewritten in place.
mv -f "$tmp/x/herdr-deck" bin/herdr-deck || build_from_source "could not move the binary into bin"
say "installed the prebuilt $asset"
