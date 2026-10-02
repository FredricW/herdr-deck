# herdr-deck

A status pane for [herdr-projects](https://github.com/eliasstravik/herdr-projects):
a Go + Charm TUI that runs in a split next to a project's coordinator and shows
its tasks, threads and their status, Linear / Notion / Figma / GitHub links and
running dev servers.

Work in progress: the deck reads a project's files, thread list, herdr's
live state and each worktree's dev servers, and refreshes as they change; the
herdr plugin is still to come. See [docs/PLAN.md](docs/PLAN.md) for the plan and
milestones.

## Build and run

Needs Go 1.27 or newer.

```sh
make build                              # → bin/herdr-deck
./bin/herdr-deck --project admin-rebuild
make run ARGS="--project admin-rebuild" # build and run
./bin/herdr-deck --fake                 # built-in sample data
make test                               # go test ./...
make lint                               # golangci-lint if installed, else gofmt + go vet
```

The project slug comes from, in order:

1. `--project <slug>`;
2. `$HERDR_DECK_PROJECT`;
3. the working directory, when it is `<root>/<slug>` or below it, where
   `<root>` is `$HERDR_PROJECTS_ROOT` or `~/.herdr-projects`.

Bare Linear IDs such as `ABC-123` link into a Linear workspace you name, with
`--linear-workspace <slug>` or `$HERDR_DECK_LINEAR_WORKSPACE` (the flag wins).
There is no default: without one, the IDs still show, but the drawer and the
`!` sources view say a workspace must be set, and opening one says so too.
Full Linear URLs always open.

The deck reloads when a file in the project folder (or its `threads/`,
`inbox/` or `.state/`) changes, and every 5 seconds. It never writes there.

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
  ]
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
| `e` | open the thread's worktree in VS Code |
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
- `internal/launch`: opens URLs (`open` / `xdg-open`) and VS Code.
- `internal/project`: works out the project slug and the projects root.
- `internal/ui`: the Bubble Tea model; renders a `deck.Snapshot` and nothing else.
  Its rendering is pinned by `internal/ui/testdata/*.golden` (every state at
  60 and 80 columns); `go test ./internal/ui -update` rewrites them.
