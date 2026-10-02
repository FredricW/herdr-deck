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
| Dev servers | herdr-deck's own manifest, `.herdr-deck/dev.json` in the worktree, else in the repo's main checkout (the thread's `repo`) | Tells the deck where a worktree's dev-server ports live: `state.file` is a path template per worktree (e.g. `.dev/$DIRNAME/state.json`; also `$BRANCH`, `$WORKTREE`, `$REPO`; relative to the main checkout), `state.ports` maps server names → JSON keys in that file (e.g. `api_port`, `frontend_port`) or fixed port numbers, and `links[]` are URL templates with `$PORT_<name>` placeholders and the servers they `need`. "Running" = TCP connect to 127.0.0.1:port, 400 ms timeout, probed in parallel, answers cached 2 s. Format in the README. A repo without a manifest is a note in Sources (not a missing source); a broken manifest or state file is missing. |
| Fallback port | herdr workspace token `port` on the workspace whose `worktree.checkout_path` is the thread's worktree (set by a worktree plugin, e.g. from a hash of the branch) | Use only when no manifest port is found; marked `~`. |

**Never** call `herdr-projects context <slug>` without `--peek`: without it
the command marks inbox items as seen.

### Loose task parsing

Accept both shapes. For each list item under a `##` heading:

- title = the line text minus checkbox, owner and thread suffix;
- done = `[x]`; list = the heading;
- thread ids = every `t-\d{4}` in the line and its indented notes;
- links (from the line, its notes, and the linked threads' `.task.md`/`.md`):
  - Linear: `\b[A-Z][A-Z0-9]{1,5}-\d+\b` → `https://linear.app/<workspace>/issue/<ID>`.
    There is no default workspace: it comes from `--linear-workspace` or
    `$HERDR_DECK_LINEAR_WORKSPACE`. Without one, IDs still show but say a
    workspace must be set instead of opening. A branch name gives a ticket
    too (`dev/abc-123-users` → ABC-123). Word boundaries on both sides, so
    ABC-110 ≠ ABC-1100 and P-ABC-49 is not ABC-49.
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
  list. Columns at 80: work, thread, status, PR, links (`L4` = four Linear
  links), dev (one dot per dev.json port). At 60 the thread and PR columns
  fold into status.
- **Needs you on top.** Threads waiting on you and unhandled inbox items move
  into a red *Needs you* group at the top of the list. If the cursor has not
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
  app, the same letter again opens the first, and `esc` cancels. `o` opens
  the first localhost link whose dev server is running, without a chooser
  (milestone 6): links to servers that are down open only by their digit.
- `enter`: on a thread row, focus its herdr pane (`herdr pane focus
  <pane_id>`); on an inbox row, the subject thread's pane (else the
  coordinator's).
- `e`: open the worktree in VS Code (`code <path>`). Milestone 4 left
  `$VISUAL` out: a terminal editor would take over the deck's own pane. `r`: the
  thread's report in the drawer at full height. `!`: sources. `?`: help.
- `u` (later): run the manifest's `up` command for the worktree, detached,
  logs to a file.
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

Ship a `herdr-plugin.toml` in the repo root:

- `[[build]]`: `go build -o bin/herdr-deck ./cmd/herdr-deck`.
- `[[panes]] id="deck" placement="split"`, command = `bin/herdr-deck`; the
  project slug comes from the pane's cwd (`~/.herdr-projects/<slug>`) or
  `$HERDR_DECK_PROJECT` (`internal/project`; `--project` wins over both).
- `[[actions]] id="toggle"`, `contexts=["workspace"]`: opens the deck in a
  split to the right of the focused pane
  (`herdr plugin pane open --plugin herdr-deck --entrypoint deck --placement split --direction right --target-pane <pane> --no-focus`),
  or closes it if one is open. Bind it in `~/.config/herdr/config.toml` with
  `[[keys.command]] type="plugin_action"` — **ask the user before editing
  their herdr config**.
- `[[events]] on="pane.agent_detected"` (or `workspace.created`): when the new
  agent's cwd is a herdr-projects project folder and no deck pane exists in
  that workspace, open one next to it. Unknown event names only warn, so test
  which event fires reliably for a coordinator start.
- Size the split after opening (`layout.set_split_ratio` over the socket, or
  `pane resize`); `plugin pane open` takes no ratio.
- Optional later: `[[link_handlers]]` so `ABC-123` and Figma URLs are
  clickable in any pane, coordinator included.

Install locally with `herdr plugin` (check `herdr plugin --help` for the
link/install-from-path command); don't publish anywhere.

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
   streams events that trigger reloads. An agent `blocked` for 5 s makes its
   thread need the user (herdr-projects waits 30 s); a `working` agent
   overrides a stale needs-you unless the thread failed; otherwise the
   herdr-projects group stands. `enter` runs `pane.focus` over the socket.
6. **Dev servers.** Reader for the `.herdr-deck/dev.json` manifest + port
   probe; per-thread localhost links with running dots; `port` token fallback.
   Done: `internal/source/dev` runs after herdr's reader (which sets each
   thread's `PortToken`); localhost links carry `Down` when their server does
   not listen; `Snapshot.Notes` holds Sources lines that are not missing data.
7. **herdr plugin.** `herdr-plugin.toml`, toggle action, auto-open event,
   split sizing. Document install in README.
8. **Polish / later.** Linear issue status next to IDs (GraphQL; key from
   env or `op`), `u` to start dev servers, link handlers, multi-project view.

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

- Name: `herdr-deck` is a working title.
- Whether the deck should also appear next to thread agents, or coordinators
  only (assumed coordinators only).
- CI, releases and self-update: options and a recommendation are in
  [docs/research/ci-releases-selfupdate.md](research/ci-releases-selfupdate.md);
  its last section lists the decisions to make.
