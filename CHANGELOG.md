# Changelog

What changed in each herdr-deck release, newest first. The deck shows this
file too: press `w` for What's new.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and versions follow [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- A split layout for the diff preview: `S` (or `|`) shows the old file on
  the left and the new one on the right, with line numbers on each side
  and removed and added lines paired up, and switches back to unified.
  `diff.layout = "split"` (or the settings page, `--diff-layout`,
  `$HERDR_DECK_DIFF_LAYOUT`) starts with it. A pane under 100 columns shows
  unified and says so in the preview's header.
- Linear issues in the drawer show their title and assignee, one line
  each: `[1 ABC-123 Fix login · in progress · ana]`, shortened to fit a
  narrow drawer. With a Linear API key bare IDs open Linear's own URL, so
  `linear.workspace` is no longer needed; the open-link action asks Linear
  for the workspace too.

## [0.1.4] - 2026-10-04

### Added

- A Commits tab in the drawer, between Files and Log: the thread branch's
  commits since its base, newest first, with short sha, subject, age and
  `+N −M`, under a `● uncommitted · N files` row that leads to Files.
  `↵`, a digit or a click previews a commit as `git show` does, with a
  syntax-coloured diff. `space`, `→` or `l` in the focused drawer (or a
  click on `▸`) expands a commit to the files it changed, and `↵` on one
  previews that file's change; `←` or `h` collapses it. `g` opens the
  commit in the PR on GitHub.
- `ui.folded_lists` in the config file (also on the settings page, as
  `--ui-folded-lists` and `$HERDR_DECK_UI_FOLDED_LISTS`) names the lists
  that start folded, by heading in any case: `["Backlog", "Resolved"]` by
  default, `[]` for none.

### Changed

- The Files tab previews by default, as Commits does: `↵`, a digit or a
  click shows a file's diff in the deck instead of opening the diff tool.
  The diff tool is now `d` in the focused Files or Commits tab (the file
  or commit under the cursor, so `d d` from the list opens the first file
  there), and `a` opens the whole diff in it.
- A list heading now has to match a folded list's name whole, so a list
  such as "Backlog later" no longer starts folded.
- Settings now live in one table per area of the config file (`[ui]`,
  `[projects]`, `[linear]`, `[figma]`, `[browser]`, `[updates]`,
  `[editor]`, `[diff]`), and each is named by its dotted path, such as
  `linear.workspace`. That name gives its flag and environment variable
  (`--diff-command`, `$HERDR_DECK_DIFF_COMMAND`), and every setting now
  has a flag. The earlier keys, flags (`--diff-tool`, `--editor`, …) and
  environment variables still work, and the `!` view notes each earlier
  key in the file. Run `herdr-deck config migrate` to see the move and
  `herdr-deck config migrate --write` to make it (the old file is kept as
  `config.toml.bak`); saving on the settings page also moves a setting
  into its table, comments and all.
- A stray argument, such as a mistyped command, is now an error instead
  of being ignored.

## [0.1.3] - 2026-10-03

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
- A diff preview: on the Files tab, `v` shows the file under the cursor's
  diff where the task list is, while `j`/`k` in the drawer move between
  files. The diff is against the merge-base, uncommitted changes and
  untracked files included, with its code coloured by the file's language,
  a green `+` and red `−` gutter on tinted lines and dim hunk headers.
  `J`/`K` scroll it a line, `pgup`/`pgdn` a page, the wheel three lines;
  `v` or `esc` brings the list back as you left it.

### Changed

- Inbox items no longer get rows under Needs you, which now holds only
  threads waiting on you. A thread with unhandled items has a yellow title
  and shows them, marked `✉`, in its Log; items about no thread only count
  in the header.
- `d` focuses the Files tab instead of opening a chooser; `d 3` still
  opens file 3 and `d d` the whole diff, and `t` on the tab switches list
  and tree. With one changed file, `d` no longer opens it at once; use
  `d 1`.
- A thread's report (`r`), What's new (`w`) and a task's notes in the
  Overview render their Markdown the way glow does: headings, bold, code,
  lists, quotes and highlighted code blocks, wrapped to the pane and styled
  for a dark or a light terminal.

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
