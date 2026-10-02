# herdr-deck

A status pane for [herdr-projects](https://github.com/eliasstravik/herdr-projects):
a Go + Charm TUI that runs in a split next to a project's coordinator and shows
its tasks, threads and their status, Linear / Notion / Figma / GitHub links and
running dev servers.

Work in progress: the deck reads a project's files and thread list and
refreshes as they change; herdr live state, dev servers and the herdr plugin
are still to come. See [docs/PLAN.md](docs/PLAN.md) for the plan and
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

## Keys

| Key | Action |
|---|---|
| `j` / `k`, `↓` / `↑` | move; the drawer follows |
| `space` | fold or unfold the list under the cursor (Backlog starts folded) |
| `1`–`9` | open the drawer's numbered link |
| `l` `f` `n` `g` `o` | open the row's Linear / Figma / Notion / PR / localhost link; with several, pick one with a digit, `a` for all, `d` for the Figma desktop app, the same letter for the first, `esc` to cancel |
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
  TASKS.md and scrapes links, `live` combines them and watches the project
  folder, `fake` is sample data for tests and `--fake`.
- `internal/launch`: opens URLs (`open` / `xdg-open`) and VS Code.
- `internal/project`: works out the project slug and the projects root.
- `internal/ui`: the Bubble Tea model; renders a `deck.Snapshot` and nothing else.
  Its rendering is pinned by `internal/ui/testdata/*.golden` (every state at
  60 and 80 columns); `go test ./internal/ui -update` rewrites them.
