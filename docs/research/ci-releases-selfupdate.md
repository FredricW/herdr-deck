# CI, releases and self-update — research

Written 2026-10-03 against herdr 0.9.3, herdr-projects 0.2.34 and Go 1.27.1.
Versions and prices below were checked that day; verify them before relying
on them. No code was changed for this document.

It assumes the repository is **public**. Where that matters, a *Private repo*
note says what would change.

**Recommendation in one paragraph.** Add a CI workflow now. Ship the deck
through herdr's own install (`herdr plugin install FredricW/herdr-deck`),
whose `[[build]]` step builds from source with `go build` for as long as the
only users have Go installed. Updating is then `herdr-deck update`, a thin
wrapper over `herdr plugin install … --ref vX.Y.Z --yes`; herdr swaps the
plugin folder atomically. Each running deck notices that the binary at its
path changed and re-executes itself in place with `syscall.Exec`. When people
without Go start using it, add a tag-triggered release workflow and let the
build step download the prebuilt binary (checked against `SHA256SUMS`) with
`go build` as the fallback — the pattern herdr-projects already uses. Do not
add a self-update library: herdr owns the plugin folder, so the deck should
not write binaries into it. Details, sources and effort are in
[§6](#6-comparison-and-recommendation).

Contents:

1. [CI](#1-ci)
2. [Releases](#2-releases)
3. [Self-update libraries](#3-self-update-libraries)
4. [macOS: quarantine, signing, notarisation](#4-macos-quarantine-signing-notarisation)
5. [herdr: how plugins are installed and updated](#5-herdr-how-plugins-are-installed-and-updated)
6. [Comparison and recommendation](#6-comparison-and-recommendation)

---

## 1. CI

A workflow on `pull_request` and on `push` to `main` that runs gofmt,
`go vet`, `go test -race` and golangci-lint.

### Setting up Go 1.27

- `actions/setup-go` (latest v7.0.0) reads the version from
  `go-version-file: go.mod`. It uses the `toolchain` directive when there is
  one, else the `go` directive; a `go` line with a patch (`go 1.27.1`) gets
  exactly that patch, `go 1.27` the newest patch.
  [setup-go README](https://github.com/actions/setup-go#readme),
  [advanced usage](https://github.com/actions/setup-go/blob/main/docs/advanced-usage.md#using-the-go-version-file-input)
- Caching is on by default and covers the module cache (`GOMODCACHE`) and the
  build cache (`GOCACHE`). The default key hashes **`go.mod`, not `go.sum`**;
  set `cache-dependency-path: go.sum` to key on the lock file. The key also
  includes the runner OS and architecture, so each matrix leg has its own
  cache. [setup-go caching](https://github.com/actions/setup-go#caching-dependency-files-and-build-outputs)

### Lint

- `golangci/golangci-lint-action` (latest v9.3.0) needs an explicit
  `actions/setup-go` step before it; v7+ runs golangci-lint v2 only.
  [action README](https://github.com/golangci/golangci-lint-action#compatibility)
- Pin the linter version (`version: v2.14`). Go 1.27 support arrived in
  golangci-lint **v2.13.0**, so anything older fails on this module.
  [golangci-lint changelog](https://github.com/golangci/golangci-lint/blob/main/CHANGELOG.md),
  [CI install docs](https://golangci-lint.run/docs/welcome/install/ci/)
- The action caches only golangci-lint's own analysis cache; Go's caches are
  left to setup-go. [action cache section](https://github.com/golangci/golangci-lint-action#cache)
- Config v2 format: `.golangci.yml` with `version: "2"`, linters under
  `linters:` (`default: standard`), formatters such as `gofmt` under a
  separate `formatters:` section.
  [config file](https://golangci-lint.run/docs/configuration/file/),
  [migration guide](https://golangci-lint.run/docs/product/migration-guide/)
- The Makefile's `lint` target already falls back to `gofmt -l` + `go vet`
  when golangci-lint is missing; keeping explicit gofmt and vet steps in CI
  matches the repo's "before finishing" rule in `CLAUDE.md`.

### `-race`

The race detector needs cgo, and on non-Darwin systems a C compiler; it
supports linux/amd64, linux/arm64, darwin/amd64 and darwin/arm64
([race detector requirements](https://go.dev/doc/articles/race_detector#Requirements)).
Ubuntu runners ship gcc and cgo is on by default, so `go test -race` works on
every runner below. Do not set `CGO_ENABLED=0` for the test job; keep it for
release builds only.

### Runners and cost

| Label | What | Public repo | *Private repo* (per minute) |
|---|---|---|---|
| `ubuntu-latest` (24.04) | Linux x64 | free | $0.006 |
| `ubuntu-24.04-arm` | Linux arm64 | free | $0.005 |
| `macos-latest` (macOS 26, M1) | macOS arm64 | free | $0.062 |
| `macos-15-intel` | macOS x64 | free | $0.062 |

Standard GitHub-hosted runners are "free and unlimited on public
repositories"; larger runners are always billed.
[runner labels](https://docs.github.com/en/actions/reference/runners/github-hosted-runners),
[Actions billing](https://docs.github.com/en/billing/concepts/product-billing/github-actions),
[runner pricing](https://docs.github.com/en/billing/reference/actions-runner-pricing)

*Private repo:* the plan's included minutes apply (2,000/month on Free), then
the rates above, so a macOS leg costs about ten times a Linux one. That is the
main reason private Go projects test on Linux only.

For this repo, test on `ubuntu-latest` and `macos-latest`. The deck has
per-OS code paths (`open` versus `xdg-open`), and its main user is on macOS.
Lint once, on Linux.

### Cancel superseded runs

```yaml
concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: ${{ github.event_name == 'pull_request' }}
```

Each PR gets its own group (`refs/pull/N/merge`); runs on `main` are not
cancelled. [concurrency docs](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/control-workflow-concurrency)

### Sketch: `.github/workflows/ci.yml`

```yaml
name: ci
on:
  pull_request:
  push:
    branches: [main]
permissions:
  contents: read
concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: ${{ github.event_name == 'pull_request' }}
jobs:
  test:
    strategy:
      fail-fast: false
      matrix:
        os: [ubuntu-latest, macos-latest]
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod
          cache-dependency-path: go.sum
      - name: gofmt
        run: test -z "$(gofmt -l .)" || { gofmt -l .; exit 1; }
      - run: go vet ./...
      - run: go test -race ./...
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod
          cache-dependency-path: go.sum
      - uses: golangci/golangci-lint-action@v9
        with:
          version: v2.14
```

with a minimal `.golangci.yml`:

```yaml
version: "2"
linters:
  default: standard
formatters:
  enable: [gofmt]
```

---

## 2. Releases

### Version tags

Use `vX.Y.Z` tags. GoReleaser refuses non-semver tags
([GoReleaser semver](https://goreleaser.com/resources/limitations/semver/)),
and the `v` prefix is what `go install …@vX.Y.Z` and herdr-projects' own
`update` (`parse_release` in its `src/update.rs`) expect.

Keep the version in **one more place than the tag**: `herdr-plugin.toml`'s
`version` field, which herdr shows in `herdr plugin list`. herdr-projects'
release workflow fails when the tag and the manifest disagree, and its install
script uses the manifest version to pick the release to download (see §5).

### Cross-compiling

The deck has no cgo, so `CGO_ENABLED=0 GOOS=… GOARCH=… go build` produces all
four targets from one Ubuntu runner; no macOS runner is needed for release
builds. darwin/arm64 binaries built this way are still ad-hoc signed (§4).

### Writing the version and commit into the binary

Two mechanisms, and they cover different install paths:

| Install path | `-ldflags -X main.version=…` | `debug.ReadBuildInfo()` |
|---|---|---|
| Release build in CI | set | `Main.Version` = tag (needs `fetch-depth: 0`), `vcs.*` present |
| `go install …/cmd/herdr-deck@v1.2.3` | **not set** | `Main.Version` = `v1.2.3`, **no `vcs.*`** (built from the module zip) |
| Local `go build` in a clone | not set | pseudo-version (`v0.0.0-<time>-<sha>`, `+dirty` if modified) or the tag when HEAD is exactly a clean tag; `vcs.revision`, `vcs.time`, `vcs.modified` present |
| `herdr plugin install` build step (shallow clone, see §5) | not set unless the build step passes it | **always a pseudo-version, even with `--ref v1.2.3`**; `vcs.*` present |

Sources: Go 1.24 started stamping the main module's version from VCS tags
([Go 1.24 release notes](https://go.dev/doc/go1.24#go-command)); the `vcs.*`
settings date from Go 1.18
([Go 1.18 notes](https://go.dev/doc/go1.18#go-version),
[debug.BuildSetting](https://pkg.go.dev/runtime/debug#BuildSetting)).
`actions/checkout` fetches one commit and no tags by default
([checkout usage](https://github.com/actions/checkout#usage)).

The last row was verified for this document: a herdr-style checkout
(`git init`, `git fetch --depth 1 origin v0.1.0`, `git checkout --detach
FETCH_HEAD`) has no local tags, so `go version -m` shows
`v0.0.0-20261002212420-7d683b4640ef` and the correct `vcs.revision`.

So: read `-X main.version` first, fall back to `ReadBuildInfo().Main.Version`
and `vcs.revision`, and make the herdr build step pass the manifest's version
with `-ldflags "-X main.version=$version"` so herdr installs report a real
version. The deck has no `--version` flag yet; add one (`herdr-deck 1.2.3
(abc1234)`), since both the install script and the re-exec check below use it.

### GoReleaser versus a plain workflow

**GoReleaser** (OSS, free; latest action v7.2.3, tool v2.18):

- Default ldflags are `-s -w -X main.version={{.Version}} -X
  main.commit={{.Commit}} -X main.date={{.Date}} -X main.builtBy=goreleaser`.
  Default targets include windows and 386, so restrict them.
  [Go builds](https://goreleaser.com/customization/builds/go/)
- Archives default to `tar.gz`; `formats: [binary]` uploads raw binaries.
  [archives](https://goreleaser.com/customization/package/archives/)
- Checksums: `<project>_<version>_checksums.txt`, SHA-256 by default; the name
  is configurable. [checksum](https://goreleaser.com/customization/package/checksum/)
- Changelog: `use: git` (default), `github`, or `github-native` (GitHub's
  generated notes). [changelog](https://goreleaser.com/customization/publish/changelog/)
- `goreleaser check` validates the config, and `release --snapshot --clean`
  is a dry run for PRs. [snapshots](https://goreleaser.com/customization/publish/snapshots/)
- The action needs `permissions: contents: write`, `fetch-depth: 0`, and a Go
  install step. [GitHub Actions](https://goreleaser.com/customization/ci/actions/)
- It also does Homebrew taps, cosign signing and macOS notarisation, none of
  which the deck needs now.

**A plain workflow** is about 30 lines for four raw binaries: a loop over
`GOOS/GOARCH`, `sha256sum > SHA256SUMS`, and
`gh release create "$TAG" --generate-notes --verify-tag dist/*`
([gh release create](https://cli.github.com/manual/gh_release_create),
[generated release notes](https://docs.github.com/en/repositories/releasing-projects-on-github/automatically-generated-release-notes)).
herdr-projects does exactly this for its Rust binary: a matrix that builds,
checks that the binary's `--version` matches the tag, then uploads raw
binaries plus `SHA256SUMS`.

Sketch:

```yaml
name: release
on:
  push:
    tags: ['v*']
permissions:
  contents: write
jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod
          cache-dependency-path: go.sum
      - name: Check the tag matches herdr-plugin.toml
        run: grep -qx "version = \"${GITHUB_REF_NAME#v}\"" herdr-plugin.toml
      - run: go test ./...
      - name: Build
        run: |
          for t in darwin/arm64 darwin/amd64 linux/arm64 linux/amd64; do
            CGO_ENABLED=0 GOOS=${t%/*} GOARCH=${t#*/} go build -trimpath \
              -ldflags "-s -w -X main.version=${GITHUB_REF_NAME#v} -X main.commit=$GITHUB_SHA" \
              -o dist/herdr-deck-${t%/*}-${t#*/} ./cmd/herdr-deck
          done
          (cd dist && sha256sum * > SHA256SUMS)
      - run: gh release create "$GITHUB_REF_NAME" --generate-notes --verify-tag dist/*
        env:
          GH_TOKEN: ${{ github.token }}
```

**Pick the plain workflow.** Asset names stay fixed and versionless, which
keeps the install script simple, and there is no extra tool or config to
learn. Switch to GoReleaser if a Homebrew tap, signing or notarisation becomes
wanted.

### Checksums and provenance

- `SHA256SUMS` from the same release protects against a corrupt download, not
  against a compromised release.
- GitHub artifact attestations (`actions/attest@v4`, needs `id-token: write`
  and `attestations: write`) add signed provenance that users check with
  `gh attestation verify <file> --owner FredricW`. They are free on public
  repos. *Private repo:* they need GitHub Enterprise Cloud.
  [artifact attestations](https://docs.github.com/en/actions/concepts/security/artifact-attestations),
  [gh attestation verify](https://cli.github.com/manual/gh_attestation_verify)
- Keyless cosign over the checksum file is the other common option
  ([GoReleaser cosign](https://goreleaser.com/customization/sign/sign/#signing-with-cosign)).
  Neither is needed for a first release; attestations are a three-line
  addition later.

*Private repo:* release downloads, `go install`, and herdr's own
`git fetch` all need credentials, so every install and update path would need
a token. A public repo needs none of that.

---

## 3. Self-update libraries

All three replace the binary the same way, which is the safe way: write
`.<name>.new` next to the target, rename the target to `.<name>.old`, rename
the new file into place, roll back on failure, then remove `.old`. None does
anything for macOS code signing or quarantine, and none needs to (§4).

| | [creativeprojects/go-selfupdate](https://github.com/creativeprojects/go-selfupdate) | [minio/selfupdate](https://github.com/minio/selfupdate) | [rhysd/go-github-selfupdate](https://github.com/rhysd/go-github-selfupdate) |
|---|---|---|---|
| Status | Maintained. v1.6.0 (2026-07-08), last commit 2026-08-05. Started as a fork of rhysd's. | Low activity. v0.6.0 (2023-01), last commit 2025-10-21. | **Unmaintained.** v1.2.3, last commit 2021-01-13; `go 1.13`, 2016-era deps. |
| License | MIT | Apache-2.0 | MIT |
| Finds releases | Yes: GitHub (+Enterprise), GitLab, Gitea, or an HTTP manifest. Lists releases via REST `GET /repos/{o}/{r}/releases`. | **No.** You pass it an `io.Reader`. | Yes: GitHub only, same REST call. |
| Asset naming | name ends in `{goos}[_-]{goarch}[ext]`; zip, tar.gz, tar.xz, gz, xz, bz2 or raw; overridable with regex filters | n/a | same suffix rule; zip, tar.gz, tar.xz, gz, xz or raw |
| Checksums | `ChecksumValidator{UniqueFilename: "checksums.txt"}` (one sums file) or `SHAValidator` (per-asset `.sha256`); fails closed if the file is missing | `Options.Checksum` — you supply the digest | `SHA2Validator`, per-asset `.sha256` only |
| Signatures | `ECDSAValidator`, `PGPValidator` (deprecated `x/crypto/openpgp`) | minisign `Verifier` | `ECDSAValidator` |
| Apply | rename scheme; resolves symlinks (`EvalSymlinks`) | rename scheme, plus `CheckPermissions()` and two-phase `PrepareAndCheckBinary`/`CommitBinary` | rename scheme via inconshreveable/go-update |
| Token | `GitHubConfig.APIToken`, else `$GITHUB_TOKEN` | n/a | `APIToken`, `$GITHUB_TOKEN`, or gitconfig `github.token` |
| Detect only | `DetectLatest` / `DetectVersion` | n/a | `DetectLatest` / `DetectVersion` |
| REST calls per check / update | 1 / ~3 (asset downloads go through the REST asset endpoint) | 0 | 1 / ~3 |

Sources: each repo's README and source (`github_source.go`, `detect.go`,
`validate.go`, `update/apply.go` in creativeprojects; `apply.go`,
`minisign.go` in minio; `selfupdate/*.go` in rhysd), and
[pkg.go.dev](https://pkg.go.dev/github.com/creativeprojects/go-selfupdate).

**Rate limits.** Unauthenticated REST calls are limited to **60 per hour per
IP** (5,000 with a token)
([REST rate limits](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api)).
With several decks per machine checking on start, a library that calls the
REST API would need its check cached. Two ways around the API entirely:

- `git ls-remote --tags --refs <repo>`, which is what herdr-projects' `update`
  and `doctor` use to find the newest `vX.Y.Z`. It is a git request, not a
  REST call.
- `https://github.com/O/R/releases/latest` (a redirect to the newest tag) and
  `/releases/download/TAG/ASSET` are documented stable URLs
  ([linking to releases](https://docs.github.com/en/repositories/releasing-projects-on-github/linking-to-releases)).
  GitHub's docs do not say whether they are rate-limited; in practice they are
  not counted against the REST limit
  ([opentofu#2802](https://github.com/opentofu/opentofu/issues/2802)), but
  treat that as unverified.

**Verdict.** If the deck ever updates a standalone binary itself,
creativeprojects/go-selfupdate is the only maintained choice that reads a
single checksums file. Its cost is a large dependency tree (go-github, GitLab
and Gitea SDKs, xz, openpgp) for a tool that has six direct dependencies
today. The stdlib alternative is about 100 lines: resolve the latest tag
without the API, download the asset and `SHA256SUMS`, verify, write a temp
file in the target's directory, `fsync`, `chmod 0755`, `os.Rename`. But §5
explains why the deck should not replace its own binary at all when herdr
installed it.

---

## 4. macOS: quarantine, signing, notarisation

**Files written by Go code are not quarantined.** Quarantine is opt-in per
app: an app gets the `com.apple.quarantine` attribute on the files it creates
when its Info.plist sets `LSFileQuarantineEnabled` (or a system exception list
names it). Browsers and mail clients do; non-sandboxed command-line tools such
as `curl`, `git` and `go` do not
([Who decides to quarantine files?](https://eclecticlight.co/2025/12/08/who-decides-to-quarantine-files/),
[Explainer: quarantine](https://eclecticlight.co/2021/12/11/explainer-quarantine/)).
A Go program downloading with `net/http` is in the second group. Sandboxed
apps are the exception, and the deck is not sandboxed.

**Gatekeeper only assesses quarantined items**, so an unquarantined binary
runs. XProtect still scans it on first launch
([Apple Platform Security: Gatekeeper](https://support.apple.com/guide/security/gatekeeper-and-runtime-protection-sec5599b66df/web)).

**Apple Silicon requires a signature, but an ad-hoc one is enough**: "There
isn't a specific identity requirement… a simple ad-hoc signature is
sufficient"
([macOS 11.0.1 universal apps notes](https://developer.apple.com/documentation/macos-release-notes/macos-big-sur-11_0_1-universal-apps-release-notes#Code-Signing)).
Since Go 1.16 the Go linker ad-hoc signs darwin/arm64 binaries itself, in
pure Go, depending on the target and not the host, so binaries cross-compiled
on Linux are signed too ([golang/go#42684](https://github.com/golang/go/issues/42684),
[`cmd/internal/codesign`](https://github.com/golang/go/blob/master/src/cmd/internal/codesign/codesign.go),
`NeedCodeSign` in [`cmd/link/internal/ld/target.go`](https://github.com/golang/go/blob/master/src/cmd/link/internal/ld/target.go)).
Do not post-process the binary (UPX, patching) after linking.

**Replace, never overwrite.** macOS caches a binary's code signature in the
kernel per vnode. Writing new bytes into the existing file leaves a stale
cache, and the process is killed with "Code Signature Invalid". Write a new
file and rename it over the old one, which gives a new inode
([Updating Mac Software](https://developer.apple.com/documentation/security/updating-mac-software);
Apple DTS on [a Go self-updater killed with signal 9](https://developer.apple.com/forums/thread/758098)
and [creating a new file](https://developer.apple.com/forums/thread/669145)).
herdr's plugin install and herdr-projects' install script both follow this
rule (both rename; the script's comment reads "Rename, never copy over: macOS
kills a binary rewritten in place").

**When a browser download is the first install**, the binary is quarantined
and, without notarisation, blocked. Since macOS 15 the Control-click override
is gone; users need System Settings → Privacy & Security → "Open Anyway", or
`xattr -d com.apple.quarantine <file>`
([Apple developer news](https://developer.apple.com/news/?id=saqachfa)).

**Homebrew:** casks must pass Gatekeeper (unsigned casks are disabled in
homebrew/cask since 2026-09-01); formulae, including ones in a personal tap,
are not quarantined and need no notarisation
([Acceptable casks](https://docs.brew.sh/Acceptable-Casks),
[Homebrew/brew#20755](https://github.com/Homebrew/brew/issues/20755)).

**Bottom line.** For install through herdr (git + `go build`, or a script
that downloads with `curl`) and updates fetched by code, **no Developer ID
signing or notarisation is needed**. They become relevant only if users
download binaries with a browser or the deck is shipped as a Homebrew cask.
The Apple Developer Program costs $99/year, and GoReleaser can notarise from
Linux ([GoReleaser notarize](https://goreleaser.com/customization/sign/notarize/)).
A public repo does not change this.

---

## 5. herdr: how plugins are installed and updated

Read from `herdr plugin --help`, `herdr plugin install --help`, the
[plugin docs](https://raw.githubusercontent.com/herdrdev/herdr/v0.9.3/docs/next/website/src/content/docs/plugins.mdx)
(from [herdr.dev/llms.txt](https://herdr.dev/llms.txt)), and herdr's source at
v0.9.3 ([`src/cli/plugin.rs`](https://github.com/herdrdev/herdr/blob/v0.9.3/src/cli/plugin.rs),
[`src/plugin_paths.rs`](https://github.com/herdrdev/herdr/blob/v0.9.3/src/plugin_paths.rs)).

### What `herdr plugin install` does

`herdr plugin install <OWNER/REPO[/SUBDIR]> [--ref <REF>] [-y]`:

1. Creates a temporary folder, `git init`, `git fetch --depth 1 origin <ref or
   HEAD>`, `git checkout --detach FETCH_HEAD`. The checkout is shallow, has no
   branch and no tags.
2. Shows a preview (or skips it with `--yes`; non-interactive installs need
   `--yes`).
3. Runs the manifest's `[[build]]` commands **in that temporary checkout**. A
   failing build aborts the install and the old install stays.
4. Refuses to continue if the build changed `herdr-plugin.toml`.
5. Renames the existing managed folder aside, renames the new checkout to
   `<herdr config dir>/plugins/github/<plugin-id-component>`, registers it, and
   deletes the old folder. The path depends only on the plugin id, so it is the
   same after every reinstall. On failure the old folder is renamed back.

"There is no separate `plugin update` in v1; reinstall from GitHub to refresh
a managed plugin." Reinstalling over a plugin linked with `herdr plugin link`
is refused, and `link` never runs build commands.

### Build from source, or use release binaries?

herdr itself only runs the `[[build]]` argv; it has no notion of releases. So
the plugin decides. herdr's docs say build commands should not assume
toolchains: "Herdr reports build failures but does not install missing
toolchains."

herdr-projects (a public herdr plugin) shows the pattern. Its `[[build]]` is
`sh scripts/install.sh`, which:

- reads `version` from `herdr-plugin.toml` and maps `uname` to an asset name;
- if the checkout has local changes or is not the `v<version>` tag commit
  (checked with `git ls-remote origin refs/tags/v<version>`, since the shallow
  checkout has no tags), builds from source;
- otherwise downloads `SHA256SUMS` and the binary from the GitHub release,
  verifies the hash, runs `--version` and checks it matches;
- moves it into place with `mv` (a rename), and falls back to
  `cargo build --release --locked` on any failure.

So a herdr plugin can use release binaries today, and fall back to building
from source.

### How a self-update interacts with a herdr-managed install

- **herdr owns the folder.** The docs: "GitHub-installed plugin roots are
  managed source checkouts", and durable state belongs in
  `HERDR_PLUGIN_STATE_DIR`/`HERDR_PLUGIN_CONFIG_DIR`. A binary the deck writes
  into its plugin root is thrown away by the next `herdr plugin install`, and
  `herdr plugin list` would still report the old `resolved_commit` and version.
- **`git pull` in the managed folder does not work:** it is a detached, shallow
  checkout made by herdr.
- **So update through herdr.** herdr-projects' `update` command reads
  `herdr plugin list --plugin <id> --json`. For a `github` source it runs
  `herdr plugin install <owner/repo> --ref v<newest> --yes`. For a `local`
  (linked) source it runs `git pull --ff-only origin main` and the install
  script. It finds the newest release with `git ls-remote --tags`. The deck
  can copy this almost line for line.

### Many deck instances, one new binary

There is one deck per coordinator, all started from the same
`<plugin root>/bin/herdr-deck`. After a reinstall, that path names the new
binary, and every running deck is still the old one (an unlinked inode, which
keeps running).

Proposal: each deck restarts itself in place.

1. **At start**, record the absolute path of the binary and its file identity
   (`os.Stat`). Take the path from `os.Executable()` at start, or from
   `$HERDR_PLUGIN_ROOT/bin/herdr-deck`. Asking later is wrong on Linux:
   `os.Executable` reads `/proc/self/exe`, which after herdr's folder swap
   points into the deleted old folder (Go strips the ` (deleted)` suffix in
   [`os/executable_procfs.go`](https://github.com/golang/go/blob/master/src/os/executable_procfs.go)).
   On macOS it returns the path the process was started with
   ([`os/executable_darwin.go`](https://github.com/golang/go/blob/master/src/os/executable_darwin.go)).
2. **On the existing 5 s tick** (or an fsnotify watch on the plugin folder's
   parent), `os.Stat` the path again. If `!os.SameFile(old, new)` and
   `path --version` runs and prints a version, it is time to restart.
3. **Restart**: let Bubble Tea quit so it restores the terminal (`tea.Quit`;
   `Program.ReleaseTerminal` also exists), then
   `syscall.Exec(path, os.Args, os.Environ())`. `execve` keeps the process id,
   so the herdr pane, its id and its metadata stay; the new deck redraws with
   the same arguments and environment. If `Exec` fails, keep running the old
   one and show a hint in the header.
4. **Working directory:** the deck's pane cwd is the project folder
   (`~/.herdr-projects/<slug>`), not the plugin root, so the folder swap
   doesn't strand it. If a future pane command runs with the plugin root as
   cwd, `chdir` to the recorded root before `Exec`.

The deck keeps no state worth saving across a restart beyond the cursor and
folds; losing them on an update is acceptable, or they can be passed through
an environment variable.

Nothing here needs herdr's help, and it works the same for linked installs:
`go build -o bin/herdr-deck.tmp && mv bin/herdr-deck.tmp bin/herdr-deck`
replaces the file, and every deck restarts. Build to a temporary name and
rename. `go build -o` straight onto the path does give the file a new inode:
it renames, or removes the old file and then copies, so it is safe for §4. But
it is not atomic. During the copy, a deck that checks at that moment sees a
new but half-written file
([`cmd/go/internal/work/shell.go`](https://github.com/golang/go/blob/master/src/cmd/go/internal/work/shell.go),
`moveOrCopyFile` and `CopyFile`). The `--version` check in step 2 catches
that too.

---

## 6. Comparison and recommendation

### The options

**A. No releases: herdr builds from source.** `[[build]]` is
`go build -ldflags "-X main.version=<manifest version>" -o bin/herdr-deck
./cmd/herdr-deck`, run by herdr in its temporary checkout. Updating is
`herdr-deck update` → `herdr plugin install FredricW/herdr-deck --ref vX.Y.Z
--yes`. For a linked checkout (development), `git pull --ff-only`, then build
to a temp file and rename. Running decks re-exec as in §5.

- Needs Go 1.27 on every machine that installs the plugin.
- Release tags are still needed (for `--ref` and "a newer version exists"),
  but no release workflow or assets.
- A build takes a few seconds; the dependencies are downloaded once per
  machine into the module cache.

**B. Releases plus herdr install.** As A, but `[[build]]` is a script that
downloads the prebuilt binary for the manifest version, checks `SHA256SUMS`,
and falls back to `go build`, as herdr-projects does. Users without Go can
install, and releases carry checksums (and later attestations).

**C. A self-update library or hand-written updater.** The binary downloads
the new release and renames it over itself. This fits a standalone install
(`curl | sh` into `~/.local/bin`). It does not fit a herdr-managed install: it
writes into a folder herdr owns, the next reinstall undoes it, and
`herdr plugin list` reports a stale version. Two update paths would also
double the testing.

### Recommendation

**A now, B when someone without Go installs the deck; not C.** The update
command, the restart-in-place and CI are the same under A and B. B only adds
the release workflow and a smarter build script, so moving from A to B later
is cheap. Add CI right away regardless.

| Part | Option | Rough effort |
|---|---|---|
| CI workflow + `.golangci.yml` (fix whatever the linter finds) | A, B | 1–2 h |
| `--version` (ldflags, falling back to `debug.ReadBuildInfo`) | A, B | 1 h |
| `[[build]]` passing the manifest version (part of milestone 7) | A | 0.5 h |
| `herdr-deck update [--check]`: read `herdr plugin list --json`, `git ls-remote --tags`, run `herdr plugin install --ref` or pull + build | A, B | 3–4 h with tests |
| Restart in place: record path and identity, check on tick, quit, `syscall.Exec` | A, B | 2–3 h with tests |
| "Update available" hint in the header (cached `ls-remote`, at most hourly) | A, B, optional | 1–2 h |
| Release workflow (plain, four targets, `SHA256SUMS`, tag/manifest check) | B | 2 h |
| `scripts/install.sh`: download, verify, `--version` check, rename, `go build` fallback | B | 2–3 h |
| Artifact attestations | B, optional | 0.5 h |
| Standalone self-updater (library or stdlib) | C | 4–6 h, plus Homebrew/permission edge cases |

### Decisions for the user

1. **A or B for v1:** is "Go 1.27 must be installed" acceptable for now?
   (Recommended: yes, A, until someone else installs it.)
2. **Who triggers an update:** only an explicit `herdr-deck update`, or should
   the deck also show "update available"? Should it never update on its own?
   (Recommended: explicit command plus a hint; no automatic updates.)
3. **Restart running decks automatically when the binary changes**, or only
   show "restart to update"? (Recommended: automatic, as it keeps all decks on
   one version and costs nothing visible.)
4. **CI matrix:** Linux only, or Linux + macOS? Both are free on a public
   repo. (Recommended: both.)
5. **Release mechanics, when B comes:** plain workflow or GoReleaser;
   attestations yes/no. (Recommended: plain, attestations later.)
6. **Version source of truth:** the tag plus `herdr-plugin.toml`'s `version`,
   with CI failing when they disagree. (Recommended.)
7. **Standalone installs** outside herdr (`go install`, `curl | sh`): support
   them at all in v1? (Recommended: `go install` works for free; skip a
   standalone updater.)

### Where the public-repo assumption matters

- **CI minutes:** free and unlimited on public repos, including macOS and
  arm64 runners. A private repo pays roughly ten times more for macOS legs.
- **Installs and updates:** herdr's `git fetch`, `git ls-remote`, release
  downloads and `go install` all work without credentials only on a public
  repo.
- **Rate limits:** apply the same either way, but a private repo would force
  every path through authenticated API calls.
- **Artifact attestations:** free on public repos; a private repo needs
  GitHub Enterprise Cloud.
- **Signing and notarisation:** unaffected.
