# herdr-deck — plan

A standalone Go + Charm TUI that runs in a split next to each herdr-projects
coordinator and shows, always up to date, what that project is doing: its
tasks, its threads and their status, links to Linear / Notion / Figma / GitHub,
and the dev servers its worktrees run.

Written 2026-10-02 from an investigation of herdr 0.9.3 and herdr-projects
0.2.34. Paths and line numbers below were true then; verify before relying on
them.

## Principles

- **Read-only towards herdr-projects.** Never fork it, never write its files.
  It is Elias Stravik's plugin (MIT) and releases often; we read its files and
  its one JSON command. The only writes we make are herdr pane/workspace
  metadata under our own `--source herdr-deck`, and opening URLs.
- **Standalone binary first, herdr plugin wrapper second.** `herdr-deck` must
  run in any terminal (`herdr-deck --project admin-rebuild`). The herdr plugin
  is a thin `herdr-plugin.toml` that opens it in a split.
- **Degrade, don't fail.** Any source can be missing (no ticker, no herdr
  socket, no manifest, odd TASKS.md). Show what is there, mark what is not.
- **Data layer separate from UI.** `internal/source/*` produces plain structs
  (`deck.Snapshot` and friends in `internal/deck`); `internal/ui` only renders
  them. Data code is unit-tested against fixture
  files in the shapes herdr-projects writes.

## Stack

- Go 1.27 or newer, module `github.com/FredricW/herdr-deck`.
- Bubble Tea v2 (model/update/view), Bubbles (list, table, viewport, help,
  key), Lip Gloss (layout/styling), imported from `charm.land/bubbletea/v2`,
  `charm.land/bubbles/v2` and `charm.land/lipgloss/v2`.
- bubblezone for clickable rows, only if Bubble Tea v2's own mouse handling
  isn't enough.
- `BurntSushi/toml` (or `pelletier/go-toml/v2`) for herdr-projects' TOML.
- No CGO. `go build` produces one binary.

## Data sources

All under the projects root (`$HERDR_PROJECTS_ROOT`, else
`~/.herdr-projects`), project folder `<root>/<slug>/`.

| What | Where | Notes |
|---|---|---|
| Project settings | `<slug>/PROJECT.md` | `+++` TOML front matter: `name`, `goal`, `[[repos]] path`. |
| Threads | `herdr-projects --root <root> thread list <slug> --json` | The only JSON output. Each row has every TOML field plus `group`, `rank`, `next[]`, `report`. Fallback: read `<slug>/threads/t-*.toml` directly. |
| Thread status | fields `last_group`, `state_line` (e.g. `needs you · ~95%`), `activity`, `percent`, `status` | Updated by the herdr-projects ticker every ~15 s. |
| PRs | thread `pr`, `pr_state`, `pr_review`; `<slug>/.state/ticker.json` → `prs[thread id]` | `prs` holds `state`, `review_decision`, `failing_checks[]`, `comment_count`, `commenters[]`. Refreshed every 2 min. |
| Tasks | `<slug>/TASKS.md` | Written freely by the coordinator agent. Expected format is `## <List>` headings and `- [ ] Title (owner) · t-0007` lines with notes indented two spaces (herdr-projects `skill/COORDINATOR.md:78-81`, parser `src/tasks.rs:126`), but real files often use plain `- ` bullets with "thread t-0002" in prose. Parse loosely; see below. |
| Inbox | `<slug>/inbox/*.md` (not `done/`) | `+++` TOML front matter: `id`, `kind`, `subject`, `created`, `summary`. Count + list of unhandled items. |
| Thread brief / report | `<slug>/threads/t-NNNN.task.md`, `t-NNNN.md` | Free text. Scrape for links. Report may start with `PR: <url>` and has a `## Next` section. |
| herdr live state | `session.snapshot` over the socket (the same JSON as `herdr api snapshot`) on every reload | Workspaces, panes, agents, tokens (`hp_project`, `hp_sub`, `hp_group`, `port`). A thread's pane has `hp_project` = slug and `hp_group` = `<slug>!1!<rank>!<thread id>`; the coordinator's `hp_group` is `<slug>!0!<pane id>`. Without tokens, the recorded `pane_id` counts when its `cwd` is the thread's worktree. |
| herdr events | Unix socket `$HERDR_SOCKET_PATH` (else `~/.config/herdr/herdr.sock`), newline-delimited JSON `{id, method, params}`; `events.subscribe {subscriptions:[{type:"pane.agent_status_changed"}, …]}` | Verified in milestone 5 (herdr 0.9.3): after `subscription_started` the connection stays open and streams `{"event","data"}` lines (event names use `_`: `pane_updated`). `pane.updated` fires for every pane each time the herdr-projects ticker rewrites tokens (~15 s) and carries the whole pane. `pane.agent_status_changed` needs a `pane_id`, so the deck subscribes once per agent pane, and again when panes come or go. One unknown pane id fails the whole subscribe and closes the connection. When the subscribe is refused the deck polls every 2.5 s. Reference round-trip: herdr-projects `src/runner.rs:279-291`. Full method list: `herdr api schema --json`. |
| Dev servers | herdr-deck's own manifest, `.herdr-deck/dev.json` in the worktree, else in the repo's main checkout (the thread's `repo`) | Tells the deck where a worktree's dev-server ports live: `state.file` is a path template per worktree (e.g. `.dev/$DIRNAME/state.json`; also `$BRANCH`, `$WORKTREE`, `$REPO`; relative to the main checkout), `state.ports` maps server names → JSON keys in that file (e.g. `api_port`, `frontend_port`) or fixed port numbers, and `links[]` are URL templates with `$PORT_<name>` placeholders and the servers they `need`. "Running" = TCP connect to 127.0.0.1:port or [::1]:port (Node dev servers may listen on IPv6 only), 400 ms timeout, probed in parallel, answers cached 2 s. Format in the README. A repo without a manifest is a note in Sources (not a missing source); a broken manifest or state file is missing. |
| Fallback port | herdr workspace token `port` on the workspace whose `worktree.checkout_path` is the thread's worktree (set by a worktree plugin, e.g. from a hash of the branch) | Use only when no manifest port is found; marked `~`. |

**Never** call `herdr-projects context <slug>` without `--peek`: without it
the command marks inbox items as seen.

### Loose task parsing

Accept both shapes. For each list item under a `##` heading:

- title = the line text minus checkbox, owner and thread suffix;
- done = `[x]`; list = the heading;
- thread ids = the threads the task belongs to: the ids in a strict
  `· t-0007` suffix, else every `t-\d{4}` on the bullet line, plus the
  suffix ids of sub-items in its notes (`  - [ ] Part · t-0005`). Other ids
  in the indented notes only mention another thread ("after t-0002 lands")
  and do not link it, or that thread's links would show on this task;
- links (from the line, its notes, and the task's threads' `.task.md`/`.md`,
  each file scraped on its own):
  - Linear: `\b[A-Z][A-Z0-9]{1,5}-\d+\b` → `https://linear.app/<workspace>/issue/<ID>`.
    There is no default workspace: it comes from `--linear-workspace` or
    `$HERDR_DECK_LINEAR_WORKSPACE`. Without one, IDs still show but say a
    workspace must be set instead of opening. A branch name gives a ticket
    too (`dev/abc-123-users` → ABC-123). Word boundaries on both sides, so
    ABC-110 ≠ ABC-1100 and P-ABC-49 is not ABC-49. Bare IDs inside code
    (backtick spans, fenced blocks) and in a `## Remember` section are
    examples or lessons, so they do not link; a full Linear URL there still
    does.
  - Figma: `figma.com/(design|file|proto)/…` URLs; label them with the file
    name and `node-id`, and offer the `figma://` desktop rewrite.
  - Notion: `notion.so/…` URLs.
  - GitHub: PR/issue URLs, plus the thread's `pr` field.

Paragraph text that is not a list item is shown as the list's note, not as a
task.

## UI

One project per instance, narrow by default (~60–80 columns, full height).
A compact list on top and a detail drawer below; mockups of every state at
60 and 80 columns are in [docs/design/d-list-detail.md](design/d-list-detail.md)
(direction D, chosen 2026-10-02; the alternatives are in
[docs/design/](design/README.md)).

```
 Admin rebuild                           ◐ 2  ◇ 1  ○ 1  ✉ 0
────────────────────────────────────────────────────────────
    WORK                       STATUS           LINKS  DEV
  In progress
 ▸◐ Users page                 Building overv…  L4
  ◐ Templates page             Writing tests    L F    ●●●
  ◇ Document select for sum…   #2320 review     L F3   ●○

  + Backlog (6)
─ Users page · t-0002 ──────────────────────────────────────
 Status  working · Building overview · ~60% · 4m
 Note    Phase 1 = layout + overview (ABC-1256); report …
 Linear  1 ABC-1246  2 ABC-1256  3 ABC-1257  4 ABC-1250
────────────────────────────────────────────────────────────
 1-9 link  l f n g o first  ↵ pane  z drawer  ? help
```

- **List.** One row per piece of work: a task joined with the threads it
  names, or a thread no task names. Rows are grouped under their TASKS.md
  list, with a blank line between two groups (folded ones too); the cursor
  and the mouse skip it. Columns at 80: work, thread, status, PR, links (`L4` = four Linear
  links), dev (one dot per dev.json port). At 60 the thread and PR columns
  fold into status.
- **Needs you on top.** Threads waiting on you and unhandled inbox items move
  into a red *Needs you* group at the top of the list. A thread is waiting on
  you when herdr-projects says so, or when its agent has been `blocked` on a
  question or permission prompt for 30 s (milestone 5); an agent that is
  working, idle, done or waiting on its own sub-agents never counts. If the cursor has not
  been moved by hand, it jumps there so the drawer shows the thread's
  `next[]`.
- **Folding.** `space` on a list heading folds or unfolds the list. A folded
  heading starts with `+` and shows its count (`+ Backlog (6)`); open
  headings have no marker. `▸` only ever marks the selected row. *Backlog*
  starts folded, and a folded list still shows its threads' glyphs.
  Folding never hides a need, because those rows are pinned on top.
- **Drawer.** It shows the selected row: status, PR (review, comments,
  checks), `next[]`, notes, branch, servers, and every link, numbered `1`–`9`
  in order Linear, Figma, Notion, GitHub, localhost. The numbered links
  replace a separate chooser. With no row selected (or in an empty project)
  the drawer shows the project's goal and repos. `z` cycles the drawer
  through ~40 % height, full height and hidden.
- **Files.** The drawer's last section lists the selected thread's
  changed files (`internal/source/diff`): its worktree against the
  merge-base with the thread's `base` (else `origin/HEAD`), `git diff
  --numstat -z -M <merge-base>` plus untracked files from `git ls-files
  --others --exclude-standard`, each with dim `+N -M` counts (`binary`,
  `untracked`, `old → new`) under a total line. Nine files, numbered, then
  `+K more`. Git runs off the UI goroutine (5 s timeout,
  `GIT_OPTIONAL_LOCKS=0`) when the selection moves to another thread and on
  every reload; answers are reused for 2 s. No section for a resolved
  thread; a missing worktree or base is a dim note.
- **Missing sources.** `! N` in the header (yellow), and `!` shows a
  *Sources* view in the drawer. Stale rows are dim with `as of HH:MM`, and
  fallback ports are marked `~`.
- **Threads no task names** (built in milestone 4) get their own rows under
  *Other threads* after TASKS.md's lists; resolved ones go to a *Resolved*
  list that starts folded like *Backlog*. A task naming several threads is
  one row showing the most pressing thread; the drawer lists them all.

Keys:

- `j`/`k` move (the drawer follows), `space` folds a list.
- `1`–`9` open the drawer's numbered links. `l` Linear, `f` Figma,
  `n` Notion, `g` GitHub PR open the first link of that kind. When there
  are several, the key highlights that drawer line and waits: a digit opens
  one, `a` opens all of that kind, `d` opens a Figma link in the desktop
  app, the same letter again opens the first, and `esc` cancels. With
  `figma_desktop` set (milestone 8) every Figma link the deck opens, by
  key, digit or click, opens in the desktop app. `o` opens
  the first localhost link whose dev server is running, without a chooser
  (milestone 6): links to servers that are down open only by their digit.
- `enter`: on a thread row, focus its herdr pane (`herdr pane focus
  <pane_id>`); on an inbox row, the subject thread's pane (else the
  coordinator's).
- `e`: open the worktree in the configured editor, `code {path}` by
  default. Milestone 4 left `$VISUAL` out because a terminal editor would
  take over the deck's own pane; since the config file (below), an editor
  with `terminal = true` opens in a new herdr pane instead. `$VISUAL` and
  `$EDITOR` are still not read implicitly. `r`: the
  thread's report in the drawer at full height. `!`: sources. `?`: help.
- `d`: the Files section waits for a file's digit (highlighted, scrolled
  into view): the digit opens that file's diff, `d` again, `a` or `enter`
  the whole diff, `esc` cancels. With one file `d` opens the whole diff at
  once. A click on a file opens it, on the total line the whole diff. The
  diff tool gets the merge-base commit as `{base}`. A rename passes both
  paths (a lone `{file}` argument becomes one per file) so git pairs them;
  an untracked file does not open, since git diff leaves it out.
- `u`: run the manifest's `up` command for the worktree, detached (own
  session and process group), logging to
  `$XDG_STATE_HOME/herdr-deck/logs/<slug>-<thread>.log` (else
  `~/.local/state/…`), with a `.pid` file beside it so a restarted deck
  still knows the command runs. Not started again while a manifest port
  answers or that pid is alive; the drawer shows the log and `starting…`
  until the ports answer.
- Open URLs with `open` (macOS) / `xdg-open`.

Mouse: a click selects a row, a click on a drawer link opens it, and clicks on
`! N` and on a list heading work like their keys. The wheel scrolls the list or
drawer under the pointer.

Colours are named ANSI colours, so the terminal theme applies (full table in
[docs/design/README.md](design/README.md#colours)). Needs you is bold red,
inbox items yellow, working cyan, review magenta, landing, passing checks and
listening ports green, and idle and metadata dim. The selected row (and the
link chooser's line) has a subtle grey background, ANSI 256 colour 237, or 254
on a light terminal, under the text's own colours. The deck never marks inbox
items handled.

## herdr integration

Ship a `herdr-plugin.toml` in the repo root. Verified in milestone 7 against
herdr 0.9.3, on a separate headless herdr server (`HOME=<scratch> herdr
server` with its own `HERDR_SOCKET_PATH`: plugins, config and sockets then
live under that home, apart from the user's own herdr):

- `[[build]]`: `go build -ldflags "-X main.version=<version>" -o
  bin/herdr-deck ./cmd/herdr-deck`. argv has no variables, so the version is
  written twice in the manifest; a test fails when they differ.
- `[[panes]] id="deck" placement="split"`, command = `bin/herdr-deck`. herdr
  runs plugin commands in the plugin folder, so the hooks open the deck with
  `cwd` = the project folder and `HERDR_DECK_PROJECT` (and
  `HERDR_PROJECTS_ROOT`) in its environment. A plugin pane's `HERDR_PANE_ID`
  is its own pane.
- Deck panes are marked with the pane token `herdr_deck=<slug>` (source
  `herdr-deck`): the hook sets it right after opening, and the deck sets it
  on its own pane at start. herdr's state has no plugin-ownership field (a
  plugin pane only has the manifest title as `label`).
- `[[actions]] id="toggle"`, `contexts=["workspace"]` runs `herdr-deck
  plugin toggle`. On the focused pane (`HERDR_PLUGIN_CONTEXT_JSON`'s
  `focused_pane_id`): when it is a deck, close it and focus the pane left of
  it (`pane.neighbor` before the close, then `pane.focus`; herdr picks when
  that fails); else when its tab has a deck, focus that deck; else open a
  focused deck to its right for the project of its folder, its `hp_project`
  token, or `$HERDR_DECK_PROJECT`; with none it shows a herdr notification.
  The auto-open hook never takes the focus. Bind it
  with `[[keys.command]] type="plugin_action" command="herdr-deck.toggle"` —
  **ask the user before editing their herdr config**.
- `[[events]] on="pane.agent_detected"` runs `herdr-deck plugin
  agent-detected`. It fires when herdr sees an agent start in a pane, also
  one started by typing in an existing shell; `workspace.created` fires
  earlier, while the pane is still a shell, so it can't tell a coordinator.
  The hook gets the pane in `HERDR_PANE_ID` and
  `HERDR_PLUGIN_EVENT_JSON` = `{"event":"pane_agent_detected","data":{"pane_id",
  "workspace_id","agent"}}`. It opens a deck when the pane's cwd is exactly
  `<root>/<slug>` with a PROJECT.md (symlinks resolved: herdr reports
  physical paths) and no pane of the workspace has the deck token. Threads
  run in worktrees or `<slug>/threads/…`, so only coordinators match. A file
  lock in `HERDR_PLUGIN_STATE_DIR` keeps two hooks from both opening one.
- Sizing: `plugin.pane.open` takes no ratio, so the hook finds the new split
  in `layout.export`'s tree (its path is a list of booleans, `true` = second
  child) and calls `layout.set_split_ratio`. The deck gets 40 % of the
  split, 60–80 columns, and the left pane keeps at least 60; below that they
  share it evenly.
- `[[link_handlers]] id="figma"` sends Ctrl+clicked Figma URLs to the
  `open-link` action (`herdr-deck plugin open-link`), which opens the
  `figma://` rewrite when `figma_desktop` is set, else the URL, as the deck
  itself does. Verified in
  milestone 8 on herdr 0.9.3: a link handler only ever sees a URL. herdr
  turns text into a link only for `http://`/`https://` (`url_byte_range` in
  its `src/app/actions.rs`), or takes an OSC 8 hyperlink's URI, and only on
  a Ctrl+click. A bare `ABC-123` is never a link, so Linear IDs go through
  the same action on a selection instead: a `[[keys.command]] type =
  "plugin_action"` key passes the selection as `selected_text`, and the
  action opens its first link or Linear ID with the deck's scraper rules.
  With no Linear workspace it opens nothing, shows a herdr notification and
  logs why. herdr re-reads a linked plugin's manifest on each click, so a
  new handler needs no relink and no `reload-config`; the binary must be
  rebuilt for a new subcommand.

Install with `herdr plugin install FredricW/herdr-deck`, or `herdr plugin
link <checkout>` for development (link never builds); see the README.

## Configuration

Settings live in `$XDG_CONFIG_HOME/herdr-deck/config.toml`, else
`~/.config/herdr-deck/config.toml` (macOS too, not Application Support);
`--config` or `$HERDR_DECK_CONFIG` names another file. Each setting comes
from the flag, else the environment variable, else the file, else the
default (`internal/config`). Only the settings page writes the file, and
never a secret.

- Keys: `linear_workspace`, `refresh_interval` (Go duration, 1s–10m, default
  5s), `projects_root`, `reuse_browser_tabs` (default true), `[editor]` and
  `[diff]`. The README has the full
  table and an example.
- `[editor]` and `[diff]` are a command plus `terminal`. Commands are split
  into argv like a POSIX shell but never run through one. Placeholders:
  `{path}` (both; appended to the editor when absent), `{base}` and `{file}`
  (diff; without a file a lone `{file}` argument and a `--` before it are
  dropped). A terminal program opens in a new herdr pane below the deck
  (`pane.split` with `cwd` = the worktree, then `pane.send_input` types the
  quoted command and Enter); outside herdr the status line says it needs
  herdr. The source that gives a command also decides its `terminal`, or a
  higher one does.
- The diff tool (`config.Settings.Diff`, `launch.DiffArgv`) opens from the
  Files section (`d`, see UI). Default `hunk
  diff {base} -- {file}` (working tree against the base ref, uncommitted
  changes included) when `hunk` is on PATH, else `git -C {path} diff
  --merge-base {base} -- {file}`, which pages itself in the pane.
- A missing file is silent. A file that does not parse is ignored as a
  whole; an unknown key or bad value is skipped on its own. Either way the
  problem is listed in `Snapshot.Missing` (the `!` view) or, for the plugin
  commands, on stderr (herdr's plugin log). A bad flag value is an error.
- Plugin mode: the hooks resolve the projects root from the same file and
  pass `$HERDR_DECK_CONFIG` on to decks they open when it is set; the deck
  finds the default path itself.
- `linear_status` (default true) and `linear_api_key_command`: see
  "Linear issue status" below.
- `update_check` (the header's update hint) and `auto_restart` (re-exec
  when the binary is replaced), both on by default. New keys are new
  optional fields, so an older deck only reports a newer file's keys as
  unknown.
- Settings page (`s`, `internal/ui/settings.go`): a full-height drawer view
  listing `config.Specs` in groups (Links and Linear, Editor and diff,
  Updates, Browser, Projects and refresh), each with its effective value
  and source (`Settings.Values`, recorded by `Resolve` as it settles each
  setting). `↵` toggles a boolean or edits a value in place, checked by
  `config.Check` as you type; `x` removes the key from the file. There are
  no enum settings yet. A save goes through `config.Save`: it patches only
  that key's line (comments, order and unknown keys stay; a new key goes
  after the last key of its table, a new table at the end), refuses a file
  that does not parse or an inline `editor = {…}` table, decodes the result
  to check only that key changed, and writes a temp file renamed over the
  old one (through a symlink, keeping the mode). A value set by a flag or
  env var still saves to the file; the page and status line say the
  override wins. After a save the deck resolves again and applies it at
  once: the UI takes the refresh interval, Figma desktop and the update
  hint, and `main` swaps an `atomic.Pointer[config.Settings]` that opening
  links, the editor and the diff tool, the Linear workspace and reader
  (rebuilt when the key command changes), the update check and
  auto-restart read each time.
  Only `projects_root` needs a restart, and the status line says so.
  `linear_api_key_command` is shown and edited as a command; the key it
  prints is never run, shown or written.

## Updates

Option B of the research doc (option A until 2026-10-03): releases carry
prebuilt binaries, and herdr's build step uses them when it can.

- `.github/workflows/release.yml` (a plain workflow, not GoReleaser): a
  `v*` tag push checks the tag against the manifest (`check-version.sh`),
  builds darwin and linux on arm64 and amd64 with `CGO_ENABLED=0` and
  `-X main.version=<tag>`, packs `herdr-deck_<version>_<os>_<arch>.tar.gz`
  (binary and README) with `SHA256SUMS`, and creates the GitHub Release with
  the tag's CHANGELOG.md section as its notes (`scripts/release-notes.sh`),
  falling back to `scripts/changelog.sh` (first-parent PRs and commits since
  the previous tag) for a tag the changelog does not cover. Only the publish job gets `contents: write`. Pull requests
  that touch the release files, and a manual run with `dry_run` on, only
  build and keep the files as a workflow artifact; a manual run with a tag
  and `dry_run` off publishes a tag that has no release yet. Artifact
  attestations are left for later.
- `herdr-plugin.toml`'s `[[build]]` runs `scripts/build.sh`. In a detached
  checkout with no local changes whose commit is the `v<manifest version>`
  tag (asked of origin, since herdr's shallow checkout has no tags) it
  downloads that release's archive for the machine, checks it against
  `SHA256SUMS`, checks that `--version` names the tag, and renames it to
  `bin/herdr-deck`. Anything else (a branch checkout such as the linked dev
  checkout, local changes, another commit, no release, no network, a
  mismatch) builds from source with `go build`, stamped with `git describe`
  or else the manifest's version. `HERDR_DECK_BUILD=source` forces a source
  build; `HERDR_DECK_DOWNLOAD_URL` points at another release host (tests).
  `scripts/test-build.sh` covers these cases against a fake release and a
  fake `go`, in CI's version job. `make build` is unchanged.
- Cutting a release: in a PR, bump `version` in `herdr-plugin.toml` and
  move CHANGELOG.md's Unreleased items into a `## [X.Y.Z] - date` section;
  merge, then push the `vX.Y.Z` tag on that merge commit.
  `check-version.sh` fails a tag without a CHANGELOG.md section (it skips
  the check in a checkout with no CHANGELOG.md, such as v0.1.0's).
- CHANGELOG.md (Keep a Changelog) is the one source of release notes. The
  deck embeds it (`go:embed` in the root package `herdrdeck`) and
  `internal/changelog` parses it into releases: a section it cannot read is
  left out and named as a problem (a test keeps the repo's file clean).
  `w` toggles What's new in the drawer: the releases newest first, the
  running one marked (a release build is its release; a build past one,
  a git describe or a bare commit, is Unreleased, which a release build
  does not show). `$XDG_STATE_HOME/herdr-deck/last-version` (else
  `~/.local/state/herdr-deck/`) records the last X.Y.Z a deck ran; a deck
  that starts on a newer one says `Updated to vX.Y.Z · w what's new` in the
  footer until the first key or click. The first run, a bare commit and a
  downgrade say nothing. While `↑` shows, the check also reads the newer
  version's CHANGELOG.md (`Updater.Changelog`: GitHub's raw host at the
  tag, or `git show <origin head>:CHANGELOG.md`, else the fetched origin
  branch, for a linked checkout, which is read again on the next check),
  once per newer version. What's new lists its Unreleased and its releases
  newer than the running one, without items this build's changelog already
  lists. Failures leave the list out, silently.

- `herdr-deck update [--check]` (`internal/update`) reads `herdr plugin list
  --plugin herdr-deck --json`. A GitHub install compares the newest `vX.Y.Z`
  tag (`git ls-remote --tags`) with the installed version and runs `herdr
  plugin install <repo> --ref <tag> --yes`. A linked checkout compares
  origin's default branch (`git ls-remote --symref origin HEAD`) with the
  installed commit; it pulls (`--ff-only`) only on that branch with no tracked
  changes, then builds to a temp file in `bin/` and renames it over
  `bin/herdr-deck`. `--check` only reports. "Installed" is the running
  binary when it is the plugin's `bin/herdr-deck`, else what that binary's
  `--version` says (the command may be typed in a shell running another
  build).
- The header hint (`↑ <tag or commit>`) comes from `update.Checker`: one
  cache file shared by all decks (`$XDG_CACHE_HOME/herdr-deck/update.json`,
  else `~/.cache/herdr-deck/`), so the remote is asked at most once an
  hour. The check runs off the UI goroutine at start and hourly; failures
  show only in Sources. Git never prompts (`GIT_TERMINAL_PROMPT=0`, ssh
  `BatchMode`).
- Restart in place (`internal/restart`): the binary path is resolved once
  at start. On each refresh tick the deck stats it; a file that differs
  (inode, size or mtime), has stayed the same for 2 s and answers
  `--version` makes Bubble Tea quit, then `syscall.Exec` runs it with the
  same args and env (same pid, so the herdr pane stays). If Exec fails the
  old deck starts again and the header says `↻ restart failed`. Not on
  Windows.

## Linear issue status

The drawer's Linear line shows each issue's state next to its ID (`1
ABC-123 in progress`), coloured by Linear's state type: started cyan (a
name with "review" magenta), triage yellow, unstarted and backlog plain,
completed and canceled dim, the ID too. The list's LINKS column keeps its
`L4` badge: six columns have no room for states. `internal/source/linear`:

- One GraphQL request (`https://api.linear.app/graphql`) per 50 IDs: per
  team key an aliased `issues(filter: { team: { key: { eq: $kN } }, number:
  { in: $nN } }, includeArchived: true)`, keys and numbers as variables. An
  ID Linear does not know is just missing from the nodes and has no state.
  Not `issue(id:)`: it returns a non-null `Issue!`, so one unknown ID would
  null the whole response.
- `live.Source.Read` calls `Apply`, which lays the cache over every Linear
  link and starts a background fetch for IDs missing or older than 3
  minutes, one at a time. When it ends, `OnUpdate` sends a reload. A reload
  never waits on the network.
- Failures go to `Snapshot.Missing` as `Linear status: …`, and the next try
  waits: a minute after an error, 5 minutes after a refused key (HTTP
  401/403 or `AUTHENTICATION_ERROR`), until the reset time after a rate
  limit (HTTP 429 or `RATELIMITED`, `X-RateLimit-Requests-Reset`). Without
  a key the Sources view has a note, not a missing source.
- Key: `$LINEAR_API_KEY`, else `linear_api_key_command` from the config
  file (user's decision, 2026-10-03), split like the editor command and run
  without a shell, with a 30 s timeout, at most once per deck start; a
  refused key lets it run again. A failed command is not retried until the
  deck restarts. The key is sent as `Authorization: <key>`, kept in memory
  only and redacted from every error; a command's output is never echoed.
- Tests use a fake `http.RoundTripper` and command runner; they never call
  Linear.

## Milestones

Each milestone is one thread / one PR, landed before the next starts unless
marked parallel.

1. **Skeleton.** `go mod init`, `cmd/herdr-deck`, Bubble Tea app with
   sections and fake data, key help, `--project` flag, `make build/test/lint`
   (golangci-lint if available, else `go vet`), README, CI-free.
2. **herdr-projects reader** (parallel with 3). `internal/source/projects`:
   PROJECT.md, `thread list --json` with TOML fallback, `ticker.json` PRs,
   inbox. Fixtures are a made-up project in the shapes herdr-projects
   writes. Tests.
   Done: the live group token maps to the deck status (`waiting-on-you` →
   needs you, `working` → working, `ready-for-review`/`landing` → review,
   `resolved` → done, `idle`/unknown → unknown). Unreadable sources are
   listed in `deck.Snapshot.Missing`; `Read` never returns an error.
3. **Tasks + links** (parallel with 2). `internal/source/tasks`: loose
   TASKS.md parser and link scraper as specified above, with tests covering
   both TASKS.md shapes (the strict format and a loose hand-written file).
4. **Live UI.** Replace the skeleton's sections with the list + drawer
   layout from the UI section (needs-you pinning, folding, numbered drawer
   links) and wire 2+3 into it. Refresh on file changes (fsnotify on the
   project folder) plus a 5 s tick. Link keys, digits and mouse open URLs.
   Done: `internal/source/live` combines the readers and watches the folder;
   `↵` and the drawer's `Dev` line are placeholders for milestones 5 and 6.
5. **herdr live state.** Snapshot + socket subscription (verify streaming
   first; fall back to polling). Pane focus on `enter`. "Needs you" from live
   agent state.
   Done: `internal/source/herdr` reads the snapshot on every reload and
   streams events that trigger reloads. "Needs you" from live state follows
   herdr-projects' own rule (`src/thread.rs`, `group`, row 4): an agent
   `blocked` for 30 s (`BLOCKED_DEBOUNCE_SECS`) makes its thread need the
   user, so the deck only runs ahead of herdr-projects' ~15 s ticker, never
   against it. A shorter block is ambiguous and the herdr-projects group
   stands. A `working` agent overrides a stale needs-you unless the thread
   failed; `idle`, `done` and `unknown` leave the group as it is. `enter`
   runs `pane.focus` over the socket.
   Fixed in t-0009: the threshold was 5 s, and a thread waiting on its own
   background /code-review showed under Needs you while herdr-projects kept
   it working. herdr 0.9.3 detects the agent's state from its screen
   (`herdr agent explain <pane>` names the rule; the Claude rules are in
   `~/.local/state/herdr/agent-detection/remote/claude.toml`). Claude's
   "Waiting for N background agents to finish" line is `working`
   (`background_agents_working`), and the snapshot has no field that says
   which rule fired, so a short `blocked` (a dialog that closes again) cannot
   be told from a real question in time. Hence the debounce.
6. **Dev servers.** Reader for the `.herdr-deck/dev.json` manifest + port
   probe; per-thread localhost links with running dots; `port` token fallback.
   Done: `internal/source/dev` runs after herdr's reader (which sets each
   thread's `PortToken`); localhost links carry `Down` when their server does
   not listen; `Snapshot.Notes` holds Sources lines that are not missing data.
7. **herdr plugin.** `herdr-plugin.toml`, toggle action, auto-open event,
   split sizing. Document install in README.
   Done: `internal/plugin` holds the toggle action and the
   `pane.agent_detected` hook (`herdr-deck plugin toggle|agent-detected`);
   coordinators only, one deck per workspace, sized with
   `layout.set_split_ratio`. Details in "herdr integration" above.
   Later (user, 2026-10-03): the toggle focuses the deck it opens, focuses
   a deck that is not focused instead of closing it, and closes only a
   focused deck.
8. **Polish / later.** Done: the config file (see Configuration), with a
   configurable editor and diff tool, a blank line between the list's
   groups, browser tab reuse (`launch.Browser`: on macOS, AppleScript
   finds a tab of the default browser showing the same page, as `pageKey`
   defines it, and focuses it; else `open`/`xdg-open`; the README's
   "Browser tabs" has the rules), `u` starts dev servers (the manifest's
   `up`), Figma link handlers with the open-link action (see herdr
   integration), and Linear issue status next to IDs (see "Linear issue
   status" above), the drawer's Files section, whose `d` opens the diff
   tool (see UI), the settings page (`s`, see Configuration),
   CHANGELOG.md with What's new (`w`, see Updates), and README GIFs that
   `make demo` renders with VHS from `docs/demo/*.tape` on `--fake` data
   (not in CI). Still to do: multi-project view (design directions to
   choose from in
   [docs/design/multi-project/](design/multi-project/README.md)).

## Definition of done (v1 = milestones 1–7)

- Running `herdr-deck` next to a project's coordinator shows its tasks
  (from a loose, non-strict TASKS.md too), its threads with live status and
  PR state, inbox count, and working localhost links for any running dev
  server.
- Every link kind opens from a key and from a mouse click.
- Opening a project with `herdr-projects open <slug>` gets a deck pane next to
  the coordinator without manual steps.
- `go test ./...` passes; no writes to anything under `~/.herdr-projects`.

## Open questions for the user

None. Settled:

- Name: it stays `herdr-deck`.
- The deck opens on its own next to coordinators only, never next to thread
  agents; the toggle action still opens one anywhere.
- `o` opens the first running localhost link directly, with no chooser.
- CI, releases and self-update: the recommendations in
  [docs/research/ci-releases-selfupdate.md](research/ci-releases-selfupdate.md)
  (its last section) are accepted.
