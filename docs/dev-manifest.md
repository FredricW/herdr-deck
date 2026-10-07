# Dev manifest, version 1

Status: **approved** (2026-10-04). herdr-deck implements lookup, ports
from fixed numbers and state files, services, `dev` and `stop`, run records
and logs; the port store, `start <service>`, other commands, seeding and
drift come later (see the README's *Dev servers*).

A dev manifest is a committed file, `.config/dev.json`, that tells any tool how
to work a repository's checkout: which services it runs and on which ports,
how to start and stop them, how to set it up, test it and lint it, and which
pages to open. It is tool-neutral. herdr-deck reads it, and so can any other
tool (a launcher, an editor extension, a script); none of them owns it.

The JSON Schema is [`schema/v1/dev.schema.json`](../schema/v1/dev.schema.json); the
port store's is [`schema/v1/dev-ports.schema.json`](../schema/v1/dev-ports.schema.json).

The key words MUST, MUST NOT, SHOULD, SHOULD NOT and MAY are used as in
RFC 2119. *Tool* means any program that reads the manifest. *Worktree* means
the checkout a tool acts on: a git worktree, or the main checkout itself.

Contents:

1. [Goals](#1-goals)
2. [File, lookup and versioning](#2-file-lookup-and-versioning)
3. [Variables](#3-variables)
4. [Top-level fields](#4-top-level-fields)
5. [Ports](#5-ports)
6. [Services and groups](#6-services-and-groups)
7. [Commands](#7-commands)
8. [Verbs](#8-verbs)
9. [How commands run](#9-how-commands-run)
10. [The shared state folder and the port store](#10-the-shared-state-folder-and-the-port-store)
11. [Links](#11-links)
12. [Seeding and drift](#12-seeding-and-drift)
13. [Tool extensions](#13-tool-extensions)
14. [Examples](#14-examples)
15. [Migrating from `.herdr-deck/dev.json`](#15-migrating-from-herdr-deckdevjson)
16. [Other tools' files (non-normative)](#16-other-tools-files-non-normative)
17. [Decisions](#17-decisions)

## 1. Goals

- **The same verbs in every repository.** `dev`, `stop`, `setup`, `test`,
  `lint`, … mean the same thing everywhere; each repository says what they run.
  No common standard does this (npm reserves a few names, Scripts to Rule Them
  All is a convention, VS Code tasks have two roles), so this file defines a
  small vocabulary of its own.
- **Several worktrees at once.** Each worktree gets its own ports, stable
  across restarts, handed out by one store that every tool shares, so two
  tools never give the same port to two worktrees.
- **Monorepos.** Several services, each with its own folder, command, ports,
  dependencies, readiness check, environment and log.
- **Small when the repository is small.** A one-service repository needs a
  few lines (see [14.1](#141-one-service)).
- **Describe, don't replace.** The manifest points at the scripts and task
  runners a repository already has. It is not a task runner.

Not goals: building, deploying, containers, remote machines, or worktree
creation and removal (those stay with worktree tools; a tool that does them
keeps its settings under its own [`x-` key](#13-tool-extensions)).

## 2. File, lookup and versioning

### 2.1 File

The manifest is `.config/dev.json` at the root of a checkout. It is JSON
([RFC 8259](https://www.rfc-editor.org/rfc/rfc8259)) in UTF-8: no comments and
no trailing commas, so every tool can read it with its standard JSON parser.
Any object that allows [`x-` keys](#13-tool-extensions) also allows a
`"$comment"` string, which tools ignore.

The order of keys in `ports`, `services`, `groups` and `commands` is
meaningful: it is the order tools show them in and the order in which
per-service commands run. Readers MUST keep it (Go: decode those objects
token by token; JavaScript: `Object.entries` keeps it for names that are not
integers, and names here never are).

`"$schema"` MAY name the schema, so editors can check the file:

```
"$schema": "https://raw.githubusercontent.com/FredricW/herdr-deck/main/schema/v1/dev.schema.json"
```

The schema's path carries its version: `schema/v1/` holds the schemas for
`"version": 1`. A published `v1` schema only changes compatibly (new optional
fields, better descriptions); a breaking change gets `schema/v2/`, together
with `"version": 2`. Once a herdr-deck release contains the schema, a
tag-pinned URL (`…/herdr-deck/v0.2.0/schema/v1/dev.schema.json`, for example)
can be used instead of `main`, for editors that must never see a change.

### 2.2 Lookup

For a worktree `W` whose repository's main checkout is `R`, a tool MUST use
the first of these files that exists:

1. `W/.config/dev.json`
2. `R/.config/dev.json`
3. `W/.herdr-deck/dev.json` (legacy, [section 15](#15-migrating-from-herdr-deckdevjson))
4. `R/.herdr-deck/dev.json` (legacy)

The worktree's own copy comes first so a branch can change the manifest
before it merges. The first file found is the manifest; files are never
merged. A tool that reads a legacy file SHOULD say that it is legacy (herdr-deck
notes it in its `!` sources view). A tool MAY skip the legacy files once it no
longer supports them.

If the file found cannot be read, is not valid JSON, or breaks a MUST of this
spec, the tool MUST report the problem and MUST NOT fall back to a later file.
Falling back would silently run the main checkout's or the legacy commands
because of a typo. Without any file, the tool has no manifest; it MAY still
offer what [detection](#12-seeding-and-drift) finds, marked as guessed.

### 2.3 Versioning

`"version": 1` is required. A tool MUST refuse a manifest whose `version` it
does not know, and say so.

Within version 1, new optional fields may be added. The schema rejects
unknown keys so that editors catch typos, but a tool SHOULD ignore a key it
does not know (and MAY warn), so an older tool keeps working on a newer
version-1 file. A change that an older tool would misread (a field that
changes meaning, or a new required field) needs `"version": 2`.

## 3. Variables

Strings marked *template* in this spec are expanded before use:

| Variable | Value |
|---|---|
| `$WORKTREE` | Absolute path of the worktree, symlinks resolved. |
| `$REPO` | Absolute path of the repository's main checkout (the parent of `git rev-parse --path-format=absolute --git-common-dir`); the same as `$WORKTREE` when the worktree is the main checkout. |
| `$DIRNAME` | The last element of `$WORKTREE`. |
| `$BRANCH` | The worktree's branch (`git symbolic-ref --short HEAD`); empty on a detached HEAD. |
| `$PORT_<name>` | The worktree's port called `<name>` ([section 5](#5-ports)). |
| `$env(NAME)`, `$env(NAME\|fallback)` | `NAME` from the manifest's [env files](#41-env-and-path), the last file that sets it winning; else `fallback`, else empty. The tool's own environment is not read, so every tool gets the same value. |
| `$$` | A literal `$`. |

`${NAME}` is the same as `$NAME`. Names are case-sensitive.

Rules:

- Any other `$NAME` or `${NAME}` is an error, and so is `$PORT_<name>` for a
  port the manifest does not define. Tools MUST report these when they check
  the manifest. Write `$$HOME` in a shell command for the shell to see
  `$HOME`; injected [environment variables](#93-environment) are often the
  simpler choice (`$$PORT`).
- Values are inserted as they are, without quoting. In a shell string, quote
  a variable that may hold spaces (`"$WORKTREE"`), or use an argv.
- A port that is not known yet (its state file does not exist, or the store
  has not assigned it) leaves the template unusable: a link with it is left
  out, a command with it is not run and the tool names the missing port.

Templates are: `run` (a string, or each argv element), `dir`, `log`, the
values of every `env` map, `env.files`, `path`, `state.file`, `links[].url`,
`links[].title` and `ready.http`. Names, `title`s of commands and services,
and `x-` values are not templates.

Relative paths (after expansion) are relative to the worktree, except a
command's argv[0], which is looked up as [section 9.1](#91-string-argv-and-object) says.

## 4. Top-level fields

| Field | Type | Meaning |
|---|---|---|
| `$schema` | string | The schema URL, for editors. Ignored by tools. |
| `version` | `1` | Required. |
| `name` | string | Display name; default: the main checkout's folder name. |
| `env` | object | [Environment](#41-env-and-path) for every command. |
| `path` | array of templates | Folders put in front of `PATH` for every command. |
| `ports` | object | [Named ports](#5-ports). |
| `state` | object | The project's own [state file](#53-ports-from-a-state-file). |
| `services` | object | [Long-running processes](#6-services-and-groups). |
| `groups` | object | [Named sets of services](#62-groups). |
| `commands` | object | [Named commands](#7-commands). |
| `links` | array | [Pages to open](#11-links). |
| `detect` | object | [Drift settings](#124-drift). |
| `x-<tool>` | any | [Tool extensions](#13-tool-extensions). |

All fields except `version` are optional. A manifest with only
`{"version": 1}` is valid and says nothing.

### 4.1 env and path

```json
"env": {
  "files": ["$REPO/.env", ".env.local"],
  "vars": { "APP_ENV": "development", "API_URL": "http://localhost:$PORT_api" }
}
```

- `files`: dotenv files (`KEY=value` lines, `#` comments, optional `export `,
  single or double quotes), templates, relative to the worktree. A missing file
  is skipped silently. They feed `$env(…)` always, and a command's environment
  only when it sets `"loadEnv": true` ([section 9.3](#93-environment)), because
  many scripts read their own `.env` and would see values twice.
- `vars`: variables set for every command and service. Keys MUST be
  environment variable names (`^[A-Za-z_][A-Za-z0-9_]*$`).
- `path` (top level): folders put in front of `PATH`, first one first, e.g.
  `["node_modules/.bin", ".venv/bin"]`.

## 5. Ports

`ports` names every port the worktree uses and says where its number comes
from:

```json
"ports": {
  "web":  { "base": 3100 },
  "api":  { "base": 8100, "range": 50 },
  "db":   { "state": "db_port" },
  "docs": 6060
}
```

| Form | Source |
|---|---|
| a number, or `{ "fixed": N }` | Fixed: every worktree gets `N`. |
| `{ "base": N, "range": R }` | Assigned per worktree from the shared [port store](#102-the-port-store): the lowest free port in `N … N+R-1`. `range` defaults to 100. |
| `{ "state": "key" }` | Read from the project's [state file](#53-ports-from-a-state-file). |

For `{ "base": N, "range": R }`, `N + R - 1` MUST NOT be above 65535; a tool
MUST report such a port, and allocation never goes above 65535.

Port names MUST match `^[A-Za-z][A-Za-z0-9_]*$`, because they become
environment variable names (`PORT_web`). The order of `ports` is the order
tools list them in.

### 5.1 Running

A port *answers* when a TCP connect to `127.0.0.1:<port>` or `[::1]:<port>`
succeeds (dev servers on Node often listen on IPv6 only). Tools SHOULD use a
timeout of about 400 ms, probe ports in parallel and may cache an answer for
a couple of seconds.

### 5.2 Fixed ports

A fixed port is the same in every worktree, so two worktrees cannot both run
the service. Use it for things that are shared on purpose (one database
server for every worktree) or for repositories with a single checkout.

### 5.3 Ports from a state file

A project that hands out its own ports (its setup or dev script picks them
and writes them down) plugs in with a state file:

```json
"state": { "file": "$REPO/.dev/$DIRNAME/state.json" },
"ports": { "frontend": { "state": "frontend_port" }, "api": { "state": "api_port" } }
```

- `state.file` is a template naming a JSON object per worktree, e.g.
  `{"frontend_port": 5181, "api_port": 8011}`. It is required when any port
  uses `state`.
- Each `{ "state": "key" }` port reads that top-level key. Its value MUST be a
  port number or a string holding one; anything else, like a missing key or a
  missing file, means the port is not known yet.
- The tool never writes the state file, and never puts these ports in the
  port store. The project owns them, so no tool can drift from it.

This is the recommended model when a project already allocates ports (it is
the only one that covers ports used by scripts the tools never run). The
port store is for projects that don't.

## 6. Services and groups

### 6.1 Services

A service is a long-running process: a web server, an API, a worker, a
database.

```json
"services": {
  "api": {
    "dir": "services/api",
    "run": ["go", "run", "./cmd/api"],
    "ports": ["api", "api_grpc"],
    "needs": ["db"],
    "ready": { "http": "/healthz", "timeout": "90s" },
    "env": { "DATABASE_URL": "postgres://dev@localhost:$PORT_db/shop" },
    "log": ".dev/logs/api.log"
  }
}
```

| Field | Meaning |
|---|---|
| `title` | Display name; default: the service's name. |
| `dir` | Template; where `run` runs. Default: the worktree. |
| `run` | The command that runs the service in the foreground (string, argv or object, [section 9.1](#91-string-argv-and-object)). It MUST keep running while the service is up; tools run it [detached](#92-background-and-terminal). Without `run`, the service is started by something else (the repository's `dev` command, or the user) and tools only watch it. |
| `stop` | Optional command that stops the service, instead of a signal. |
| `ports` | Names of the ports the service listens on, in order; the first is its main port. Default: the port with the service's own name, if there is one, else none. A port belongs to at most one service. |
| `needs` | A service name or a list of them: services that must be ready before this one starts. |
| `ready` | When the service counts as ready ([below](#readiness)). |
| `env` | Variables for `run` and `stop` (templates). |
| `loadEnv` | `true`: also load `env.files` into the environment. Default `false`. |
| `log` | Template: the file holding the service's log. Default: the shared state folder's log ([section 10.3](#103-logs-and-run-records)). When a tool starts the service it appends the output there; when something else starts it, the tool only reads it. |

Service names MUST match `^[A-Za-z][A-Za-z0-9_]*$` (they become
`DEV_SERVICE` values and often port names) and MUST NOT be `run`, so a
[per-service command map](#72-per-service-commands) cannot be mistaken for a
command object.

`needs` MUST NOT form a cycle, and MUST only name services.

**Ports without a service.** A port that no service lists counts as a
service of its own, with the port's name, no `run`, and ready when the port
answers. So a manifest with only `ports` (like a migrated legacy file) still
has a dot per port, and `needs` can name those ports. A port that has a
service's name MUST be one of that service's ports, so the two never clash.
Wherever this spec says *service*, these count too; they come after the
declared services, in the order of `ports`.

#### Readiness

- No `ready`: the service is ready when its main port answers. A service
  without ports is ready as soon as it is started (its process is alive).
- `"ready": { "port": "api_grpc" }`: when that port (one of the service's)
  answers.
- `"ready": { "http": "/healthz" }`: when `GET http://127.0.0.1:<port><path>`
  (or `[::1]`) returns a status from 200 to 399 within 2 s. `port` picks the
  port (default: the main one). `http` is a template starting with `/`.
- `timeout` (`"60s"` by default; a number with `ms`, `s` or `m`): how long a
  tool waits for readiness while starting things that need the service,
  before it gives up and reports.

A service's *state*, which tools SHOULD show: **ready**; **starting** (a tool
started it, its process is alive, it is not ready yet); **exited** (a tool
started it, the process is gone, it is not ready); **stopped** (none of
these). A service without `run` is ready or stopped.

### 6.2 Groups

```json
"groups": {
  "default": ["web", "api"],
  "backend": ["api", "worker"],
  "all":     ["web", "api", "worker", "storybook"]
}
```

A group is a named list of services. Group names MUST match
`^[A-Za-z][A-Za-z0-9_-]*$`; each list MUST name only services and MUST NOT be
empty.

- The *default group* is the group called `default`, else every service in
  manifest order.
- `dev <group>` starts the group's services plus everything they `need`,
  transitively, even outside the group.
- The worktree counts as *running* (one dot, one word in a compact view) when
  every service of the default group is ready.

## 7. Commands

`commands` maps names to things to run:

```json
"commands": {
  "setup":   "pnpm install && scripts/db-setup",
  "test":    { "*": "pnpm -r test", "web": "pnpm --filter web test", "api": ["go", "test", "./..."] },
  "lint":    { "run": "pnpm lint", "loadEnv": true },
  "format":  null,
  "migrate": { "run": ["scripts/db", "migrate"], "needs": "db" },
  "storybook": { "run": "pnpm --filter web storybook", "title": "Storybook", "terminal": false }
}
```

Command names MUST match `^[A-Za-z][A-Za-z0-9_-]*$` and MUST NOT be `up`,
`down`, `start` or `restart` (those are verbs or aliases, [section 8](#8-verbs)).

### 7.1 Standard names

These names have a meaning every tool can rely on. A tool SHOULD offer each
standard command the manifest defines under that name.

| Name | Meaning | Default mode |
|---|---|---|
| `setup` | Make a fresh checkout or worktree ready: dependencies, env files, database. Safe to run again. | terminal |
| `dev` | Start the worktree's dev services in one command (instead of per-service `run`s), [section 8](#8-verbs). | background |
| `stop` | Stop what `dev` started. | background |
| `test` | Run the tests. | terminal |
| `lint` | Lint and type-check. | terminal |
| `format` | Format the code in place. | terminal |
| `migrate` | Apply database migrations. | terminal |
| `seed` | Load development data. | terminal |
| `open` | Open the app; overrides the default, which opens the first [link](#11-links). | background |
| `teardown` | Free what the worktree holds outside itself (databases, containers, volumes) before it is removed. | terminal |

Any other name is a free-form command (`storybook`, `proxy-start`, `console`).
Tools SHOULD list free-form commands by `title`, else name, in manifest order.

`dev` and `stop` MUST be one repository-wide command (a string, argv or
object), not a per-service map: per-service starting and stopping is what
`services` is for.

### 7.2 Values

| Value | Meaning |
|---|---|
| `null` | The repository has no such command. Tools MUST NOT offer it, and detection MUST NOT fill it in or report it as [missing](#124-drift). |
| a string, an array, or an object with `run` | One repository-wide command ([section 9.1](#91-string-argv-and-object)). |
| an object without `run` | A *per-service map*: keys are service names (or `*`), values are commands (string, array or object). |

#### Per-service commands

With a map, `test api` runs the `api` entry. Plain `test`:

- runs the `*` entry when there is one;
- else runs every entry, in manifest order, one after another, each in its
  own folder. It goes on after one fails, and fails if any failed, so one
  run reports every service.

An entry's `dir` defaults to its service's `dir`, and it runs with that
service's `env` and `DEV_SERVICE`. The `*` entry's `dir` defaults to the
worktree.

### 7.3 Command options

An object with `run` takes these options:

| Option | Meaning |
|---|---|
| `run` | Required: a string or an argv. |
| `dir` | Template; where it runs. Default: the worktree (for a per-service entry, its service's `dir`). |
| `env` | Variables (templates) for this command. |
| `loadEnv` | `true`: also load `env.files`. Default `false`. |
| `terminal` | `true`: run where the user sees it and can type; `false`: run in the background with a log ([section 9.2](#92-background-and-terminal)). Default from the table above; `true` for free-form names. |
| `needs` | A service or list of services that must be ready first. A tool MUST NOT run the command before they are; it MAY start them first, as `start` would, or say what is missing. |
| `title`, `description` | For menus and help. |

## 8. Verbs

Verbs are what a user asks a tool to do. Tools SHOULD use these names in
their command lines and menus.

| Verb | Alias | What it does |
|---|---|---|
| `dev [group]` | `up` | Starts a group (default: the default group). If `commands.dev` exists, it runs that once, in the background, with `DEV_GROUP` set; services' `run`s are not used. Else it starts each service of the group with a `run`, plus their `needs`, in dependency order. |
| `start <service>` | | Starts one service (its `run`), after its `needs`. |
| `stop [service]` | `down` | `stop <service>`: runs the service's `stop` and waits for it, else signals the process a tool started for it. Plain `stop`: runs `commands.stop` if there is one and waits for it to end, then stops every process that still has a [run record](#103-logs-and-run-records) in this worktree (services and `dev` commands), dependents before what they need. |
| `restart [service]` | | `stop`, and once everything it stopped is gone, `dev` (or `stop <service>`, then `start <service>`). |
| `<command> [service]` | | Runs a command ([section 7](#7-commands)): `test`, `test web`, `migrate`, `storybook`. |
| `open` | | Runs `commands.open`, else opens the first link whose `needs` are ready. |

Rules:

- **Already running.** `dev` and `start` skip a service that is ready or
  starting. `dev <group>` does not run `commands.dev` again while the process
  it started for that same group is alive, or while every service of that
  group is ready. They say what they skipped. A `dev` for another group runs
  `commands.dev` again, with that `DEV_GROUP`; the script decides what that
  means.
- **Order.** A service starts only after every service it `needs` is ready.
  Services whose needs are met MAY start in parallel. If a needed service
  fails (its process exits, or it is not ready within its `timeout`), the
  services that need it are not started, and the tool reports which one
  failed and where its log is.
- **A name for `dev`.** `dev <name>` with no such group but a service of that
  name starts that service, as `start` would.
- **A service without `run`.** `start` refuses it and says it is started by
  `dev` (or by hand). Tools still show its state and log.
- **Stopping a process.** Send `SIGTERM` to its process group, wait up to 10
  s, then `SIGKILL` the group. Remove the run record once the process is gone.
  A stop is finished when every process it signalled is gone, and a
  `commands.stop` or service `stop` when it has ended (after at most 60 s,
  when the tool reports it as hanging).
- `stop` never frees the worktree's ports; only [freeing a
  worktree](#104-freeing-a-worktree) does.

## 9. How commands run

### 9.1 String, argv and object

- **A string** runs as `/bin/sh -c <string>` after template expansion.
- **An array** is an argv: each element is expanded on its own and passed as
  it is, with no shell and no word splitting. An argv[0] containing a `/` is
  a path relative to the command's `dir`; otherwise it is looked up in `PATH`
  (with the manifest's `path` in front).
- **An object** has the argv or string in `run`, plus the options of
  [section 7.3](#73-command-options).

### 9.2 Background and terminal

**Background** (`terminal: false`; service `run`s, `dev`, `stop`):

- The tool starts the process in its own session and process group (`setsid`),
  with stdin from `/dev/null` and stdout and stderr appended to its log, and
  does not wait for it: it MUST keep running when the tool quits.
- Before starting, the tool appends a line to the log:
  `# <RFC 3339 time> <tool> start <name>: <command>`.
- For a service's `run` and for `commands.dev`, which keep running, the tool
  writes a [run record](#103-logs-and-run-records), so any tool can see, show
  and stop it.
- Every other background command (`stop`, a service's `stop`, `open`, a
  background free-form command) is expected to end. It gets a log but no run
  record, so a plain `stop` never signals it, and the tool reports its exit
  status when it ends. It SHOULD NOT block the tool's UI while it runs. A
  free-form command that starts a daemon (`proxy-start`) is still one that
  ends: the daemon is the script's business, and a matching command
  (`proxy-stop`) stops it.

**Terminal** (`terminal: true`):

- The tool runs the command where the user sees its output and can answer
  prompts: herdr-deck opens a herdr pane in the worktree; other tools a
  terminal window or tab. The working folder and environment are set as for
  background commands.
- No log or run record is kept: the terminal is the record.

### 9.3 Environment

A command's environment is built from these, each later one winning:

1. The tool's own environment, without variables that describe the tool's own
   terminal or pane (herdr-deck removes `HERDR_PANE_ID`).
2. `PATH` with the manifest's `path` in front.
3. `env.files`, when `loadEnv` is true (the last file setting a key wins).
4. `env.vars`.
5. The service's `env` (for a service, or a per-service command entry).
6. The command's `env`.
7. Injected variables, which always win:

| Variable | Value |
|---|---|
| `DEV_WORKTREE`, `DEV_REPO`, `DEV_DIRNAME`, `DEV_BRANCH` | As the [variables](#3-variables) of the same name. |
| `DEV_SERVICE` | The service, for a service `run`/`stop` and a per-service command entry. |
| `DEV_GROUP` | The group, for `commands.dev`. |
| `DEV_TOOL` | The tool running it, e.g. `herdr-deck`. |
| `PORT_<name>` | Every port whose number is known. |
| `PORT` | For a service (or a per-service entry): the service's main port. Otherwise: the manifest's only port, when it has exactly one. Else unset. |

`PORT` follows the twelve-factor habit most dev servers already understand,
so `"run": "npm run dev"` often needs no port flag at all.

## 10. The shared state folder and the port store

### 10.1 The state folder

Every tool shares one folder for state that is not the project's:

```
$XDG_STATE_HOME/dev-manifest/     (else ~/.local/state/dev-manifest/, on macOS too)
  lock                            the lock for everything below
  ports.json                      the port store
  runs/<key>/<name>.json          run records
  logs/<key>/<name>.log           logs
```

`<key>` names a worktree on disk: its `$DIRNAME`, a `-`, and the first 8 hex
digits of the SHA-256 of its absolute path, e.g. `abc-123-cart-3f9a1c0e`
(characters outside `A-Za-z0-9._-` in the dirname become `_`). `<name>`
says what ran, in a form that cannot clash, since no service, group or
command name contains a `.`:

| What | `<name>` |
|---|---|
| a service's `run` | `service.<service>` |
| `commands.dev` for a group | `dev.<group>` |
| any other background command | `command.<command>` (`command.test.web` for a per-service entry) |

**The lock.** A tool MUST hold the lock while it changes `ports.json` or
creates or removes run records, and MUST NOT hold it longer than needed (at
most a few seconds; never across starting a service and waiting for it).

- Take it by creating `lock` exclusively (`O_CREAT|O_EXCL`; Node `fs.openSync(p, "wx")`),
  and write `{"pid": …, "host": "…", "tool": "…", "time": "<RFC 3339>"}` into it.
- Also write a random `token` into it, so each lock's content is unique.
- If it exists, retry every 50 ms for up to 5 s. A lock older than 30 s, or
  one whose `host` is this machine and whose `pid` is not alive, is stale.
  Take a stale lock away atomically, so two tools that both judge it stale
  cannot both end up holding the lock:
  1. read `lock` (content *C*) and judge it stale;
  2. rename `lock` to `lock.<own pid>.<random>` (only one tool's rename
     succeeds; on failure, retry from the start);
  3. read the renamed file: if it is *C*, delete it and retry taking the lock;
     if it is not (a live lock replaced the stale one in between), put it
     back with `link(renamed, "lock")`, which fails rather than overwrite a
     newer lock, delete the renamed name, and retry.
- Release it by removing the file, after checking that it still holds your
  `token`.

`flock` is not used because Node has no binding for it, and the extension
tools that read the manifest include Node ones.

Writes are atomic: write a temporary file in the same folder, then rename it
over the old one. Reading needs no lock.

### 10.2 The port store

`ports.json` holds every port the store has handed out, for every worktree of
every repository, because projects counting up from different bases
eventually meet.

<!-- example: dev-ports.json -->
```json
{
  "version": 1,
  "worktrees": {
    "/home/sam/src/acme-shop-worktrees/abc-123-cart": {
      "repo": "/home/sam/src/acme-shop",
      "ports": { "web": 3101, "api": 8101 },
      "updated": "2026-10-03T09:12:44Z"
    }
  }
}
```

Worktrees are keyed by their absolute path, symlinks resolved. `repo` is
their main checkout; `updated` when the entry last changed.

**Allocation.** A worktree keeps the number it has for a name as long as
that number is inside `N … N+R-1`, even if something else listens on it now:
a port that moves breaks bookmarks and running processes (the tool MAY warn
instead). When a tool needs a `{ "base": N, "range": R }` port that the
worktree has no number for, or whose number is outside the range (the
manifest changed), it takes the lock, reads the store and:

1. drops the entries whose worktree folder no longer exists;
2. picks the lowest port in `N … N+R-1` that no entry in the store holds
   (any worktree, any name) and that does not answer
   ([section 5.1](#51-running)) right now, probing in parallel;
3. writes the store and releases the lock.

If the range has no free port, allocation fails and the tool MUST say so; it
MUST NOT pick a port outside the range. A name the manifest no longer has
keeps its number until the worktree is freed (another branch may still use
it). A corrupt store is moved aside as `ports.json.corrupt` and the store
starts empty.

A tool MAY allocate as soon as it needs a worktree's ports, also just to show
links, or only when it starts something. Reading assignments needs no lock.

### 10.3 Logs and run records

A run record says a tool started a long-running background process (a
service's `run` or `commands.dev`), so every tool can show it and stop it:

```json
{
  "version": 1,
  "worktree": "/home/sam/src/acme-shop-worktrees/abc-123-cart",
  "name": "service.api",
  "pid": 48213,
  "started": "2026-10-03T09:13:02Z",
  "command": "go run ./cmd/api",
  "dir": "/home/sam/src/acme-shop-worktrees/abc-123-cart/services/api",
  "log": "/home/sam/.local/state/dev-manifest/logs/abc-123-cart-3f9a1c0e/service.api.log",
  "tool": "herdr-deck"
}
```

- `pid` is the leader of the process's own group.
- The record is *alive* while that pid exists, leads its own process group,
  and started within 2 s of `started` (from `ps -o etime=`), so a pid reused
  after a reboot is not mistaken for it. A record that is not alive is stale;
  any tool MAY remove it.
- To start a process, a tool takes the lock, checks that no live record
  exists for that worktree and name, starts the process, writes its record
  with the pid, and only then releases the lock. So two tools cannot start
  the same service twice. Waiting for readiness happens after the lock is
  released.

A service's log is its `log` field, else `logs/<key>/service.<service>.log`;
`commands.dev`'s is `logs/<key>/dev.<group>.log`, and another background
command's `logs/<key>/command.<command>.log`. The run record's `name` is the
`<name>` above (`service.api`).

### 10.4 Freeing a worktree

When a worktree is going away, a tool that knows it (one that removes
worktrees, or a worktree hook calling it) SHOULD:

1. `stop` everything ([section 8](#8-verbs));
2. run `teardown`, if the manifest has one;
3. under the lock, remove the worktree's store entry, its run records and its
   `runs/<key>` and `logs/<key>` folders.

Entries left behind by a worktree removed some other way are dropped by the
next allocation (step 1 above).

## 11. Links

```json
"links": [
  { "title": "Shop", "url": "http://localhost:$PORT_web" },
  { "title": "API docs", "url": "http://localhost:$PORT_api/docs", "needs": "api" },
  { "title": "Mail catcher", "url": "http://localhost:8025", "needs": ["mail"] }
]
```

- `url` is required, a template. `title` is a template too.
- `needs` names services, or ports, that must be ready (or answer) for the
  link to work; a name that is both is the service. Default: the ports the
  URL uses. A link whose needs are not met SHOULD be shown as down, not
  hidden.
- A link whose URL uses a port that is not known yet is left out until it is.
- Without `links`, a tool MAY show `http://localhost:<port>` for each port of
  each service.
- `open` without a `commands.open` opens the first link whose needs are met.

## 12. Seeding and drift

Detection reads the task runners a repository already has. It is **never a
runtime source**: tools run what the manifest says, never what detection
guesses. Detection does two things: it writes a first manifest (*seeding*),
and it warns when the manifest and the repository have drifted apart.

### 12.1 Sources

Detection reads these files in the worktree (and, for package.json, in the
workspace packages it lists), in this order of priority. A source that
appears earlier wins when several offer the same verb, because dedicated
task runners usually wrap the others.

| Source | Tasks | Invocation |
|---|---|---|
| `mise.toml`, `.mise.toml` | `[tasks.<name>]`, and executable files in `mise-tasks/` and `.mise/tasks/` | `mise run <name>` |
| `justfile`, `.justfile`, `Justfile` | recipe lines (`^@?[A-Za-z0-9_-]+( [^:=]*)?:` not followed by `=`), not private (`_x`, `[private]`) | `just <name>` |
| `Taskfile.yml`, `Taskfile.yaml` | keys of `tasks:` | `task <name>` |
| `package.json` | keys of `scripts`, except npm's install lifecycle (`preinstall`, `install`, `postinstall`, `prepare`, `prepublish*`, `prepack`, `postpack`) and `pre*`/`post*` hooks of other scripts | `<pm> run <name>` |
| `Makefile`, `makefile`, `GNUmakefile` | target lines (`^[A-Za-z0-9][A-Za-z0-9_.-]*:` not followed by `=`), not pattern rules or special targets | `make <name>` |
| `script/<name>`, `bin/setup`, `bin/dev` | executable files | the path |
| `Procfile.dev`, else `Procfile` | `name: command` lines | the command |
| `.devcontainer/devcontainer.json`, `.devcontainer.json` | `postCreateCommand`; `forwardPorts` with `portsAttributes.label` | the command |

`<pm>` is the `packageManager` field's tool, else the one whose lockfile
exists (`pnpm-lock.yaml` → pnpm, `yarn.lock` → yarn, `bun.lock` or
`bun.lockb` → bun), else npm. Workspace packages are the folders matched by
`workspaces` (or `pnpm-workspace.yaml`'s `packages`).

Detection MUST NOT run any of the repository's code. It parses files; it MAY
call a runner's own listing (`just --dump --dump-format json`) only when that
listing is known not to evaluate the repository's code. It MUST NOT block a
tool's UI, and SHOULD give up on a source after 5 s.

### 12.2 Verb names

A task maps to a verb when its name, lowercased, is one of the verb's local
names; the first name in the list wins.

| Verb | Local names |
|---|---|
| `setup` | `setup`, `bootstrap`, `install` (not in package.json); `script/bootstrap`, `script/setup`, `bin/setup`; devcontainer `postCreateCommand` |
| `dev` | `dev`, `start`, `serve`, `server`, `up`; `script/server`, `bin/dev`; Procfile entries |
| `stop` | `stop`, `down` |
| `test` | `test`, `tests`, `check` (Makefile only); `script/test` |
| `lint` | `lint`, `typecheck`, `type-check` |
| `format` | `format`, `fmt` |
| `migrate` | `migrate`, `db:migrate`, `db-migrate` |
| `seed` | `seed`, `db:seed`, `db-seed` |
| `teardown` | `teardown` |

`start` maps to `dev` only when the same source has no `dev`, since a
package's `start` is often its production server.

### 12.3 Seeding

A tool that offers seeding (herdr-deck: `herdr-deck dev init`) writes a first
`.config/dev.json`:

- It MUST NOT overwrite an existing `.config/dev.json` unless asked to
  (`--force`), and SHOULD offer to print the result instead of writing it.
- With a legacy `.herdr-deck/dev.json`, it converts that file
  ([section 15](#15-migrating-from-herdr-deckdevjson)) and adds what
  detection finds for commands the legacy file lacks.
- Each verb found becomes `commands.<verb>`, from the highest-priority source.
- Services:
  - each Procfile entry becomes a service (`name`, `run`);
  - else each workspace package with a `dev` (or `start`) script becomes a
    service named after its folder (made a valid name), with `dir` and `run`;
    other per-package verbs become [per-service maps](#72-per-service-commands),
    with a `*` entry when the root has the same verb;
  - else a root `dev` task becomes a service `app` with that `run`, rather
    than `commands.dev`, so it gets per-service start, stop and logs.
- Ports: devcontainer `forwardPorts` become fixed ports, named after their
  label (made a valid name) or `port<N>`. Detection cannot know other ports;
  the tool says which services have none, and that a `{ "base": N }` port
  gives each worktree its own (the service gets it in `PORT`; in a shell
  string write `$$PORT`, or `$PORT_<name>`).
- One link per port of each service, `http://localhost:$PORT_<name>`.
- It prints what it chose and from where, and the alternatives it skipped.

### 12.4 Drift

Drift is a difference between the manifest and the repository that probably
needs an edit. Each kind has an ID, which the warning shows:

| ID | When |
|---|---|
| `missing:<verb>` | Detection finds a task for a standard verb, but `commands` has no entry for it (not even `null`). For `dev`: no `commands.dev` and no service with `run`. `stop` and `open` are never missing (tools stop by signal and open links). |
| `missing:<verb>.<service>` | `commands.<verb>` is a per-service map without `*` and without an entry for a service whose `dir` has a task for that verb. |
| `dangling:<pointer>` | Something the manifest points at no longer exists: a `dir`; a `run` whose argv[0] (or a string's first word) contains `/` and names no file; a `run` that calls a task runner target (`npm`/`pnpm`/`yarn`/`bun run <x>`, `npm test`, `npm start`, `just <x>`, `task <x>`, `mise run <x>`, `make <x>`) that the runner's file in that folder does not define. `<pointer>` is the field's dotted path: `commands.test.web`, `services.api.run`, `services.api.dir`. |

Only these are drift. A command the checker cannot read (a string whose first
word holds a `$`, quotes, or anything it does not recognise) is never
dangling. A detected free-form task with no manifest entry is not drift.

Tools SHOULD check drift when the manifest or a source file changes, not on
every refresh, and show warnings where they show other diagnostics
(herdr-deck: the `!` sources view). Drift never changes what a tool runs.

**Silencing.** `commands.<verb>: null` says the repository has no such
command, which silences `missing:<verb>`. For anything else, list the IDs:

```json
"detect": {
  "ignore": ["dangling:commands.test.api", "missing:lint.*"]
}
```

`*` in an ID matches any characters. `"drift": false` turns drift checks off.

## 13. Tool extensions

Keys starting with `x-` hold one tool's settings, in any object of the
manifest that lists them in the schema (the top level, ports in object form,
services, commands in object form, links, `ready`, `state`, `env`, `detect`):

```json
"x-herdr-deck": { "dev_column": ["web", "api"] },
"links": [
  { "title": "Shop", "url": "http://localhost:$PORT_web", "x-mytool": { "key": "cmd+s" } }
]
```

- A tool uses `x-<its name>` and MUST ignore every other `x-` key.
- Settings that only make sense to one tool belong there: keyboard shortcuts,
  issue tracker teams, worktree folders, worktree creation and removal
  commands, database helpers.
- A tool MUST NOT use `x-` keys to change the meaning of standard fields.
- When two tools need the same extension, it should become a standard field
  in a later revision.

## 14. Examples

Every example below is checked against the schema by `go test ./schema`.

### 14.1 One service

A site whose dev server reads `PORT`. Each worktree gets its own port from
3100 up.

<!-- example: dev.json -->
```json
{
  "$schema": "https://raw.githubusercontent.com/FredricW/herdr-deck/main/schema/v1/dev.schema.json",
  "version": 1,
  "ports": { "web": { "base": 3100 } },
  "services": { "web": { "run": "npm run dev" } },
  "commands": {
    "setup": "npm install",
    "test": "npm test",
    "lint": "npm run lint",
    "format": "npm run format"
  },
  "links": [{ "title": "Site", "url": "http://localhost:$PORT_web" }]
}
```

`dev` starts `web` with `PORT` and `PORT_web` set; `stop` signals it; `open`
opens the site once the port answers.

### 14.2 A monorepo

A web app, an API with a gRPC port, a worker and a Postgres per worktree.

<!-- example: dev.json -->
```json
{
  "$schema": "https://raw.githubusercontent.com/FredricW/herdr-deck/main/schema/v1/dev.schema.json",
  "version": 1,
  "name": "Acme shop",
  "env": {
    "files": ["$REPO/.env", ".env.local"],
    "vars": { "APP_ENV": "development" }
  },
  "path": ["node_modules/.bin"],
  "ports": {
    "web": { "base": 3100 },
    "api": { "base": 8100 },
    "api_grpc": { "base": 9100 },
    "db": { "base": 54300 },
    "storybook": 6006
  },
  "services": {
    "db": {
      "run": ["docker", "compose", "-p", "shop-$DIRNAME", "up", "db"],
      "stop": ["docker", "compose", "-p", "shop-$DIRNAME", "stop", "db"],
      "env": { "DB_PORT": "$PORT_db" }
    },
    "api": {
      "dir": "services/api",
      "run": ["go", "run", "./cmd/api"],
      "ports": ["api", "api_grpc"],
      "needs": "db",
      "ready": { "http": "/healthz", "timeout": "90s" },
      "env": { "DATABASE_URL": "postgres://dev@localhost:$PORT_db/shop" },
      "loadEnv": true
    },
    "worker": {
      "dir": "services/api",
      "run": ["go", "run", "./cmd/worker"],
      "needs": ["db"],
      "env": { "DATABASE_URL": "postgres://dev@localhost:$PORT_db/shop" }
    },
    "web": {
      "dir": "apps/web",
      "run": "pnpm dev",
      "needs": "api",
      "env": { "API_URL": "http://localhost:$PORT_api" }
    },
    "storybook": {
      "title": "Storybook",
      "dir": "apps/web",
      "run": "pnpm storybook --ci --port $$PORT"
    }
  },
  "groups": {
    "default": ["web", "api"],
    "backend": ["api", "worker"],
    "all": ["web", "api", "worker", "storybook"]
  },
  "commands": {
    "setup": { "run": "pnpm install && scripts/db-setup", "needs": "db" },
    "test": {
      "web": "pnpm test",
      "api": ["go", "test", "./..."]
    },
    "lint": { "*": "pnpm -r lint && (cd services/api && go vet ./...)", "web": "pnpm lint", "api": ["go", "vet", "./..."] },
    "format": "pnpm format && gofmt -w services",
    "migrate": { "run": ["scripts/db", "migrate"], "needs": "db", "loadEnv": true },
    "seed": { "run": ["scripts/db", "seed"], "needs": "db" },
    "teardown": ["docker", "compose", "-p", "shop-$DIRNAME", "down", "--volumes"],
    "proxy-start": { "title": "Start auth proxy", "run": ["scripts/auth-proxy", "start"], "needs": "api", "terminal": false },
    "proxy-stop": { "title": "Stop auth proxy", "run": ["scripts/auth-proxy", "stop"], "terminal": false }
  },
  "links": [
    { "title": "Shop", "url": "http://localhost:$PORT_web" },
    { "title": "API docs", "url": "http://localhost:$PORT_api/docs" },
    { "title": "Storybook", "url": "http://localhost:$PORT_storybook", "needs": "storybook" }
  ],
  "detect": { "ignore": ["missing:test.storybook"] },
  "x-herdr-deck": { "dev_column": ["web", "api"] }
}
```

- The default group is `web` and `api`, so `dev` starts `db` (which `api`
  needs), waits for port `db`, starts `api`, waits for `/healthz`, then starts
  `web`. `dev backend` starts `db`, `api` and `worker`; `dev all` everything.
- `start storybook` starts only Storybook; `stop web` stops only the web app;
  plain `stop` stops all of it, `web` first and `db` last.
- `test` runs the web tests, then the API tests, and fails if either failed;
  `test api` runs one. `lint` runs its `*` entry.

### 14.3 The project allocates its ports

A repository whose own script starts everything and writes the ports it
picked. Tools only read them, and point at the logs the script writes.

<!-- example: dev.json -->
```json
{
  "version": 1,
  "state": { "file": "$REPO/.dev/$DIRNAME/state.json" },
  "ports": {
    "frontend": { "state": "frontend_port" },
    "api": { "state": "api_port" },
    "docs": 6060
  },
  "services": {
    "api": { "log": "$REPO/.dev/$DIRNAME/logs/api.log" },
    "frontend": { "log": "$REPO/.dev/$DIRNAME/logs/frontend.log" },
    "worker": { "log": "$REPO/.dev/$DIRNAME/logs/worker.log" }
  },
  "groups": { "default": ["api", "frontend"] },
  "commands": {
    "setup": ["scripts/dev", "new", "$DIRNAME"],
    "dev": ["scripts/dev", "up", "--name", "$DIRNAME"],
    "stop": ["scripts/dev", "down", "--name", "$DIRNAME"],
    "migrate": { "run": ["scripts/dev", "migrate"], "loadEnv": true },
    "seed": ["scripts/dev", "seed"],
    "teardown": ["scripts/dev", "rm", "$DIRNAME"]
  },
  "env": { "files": ["$REPO/.env", ".env.worktree"] },
  "links": [
    { "title": "Frontend", "url": "http://localhost:$PORT_frontend" },
    { "title": "API docs", "url": "http://localhost:$PORT_api/docs" },
    { "title": "Queue dashboard", "url": "https://queue.$env(DEV_DOMAIN|example.test)" }
  ]
}
```

`dev` runs `scripts/dev up` once in the background; the worktree is running
when `api` and `frontend` answer. `worker` has no port, so it shows only its
log. `start api` is refused, because `api` has no `run`.

## 15. Migrating from `.herdr-deck/dev.json`

| `.herdr-deck/dev.json` | `.config/dev.json` |
|---|---|
| (no version) | `"version": 1` |
| `state.file`, relative to the main checkout | `state.file` with `$REPO/` in front (relative paths are now relative to the worktree) |
| `state.ports.<name>: "<key>"` | `ports.<name>: { "state": "<key>" }` |
| `state.ports.<name>: <number>` | `ports.<name>: <number>` |
| the order of `state.ports` | the order of `ports` |
| `links[]` (`title`, `url`, `needs`) | `links[]`, unchanged |
| `up` (a shell string) | `commands.dev`, the same string |
| — | `commands.stop`: until it exists, `stop` signals the process group `dev` started, as herdr-deck could already |
| `$DIRNAME`, `$BRANCH`, `$WORKTREE`, `$REPO`, `$PORT_<name>`, `$$`, `${…}` | unchanged |
| log in herdr-deck's own state folder (`<slug>-<thread>.log`) | `dev-manifest/logs/<key>/dev.default.log`, shared |

For example:

```
{"state": {"file": ".dev/$DIRNAME/state.json",
           "ports": {"frontend": "frontend_port", "docs": 6060}},
 "up": "scripts/dev-up --name $DIRNAME"}
```

becomes

<!-- example: dev.json -->
```json
{
  "version": 1,
  "state": { "file": "$REPO/.dev/$DIRNAME/state.json" },
  "ports": { "frontend": { "state": "frontend_port" }, "docs": 6060 },
  "commands": { "dev": "scripts/dev-up --name $DIRNAME" }
}
```

Its ports have no service, so each counts as its own service for `needs` and
the dot display: a port that no service lists is shown, and needed, on its
own.

Other tools with a manifest of their own move to this file the same way:
shared fields to their standard places, the rest under `x-<tool>`. During the
move a tool MAY read its old file after `.config/dev.json`, never before it.

## 16. Other tools' files (non-normative)

Coding-agent and worktree tools each have their own file for setup, run
commands and ports (Conductor, Codex environments, `.claude/launch.json`,
Superset, worktrunk). A later revision may let detection read them as
[seeding and drift](#12-seeding-and-drift) sources, but only for tools that
publish a specification of their format, so detection does not depend on
guessing another tool's private format. They would never be runtime sources,
for the same reason as the task runners above.

## 17. Decisions

The user settled the draft's open questions on 2026-10-04:

1. **Strict `$` in shell strings** stays: an unknown `$NAME` is an error, and
   shell commands write `$$HOME`.
2. **`$env(…)` reads only `env.files`**, never the tool's own environment.
3. **Relative paths are relative to the worktree**, `state.file` included;
   migrations add `$REPO/`.
4. **The shared folder** is `~/.local/state/dev-manifest/` (or under
   `$XDG_STATE_HOME`) on every platform, macOS included. A tool with a port
   store of its own imports its assignments once.
5. **Port store defaults**: a range of 100 per name, failing when full;
   entries are dropped once their folder is gone.
6. **Plain `test` with a per-service map** goes on after a failure and fails
   at the end.
7. **`dev` with `commands.dev`** runs only that command, not the services'
   `run`s.
8. **Verbs**: `dev`/`stop` with `up`/`down` as aliases, plus
   `start <service>` and `restart`.
9. **Service names have no dashes**, since they become environment variable
   names.
10. **The schema URL is versioned**: `schema/v1/…`, changed only compatibly;
    a breaking change gets `v2` ([section 2.1](#21-file)). A tag-pinned URL
    can be used once a release contains the schema.
11. **Worktree lifecycle** (creating and removing worktrees) stays out of the
    shared file, under `x-<tool>`.
