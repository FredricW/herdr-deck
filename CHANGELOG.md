# Changelog

What changed in each herdr-deck release, newest first. The deck shows this
file too: press `w` for What's new.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and versions follow [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- A Files section in the drawer lists the files the selected thread changed,
  with `+N -M` counts. `d` opens the diff tool on one file or the whole diff.
- A settings page (`s`) shows every setting, its value and where it comes
  from, and edits the config file in place. Most changes apply at once.
- What's new (`w`) shows this changelog in the deck. After an update the
  deck says once which version it now runs, and while `↑` shows it also
  lists what the newer version brings.
- Releases carry prebuilt binaries for macOS and Linux (arm64 and amd64),
  with `SHA256SUMS`. Installing a release through herdr downloads the
  binary instead of building it, so Go is only needed for other commits.

### Changed

- Release notes on GitHub come from this changelog.

## [0.1.0] - 2026-10-03

The first release.

### Added

- A status pane for a herdr-projects project: its tasks and threads in one
  list, with what needs you pinned on top, and a drawer with details about
  the selected row. Long lists such as Backlog fold.
- Live status from herdr-projects and herdr: each thread's state and
  activity, its pane and agent, the inbox, and pull requests with their
  review state, comments and failing checks.
- Numbered links in the drawer for Linear, Figma, Notion, GitHub and
  localhost. A digit opens one; `l f n g o` open the first of a kind.
- Linear issue status next to each issue ID, when a Linear API key is set.
- Dev servers from each worktree's dev manifest, with a dot per port.
  `u` starts a thread's dev servers.
- Keys to focus a thread's herdr pane (`↵`), open its worktree in the
  editor (`e`), read its report (`r`), resize the drawer (`z`) and see
  sources that could not be read (`!`).
- Web links open in a browser tab that already shows them, on macOS.
- A herdr plugin: a deck opens next to each coordinator by itself, a key
  toggles it anywhere, and Figma links and selected Linear IDs open from
  any herdr pane.
- `herdr-deck update` updates the plugin. The header shows `↑` when a newer
  version exists, and running decks restart in place on the new binary.
- A config file, `~/.config/herdr-deck/config.toml`, for the Linear
  workspace, editor, refresh interval, update check and more.
