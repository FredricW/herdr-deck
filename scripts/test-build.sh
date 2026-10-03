#!/bin/sh
# Tests scripts/build.sh against a fake release served from file:// and a
# fake `go`, in a checkout made the way herdr makes one (shallow, detached,
# no tags). Needs git, curl, tar and sha256sum or shasum; not Go.
#
# Usage: scripts/test-build.sh
set -eu

repo=$(cd "$(dirname "$0")/.." && pwd)
work=$(mktemp -d "${TMPDIR:-/tmp}/herdr-deck-build-test.XXXXXX")
trap 'rm -rf "$work"' EXIT
failures=0

# Only these tools are on PATH, so `go` is the fake one or missing.
tools="$work/tools"
mkdir -p "$tools"
for t in sh git curl tar gzip awk sed cut mktemp rm mv chmod mkdir dirname uname cat grep sha256sum shasum perl; do
	p=$(command -v "$t" 2>/dev/null) && ln -s "$p" "$tools/$t"
done
gobin="$work/go"
mkdir -p "$gobin"
# The fake go writes a binary that reports the version it was stamped with.
cat >"$gobin/go" <<'EOF'
#!/bin/sh
stamp= out=
while [ $# -gt 0 ]; do
	case $1 in
	-ldflags) stamp=${2#-X main.version=}; shift ;;
	-o) out=$2; shift ;;
	esac
	shift
done
printf '#!/bin/sh\necho "herdr-deck %s (source)"\n' "$stamp" >"$out"
EOF
chmod 755 "$gobin/go"

case "$(uname -s)" in Darwin) os=darwin ;; *) os=linux ;; esac
case "$(uname -m)" in arm64 | aarch64) arch=arm64 ;; *) arch=amd64 ;; esac
asset="herdr-deck_9.9.9_${os}_${arch}.tar.gz"

sha256() {
	if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | cut -d ' ' -f 1; else shasum -a 256 "$1" | cut -d ' ' -f 1; fi
}

# origin: a repository with the script, a manifest at 9.9.9 and its tag.
git init -q "$work/origin"
mkdir -p "$work/origin/scripts"
cp "$repo/scripts/build.sh" "$work/origin/scripts/"
printf 'id = "herdr-deck"\nversion = "9.9.9"\n\n[[build]]\ncommand = ["sh", "scripts/build.sh"]\n' >"$work/origin/herdr-plugin.toml"
printf 'bin/\n' >"$work/origin/.gitignore"
git -C "$work/origin" add -A
git -C "$work/origin" -c user.name=test -c user.email=test@example.com commit -qm release
git -C "$work/origin" tag v9.9.9
git -C "$work/origin" -c user.name=test -c user.email=test@example.com commit -q --allow-empty -m later

# release: what the release workflow publishes, for `9.9.9`.
make_release() { # make_release <dir> <version the binary reports>
	mkdir -p "$1/v9.9.9" "$work/pkg"
	printf '#!/bin/sh\necho "herdr-deck %s (abc1234def56)"\n' "$2" >"$work/pkg/herdr-deck"
	chmod 755 "$work/pkg/herdr-deck"
	tar -czf "$1/v9.9.9/$asset" -C "$work/pkg" herdr-deck
	(cd "$1/v9.9.9" && printf '%s  %s\n' "$(sha256 "$asset")" "$asset" >SHA256SUMS)
}

# clone: a fresh herdr-style checkout of <ref>.
clone() {
	rm -rf "$work/clone"
	git init -q "$work/clone"
	git -C "$work/clone" remote add origin "$work/origin"
	git -C "$work/clone" fetch -q --depth 1 origin "$1"
	git -C "$work/clone" checkout -q --detach FETCH_HEAD
}

# check <name> <expected bin/herdr-deck --version, or "fails"> [env…]
check() {
	name=$1 want=$2
	shift 2
	if env PATH="$gobin:$tools" "$@" sh "$work/clone/scripts/build.sh" >"$work/log" 2>&1; then
		got=$("$work/clone/bin/herdr-deck" --version 2>&1) || got="no binary"
	else
		got=fails
	fi
	if [ "$got" = "$want" ]; then
		echo "ok   $name"
	else
		echo "FAIL $name: want \"$want\", got \"$got\""
		sed 's/^/     /' "$work/log"
		failures=$((failures + 1))
	fi
	rm -rf "$work/clone/bin"
}

prebuilt="herdr-deck v9.9.9 (abc1234def56)"
source="herdr-deck v9.9.9 (source)"
url="file://$work/release"
make_release "$work/release" v9.9.9

clone v9.9.9
check "release checkout downloads the prebuilt binary" "$prebuilt" HERDR_DECK_DOWNLOAD_URL="$url"
check "HERDR_DECK_BUILD=source builds from source" "$source" HERDR_DECK_DOWNLOAD_URL="$url" HERDR_DECK_BUILD=source
check "offline falls back to source" "$source" HERDR_DECK_DOWNLOAD_URL=http://127.0.0.1:9
check "no release builds from source" "$source" HERDR_DECK_DOWNLOAD_URL="file://$work/nothing"
check "without go and offline it fails" fails HERDR_DECK_DOWNLOAD_URL=http://127.0.0.1:9 PATH="$tools"

cp -R "$work/release" "$work/bad"
printf '%s  %s\n' 0000000000000000000000000000000000000000000000000000000000000000 "$asset" >"$work/bad/v9.9.9/SHA256SUMS"
check "checksum mismatch falls back to source" "$source" HERDR_DECK_DOWNLOAD_URL="file://$work/bad"

printf '%s  herdr-deck_9.9.9_plan9_mips.tar.gz\n' 0000000000000000000000000000000000000000000000000000000000000000 >"$work/bad/v9.9.9/SHA256SUMS"
check "asset missing from SHA256SUMS falls back to source" "$source" HERDR_DECK_DOWNLOAD_URL="file://$work/bad"

cp "$work/release/v9.9.9/SHA256SUMS" "$work/bad/v9.9.9/SHA256SUMS"
rm "$work/bad/v9.9.9/$asset"
check "asset missing from the release falls back to source" "$source" HERDR_DECK_DOWNLOAD_URL="file://$work/bad"

rm -rf "$work/bad"
make_release "$work/bad" v1.0.0
check "binary of another version falls back to source" "$source" HERDR_DECK_DOWNLOAD_URL="file://$work/bad"

echo "# local change" >>"$work/clone/herdr-plugin.toml"
check "uncommitted changes build from source" "$source" HERDR_DECK_DOWNLOAD_URL="$url"

clone HEAD
check "a commit after the tag builds from source" "$source" HERDR_DECK_DOWNLOAD_URL="$url"

rm -rf "$work/clone"
git clone -q "$work/origin" "$work/clone"
git -C "$work/clone" checkout -q -B main v9.9.9
check "a branch checkout builds from source, stamped by git describe" "herdr-deck v9.9.9 (source)" HERDR_DECK_DOWNLOAD_URL="$url"
git -C "$work/clone" checkout -q -B main origin/HEAD
check "a dev checkout past the tag stamps git describe" "herdr-deck v9.9.9-1-g$(git -C "$work/origin" rev-parse --short=7 HEAD) (source)" HERDR_DECK_DOWNLOAD_URL="$url"

if [ "$failures" -gt 0 ]; then
	echo "$failures failed"
	exit 1
fi
echo "all passed"
