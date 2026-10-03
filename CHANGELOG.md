# Changelog

What changed in each herdr-deck release, newest first. The deck shows this
file too: press `w` for What's new.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and versions follow [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- `p` opens a project picker over the deck: every project under the
  projects root, with the threads waiting on you in each and how long they
  have waited. Inbox items are progress updates, not needs: each project
  shows them as a dim `✉ N updates`. Projects with a waiting thread come
  first; paused ones are dim and archived ones show after `tab`. Type to
  filter. `↵` on a project focuses its coordinator, or starts one with
  `herdr-projects open` when none runs; on a waiting thread, it focuses that
  thread's pane, in whichever workspace it is.
- When threads in other projects wait on you, the list ends with a line
  such as `● 2 other projects need you`; click it or press `p` to see them.
  Their threads never join this project's Needs you group.
- The drawer has a header card (the title, the status as a glyph and a
  word, the percent with a progress bar) and three tabs: Overview
  (Next, PR with commenters and failing checks, Note, Links as numbered
  chips, Dev, Thread), Files (the changed files as a diffstat with a
  bar per file, every file listed) and Log (the thread's timeline from
  its thread file and its inbox items, handled ones included). The
  active tab is dark text on blue. `[` and `]` switch tabs; `tab` moves
  the focus into the drawer, where `j`/`k` and `↵` pick a chip, file or
  event. The drawer takes half of the pane.

### Changed

- Inbox items no longer get rows under Needs you, which now holds only
  threads waiting on you. A thread with unhandled items has a yellow title
  and shows them, marked `✉`, in its Log; items about no thread only count
  in the header.
- `d` shows the Files tab instead of a chooser; `d 3` still opens file 3
  and `d d` the whole diff, and `t` on the tab switches list and tree.

## [0.1.2] - 2026-10-03

### Changed

- The Files section colours each file by how it changed: a status letter
  `A` added (green), `M` modified (yellow), `D` deleted (red), `R` renamed
  (cyan) and `?` untracked (faint green). Line counts are green and red;
  an added file shows only `+N` and a deleted one only `-M`.
- `d t` switches the Files section between the list and a folder tree, with
  folder names and their summed counts muted so the files stand out.
  `diff_view = "tree"` in the config file (or the settings page) starts in
  the tree.

### Fixed

- The settings page scrolls to keep an invalid value's error in view.

## [0.1.1] - 2026-10-03

### Added

- A Files section in the drawer lists the files the selected thread changed,
  with `+N -M` counts. `d` opens the diff tool on one file or on the whole
  diff.
- A settings page (`s`) shows every setting, its value and where it comes
  from, and edits the config file in place. Most changes apply at once.
- What's new (`w`) shows this changelog in the deck. After an update the
  deck says once which version it now runs, and while `↑` shows, it lists
  what the newer version brings.
- Releases carry prebuilt binaries for macOS and Linux (arm64 and amd64)
  with a `SHA256SUMS` file. Installing a release through herdr downloads
  the binary instead of building it, so Go is only needed for other commits.
- The README shows the deck in short demo GIFs.

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
