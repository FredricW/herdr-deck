# herdr-deck

[![ci](https://github.com/FredricW/herdr-deck/actions/workflows/ci.yml/badge.svg)](https://github.com/FredricW/herdr-deck/actions/workflows/ci.yml)

A status pane for [herdr-projects](https://github.com/eliasstravik/herdr-projects):
a Go + Charm TUI that runs in a split next to a project's coordinator and shows
its tasks, threads and their status, Linear / Notion / Figma / GitHub links and
running dev servers.

The deck reads a project's files, thread list, herdr's live state and each
worktree's dev servers, and refreshes as they change. As a herdr plugin it
opens next to each coordinator by itself. See [docs/PLAN.md](docs/PLAN.md)
for the plan and milestones.

## Install as a herdr plugin

Needs herdr 0.9.3 or newer and Go 1.27 or newer: herdr builds the deck from
source when it installs the plugin.

```sh
herdr plugin install FredricW/herdr-deck            # add --ref v0.1.0 to pin a release
```

For local development, link a checkout instead. `link` never builds, so
build first and again after each change:

```sh
make build
herdr plugin link "$PWD"
```

What the plugin does (`herdr-plugin.toml`):

- **Opens a deck next to each coordinator.** When herdr detects an agent
  whose working directory is a project folder (`<root>/<slug>` with a
  `PROJECT.md`, as `herdr-projects open` starts it), a deck opens in a split
  to its right, about 40 % of the width (60–80 columns), without taking the
  focus. A workspace gets one deck: a second coordinator in it, or the same
  one restarting, opens no other. Thread agents run in worktrees or under
  `<slug>/threads/` and never get a deck of their own.
- **Toggle** (`herdr-deck.toggle`) works in three steps. With no deck in
  the focused pane's tab it opens one to the right of the focused pane, any
  pane, and focuses it. With a deck in the tab but the focus elsewhere it
  focuses the deck. On the deck itself it closes the deck, and the focus
  goes back to the pane left of it. The project
  is the one the pane works in: its folder under the projects root, else the
  herdr-projects `hp_project` token on the pane (thread panes have it), else
  `$HERDR_DECK_PROJECT`. Run it with `herdr plugin action invoke
  herdr-deck.toggle`, or bind it to a key in `~/.config/herdr/config.toml`:

  ```toml
  [[keys.command]]
  key = "prefix+d"
  type = "plugin_action"
  command = "herdr-deck.toggle"
  description = "toggle the deck"
  ```

  and reload with `herdr server reload-config`. Pick any free key.
- **Figma links and Linear IDs in any pane** (`herdr-deck.open-link`).
  Ctrl+click a Figma link anywhere in herdr (Ctrl on macOS too) and it opens
  in the Figma desktop app when `figma_desktop = true` is set in the deck's
  config file, else in the browser. herdr only makes http(s) URLs clickable,
  so a bare Linear ID such as `ABC-123` works through a selection instead:
  double-click the ID, then press a key bound to the action, and it opens in
  your `linear_workspace` by the same rules the deck links it. The first
  link or ID in a longer selection opens. Without a workspace nothing opens
  and a herdr notification says why.

  ```toml
  [[keys.command]]
  key = "prefix+l"
  type = "plugin_action"
  command = "herdr-deck.open-link"
  description = "open the selected Linear ID or link"
  ```

The deck marks its pane with the token `herdr_deck=<slug>` (source
`herdr-deck`); that is how the plugin finds it again. `herdr plugin log list
--plugin herdr-deck` shows what the hooks did and why one failed.

To update, install again (`herdr plugin install FredricW/herdr-deck`). To
remove it, `herdr plugin uninstall herdr-deck`, or `herdr plugin unlink
herdr-deck` for a linked checkout, and delete the keybinding.

## Build and run

Needs Go 1.27 or newer.

```sh
make build                              # → bin/herdr-deck
./bin/herdr-deck --project admin-rebuild
make run ARGS="--project admin-rebuild" # build and run
./bin/herdr-deck --fake                 # built-in sample data
./bin/herdr-deck --version              # version and commit
make test                               # go test ./...
make lint                               # golangci-lint if installed, else gofmt + go vet
```

`make build` stamps the version from `git describe --tags --always --dirty`;
other builds fall back to the module version and commit Go records.
CI (`.github/workflows/ci.yml`) runs gofmt, `go vet`, `go test -race` and
golangci-lint on Linux and macOS.

The project slug comes from, in order:

1. `--project <slug>`;
2. `$HERDR_DECK_PROJECT`;
3. the working directory, when it is `<root>/<slug>` or below it, where
   `<root>` is the projects root (see below).

The slug is per pane, so it has no config-file key.

Bare Linear IDs such as `ABC-123` link into the Linear workspace you set
(`linear_workspace`, below). There is no default: without one, the IDs still
show, but the drawer and the `!` sources view say a workspace must be set,
and opening one says so too. Full Linear URLs always open.

The deck reloads when a file in the project folder (or its `threads/`,
`inbox/` or `.state/`) changes, and every `refresh_interval` (5 seconds by
default). It never writes there.

## Configuration

The deck reads `$XDG_CONFIG_HOME/herdr-deck/config.toml`, else
`~/.config/herdr-deck/config.toml` (on macOS too). `--config <path>` or
`$HERDR_DECK_CONFIG` names another file. Decks the herdr plugin opens read
the same file, so settings there reach them even though they do not see your
shell's environment.

Each setting comes from, in order: the command-line flag, the environment
variable, the config file, the built-in default.

```toml
# ~/.config/herdr-deck/config.toml

# Linear workspace that bare IDs such as ABC-123 link into.
linear_workspace = "acme"

# Open Ctrl+clicked Figma links in herdr in the Figma desktop app.
figma_desktop = false

# How often the deck reloads when no file change says to; 1s to 10m.
refresh_interval = "5s"

# The herdr-projects root; ~ is your home folder.
projects_root = "~/.herdr-projects"

# Open a web link in a browser tab that already shows it (macOS); see
# "Browser tabs" below.
reuse_browser_tabs = true

# What `e` opens a thread's worktree with. {path} is the worktree folder;
# without it the folder is added at the end. Not run through a shell: quote
# arguments with spaces. terminal = true opens it in a new herdr pane below
# the deck, in the worktree, instead of starting a desktop app.
[editor]
command = "code {path}"   # e.g. "zed {path}", "cursor {path}"
terminal = false          # e.g. command = "nvim {path}" with terminal = true

# The diff tool, for showing a thread's changes (the deck does not use it
# yet). {path} is the worktree, {base} the branch it is compared against,
# {file} one file; without a file an argument that is just {file} is left
# out, with a "--" right before it. It runs in the worktree.
[diff]
command = "hunk diff {base} -- {file}"
terminal = true
```

| Setting | Flag | Environment variable | Default |
|---|---|---|---|
| `linear_workspace` | `--linear-workspace` | `HERDR_DECK_LINEAR_WORKSPACE` | none |
| `figma_desktop` | none | `HERDR_DECK_FIGMA_DESKTOP` | `false` |
| `refresh_interval` | `--refresh-interval` | `HERDR_DECK_REFRESH_INTERVAL` | `5s` |
| `projects_root` | `--projects-root` | `HERDR_PROJECTS_ROOT` | `~/.herdr-projects` |
| `reuse_browser_tabs` | `--reuse-browser-tabs` | `HERDR_DECK_REUSE_BROWSER_TABS` | `true` |
| `[editor] command` | `--editor` | `HERDR_DECK_EDITOR` | `code {path}` |
| `[editor] terminal` | `--editor-terminal` | `HERDR_DECK_EDITOR_TERMINAL` | `false` |
| `[diff] command` | `--diff-tool` | `HERDR_DECK_DIFF_TOOL` | `hunk diff {base} -- {file}`, or `git -C {path} diff --merge-base {base} -- {file}` without hunk |
| `[diff] terminal` | `--diff-terminal` | `HERDR_DECK_DIFF_TERMINAL` | `true` |

A command and its `terminal` option go together: the source that gives the
command also decides `terminal` (or one above it does), so `--editor "zed
{path}"` does not open in a pane because the file says `terminal = true` for
nvim. Without a `terminal` value, the editor is a desktop app and the diff
tool a terminal program. A terminal program needs herdr; outside herdr, `e`
says so in the status line.

The deck does not read `$VISUAL` or `$EDITOR`. To use yours, put it in the
file, e.g. `command = "nvim {path}"` with `terminal = true`.

The deck only reads this file and never writes secrets to it. A missing
file is fine. A file that does not parse, an unknown key or a bad value
never stops the deck: the `!` sources view lists the problem (the plugin
commands print it to herdr's plugin log), and that setting falls back to
the next source. A bad flag value is an error.

## Browser tabs

On macOS the deck opens a web link by focusing a tab that already shows the
same page, raising its window and bringing the browser forward, and only
opens a new tab when there is none. It does this for the default browser
when that is Chrome, Chromium, Brave, Edge, Vivaldi, Arc or Safari, through
AppleScript (`osascript`). Any other browser (Firefox, for one), a browser
that is not running, or Linux opens a new tab with `open` or `xdg-open`.
Turn it off with `reuse_browser_tabs = false` (or the flag or variable
above).

The same page means:

- Linear: the same workspace and issue ID; the title slug may differ.
- GitHub: the same pull request or issue, whichever of its tabs (files,
  commits, checks) is showing.
- Figma: the same file (or branch), whatever frame is selected. A
  prototype is not the same page as its design file.
- Notion: the same page ID, also when it is open over a database (`?p=`).
- A dev server on `localhost`, `127.0.0.1`, `[::1]` or `*.localhost`: the
  same scheme, host and port, wherever in the app the tab is.
- Anything else: the same URL, ignoring case in the host, a trailing slash
  and the `#fragment`.

A tab showing exactly the link wins over one that only shows the same page,
and the deck does not navigate the tab it focuses. It reads tab URLs only to
compare them; nothing is kept or sent anywhere.

The first time, macOS asks whether your terminal app (the one herdr runs
in) may control the browser. If you refuse, or the question is still open
after 10 seconds, the link opens in a new tab, as it does every time after
a refusal. To change your answer, use System Settings → Privacy & Security
→ Automation, or turn reuse off.

## Dev servers

The deck shows the dev servers of each open thread's worktree: one dot per
server in the list's DEV column (green `●` when something listens on the
port, dim `○` when not), and in the drawer a `Dev` line with each port and an
`Open` line with numbered localhost links. A port is running when a TCP
connect to `127.0.0.1:<port>` or `[::1]:<port>` succeeds within 400 ms. Ports are probed in
parallel and each answer is reused for 2 seconds.

A repository says where its worktrees' ports are in `.herdr-deck/dev.json`.
The deck reads the worktree's own copy, else the one in the repository's main
checkout:

```json
{
  "state": {
    "file": ".dev/$DIRNAME/state.json",
    "ports": { "frontend": "frontend_port", "api": "api_port", "docs": 6060 }
  },
  "links": [
    { "title": "Frontend", "url": "http://localhost:$PORT_frontend", "needs": "frontend" },
    { "title": "API docs", "url": "http://localhost:$PORT_api/docs", "needs": ["api"] }
  ],
  "up": "scripts/dev-up --name $DIRNAME"
}
```

- `state.ports` names the servers, in the order the deck shows them. Each
  maps to the key in the state file that holds its port (a number or a
  numeric string), or to a fixed port number.
- `state.file` is the per-worktree state file your dev scripts write when
  they start the servers, e.g. `{"frontend_port": 5181, "api_port": 8011}`.
  It may use `$DIRNAME` (the worktree's folder name), `$BRANCH`, `$WORKTREE`
  and `$REPO` (absolute paths of the worktree and the main checkout), also as
  `${…}`; write `$$` for a literal `$`. A relative path is under the main checkout. While the file does not
  exist the drawer says the servers are not started.
- `links` are URL templates with `$PORT_<name>` placeholders and the same
  variables. `needs` is the server or servers that must run for the link to
  work (default: the ports the URL uses); a link to a server that is down,
  or has no port yet, shows `○`, and `o` skips it. A link whose URL uses a
  port that is not known yet is left out until it is.
  Without `links`, each server gets `http://localhost:<port>`.
- `up` (optional) is a shell command that starts the worktree's dev
  servers; `u` runs it for the selected thread. It runs in the worktree with
  `/bin/sh -c`, after the deck fills in the same variables as `state.file`
  (values go in as they are, so quote paths that may hold spaces). Every
  other `$` is a mistake the deck reports, so write `$$` for one the shell
  should see (`$$HOME`). The command runs detached, in its own process
  group, so it keeps running when the deck quits or restarts; its output is
  appended to `$XDG_STATE_HOME/herdr-deck/logs/<project>-<thread>.log`, else
  `~/.local/state/herdr-deck/logs/…`. The drawer shows that log and
  `starting…` until every port answers, or `up exited` when the command
  ended with no port answering. `u` does not start it again while a port
  answers or the command it started still runs. Without `up`, `u` says how
  to add one.

When the manifest gives no port for a worktree, the deck falls back to the
herdr workspace token `port` of the workspace opened on that worktree (a
worktree plugin may set it), marked `~` in the list and drawer. A repository
without a manifest is noted in the `!` sources view; a manifest or state file
that cannot be read is listed there as missing, and the rest of the deck
carries on.

## Keys

| Key | Action |
|---|---|
| `j` / `k`, `↓` / `↑` | move; the drawer follows |
| `space` | fold or unfold the list under the cursor (Backlog starts folded) |
| `1`–`9` | open the drawer's numbered link |
| `l` `f` `n` `g` | open the row's Linear / Figma / Notion / PR link; with several, pick one with a digit, `a` for all, `d` for the Figma desktop app, the same letter for the first, `esc` to cancel |
| `o` | open the row's first localhost link whose dev server is running |
| `enter` | focus the thread's herdr pane; on an inbox item, its thread's pane, else the coordinator's |
| `e` | open the thread's worktree in the editor (VS Code unless configured) |
| `u` | start the thread's dev servers: the dev manifest's `up` command, detached (see Dev servers) |
| `r` | the thread's report, full height |
| `z` | drawer: normal, full height, hidden |
| `pgup` / `pgdn` | scroll the drawer |
| `!` | sources the deck could not read |
| `?` | all keys |
| `esc` | back to the selected row |
| `q`, `ctrl+c` | quit |

Mouse: click a row to select it, a list heading to fold it, a drawer link to
open it, `! N` for the sources; the wheel moves the list or scrolls the
drawer.

## Layout

- `cmd/herdr-deck`: flag parsing and wiring.
- `internal/deck`: the plain structs the deck shows (`deck.Snapshot`).
- `internal/source/*`: produces snapshots and never imports the UI.
  `projects` reads herdr-projects' files and thread list, `tasks` parses
  TASKS.md and scrapes links, `herdr` reads herdr's live panes and events,
  `dev` reads dev-server manifests and probes ports, `live` combines them and
  watches the project folder, `fake` is sample data for tests and `--fake`.
- `internal/config`: finds and reads `config.toml` and resolves each setting
  (flag > environment > file > default).
- `internal/launch`: opens URLs (`open` / `xdg-open`, reusing a browser tab
  on macOS) and runs the editor and diff tool, as desktop apps or in a new
  herdr pane.
- `internal/project`: works out the project slug.
- `internal/plugin`: the herdr plugin's toggle and open-link actions and
  auto-open hook (`herdr-deck plugin toggle|open-link|agent-detected`, run
  by herdr).
- `internal/ui`: the Bubble Tea model; renders a `deck.Snapshot` and nothing else.
  Its rendering is pinned by `internal/ui/testdata/*.golden` (every state at
  60 and 80 columns); `go test ./internal/ui -update` rewrites them.
