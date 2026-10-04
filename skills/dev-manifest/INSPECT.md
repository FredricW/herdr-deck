# Inspecting a repo

Reference for steps 1 and 2 of [SKILL.md](SKILL.md). The spec's section 12
(detection sources, verb names, seeding) is the base; this file adds the
checklist and what the spec leaves to judgement. Read files; run nothing.

## Checklist

Mark each one found or absent, at the root and in every workspace package.

| Source | Look for |
|---|---|
| `.config/dev.json` | An existing manifest: keep it, update it. |
| `.herdr-deck/dev.json` | A legacy manifest: migrate it (spec section 15). |
| `mise.toml`, `.mise.toml`, `mise-tasks/`, `.mise/tasks/` | Tasks. |
| `justfile`, `.justfile`, `Justfile` | Recipes. |
| `Taskfile.yml`, `Taskfile.yaml` | Tasks. |
| `package.json` | `scripts`, `workspaces`, `packageManager`; dependencies that name the framework. |
| `pnpm-workspace.yaml`, `turbo.json`, `nx.json`, lockfiles | Workspace packages and the package manager. |
| `Makefile`, `makefile`, `GNUmakefile` | Targets. |
| `script/`, `bin/` | `script/setup`, `script/server`, `script/test`, `bin/setup`, `bin/dev`, other dev helpers. |
| `Procfile.dev`, `Procfile` | One service per entry. |
| `.devcontainer/devcontainer.json`, `.devcontainer.json` | `postCreateCommand`, `forwardPorts` and their labels. |
| `docker-compose*.yml`, `compose*.yaml` | Databases and other services, their `ports:` and whether those use `${VAR:-default}`. |
| `.env.example`, `.env.sample`, `.env*` | Which env files exist and which variables hold ports or URLs. Read names, not secret values. |
| Framework config (`vite.config.*`, `next.config.*`, `astro.config.*`, `angular.json`, `config/puma.rb`, `manage.py`, `mix.exs`, `go.mod`, …) | The dev server and any port it pins. |
| `README`, `CONTRIBUTING`, `docs/` | How the team starts things, which databases are shared, commands the repo deliberately lacks. |
| Scripts that write ports to a file (`state.json`, `.ports`, …) | The repo allocates its own ports: use `state` ports. |

## Mapping rules beyond the spec

**Services or `commands.dev`.** Per-service `run`s give per-service start,
stop, logs and readiness, so prefer them whenever each service starts on its
own. Use `commands.dev` only when the repo's own script must start
everything together (it allocates ports, or orchestrates containers); with
`commands.dev` set, `dev` runs only it and ignores the services' `run`s
(spec section 8). Services then have no `run` and only describe ports and
logs, as in spec example 14.3.

**Monorepos.** Each workspace package with a `dev` (or `start`, see below)
script is a service named after its folder (`apps/admin-web` → `admin_web`:
service names have no dashes). Run it in its folder (`dir`) with the
package manager's `run dev`, rather than a root `--filter` call. Per-package
`test`/`lint` become per-service maps, with a `*` entry when the root has the
same task. Databases from compose are services too. `needs` follows the
wiring: a web app whose env points at the API needs `api`; an API with a
`DATABASE_URL` needs `db`. The `default` group is what a developer starts
every day; leave occasional services (Storybook, docs, workers) out of it.

**`start` scripts** are often production servers (`node dist/server.js`,
`next start`). Map `start` to `dev` only when the package has no `dev` and
the script really is a dev server; ask otherwise.

**Ports.** One `{ "base": N }` port per listening service, named after the
service, with `N` the framework's default from the table below (or the port
the repo's docs use). The store hands each worktree the lowest free port
from `N` up, so the main checkout usually keeps the familiar number. Then
make sure the service listens on it:

- tools set `PORT` (the service's main port) and `PORT_<name>`;
- a server that reads `PORT` needs nothing;
- otherwise pass it in `run` as a flag, `$$PORT` in a shell string, or
  `$PORT_<name>` in an argv or a string.

Use fixed ports only for something deliberately shared by every worktree
(one local Postgres for all of them), and say so. When the repo writes its
own ports to a state file, use `state` ports and its log files, never the
store.

**Databases from docker compose.** When the compose file reads its host port
from a variable (`"${DB_PORT:-5432}:5432"`), give it a `base` port and run it
per worktree: `"run": ["docker", "compose", "-p", "<repo>-$DIRNAME", "up", "db"]`,
`"stop"` with `stop db`, `"env": { "DB_PORT": "$PORT_db" }`, and offer a
`teardown` with `down --volumes` (it deletes the worktree's data: ask). When
the port is hard-coded, use a fixed port and ask whether the database is
meant to be shared; changing the compose file is the user's call.

**Readiness.** Default is the main port answering. Add `"ready": { "http": "/healthz" }`
when the service has a health route (search its routes), and a longer
`timeout` for slow starts (compiling, containers).

**Env.** List env files the tools should read in `env.files` (`$REPO/.env`
for one shared by every worktree, `.env.local` for a worktree's own) and
set `loadEnv` only on commands that do not load them themselves. Values the
services need from each other go in `env` with port variables
(`"API_URL": "http://localhost:$PORT_api"`). Secrets stay in the env files:
the manifest only names those files.

**Shell strings.** Every `$` that is not a manifest variable is written `$$`
(`$$HOME`, `$$PORT`). Prefer an argv when the command has no shell syntax.

**Null.** `"lint": null` says the repo has no linter on purpose and silences
drift. Write it only when you know; an unknown stays absent and becomes a
question.

## Dev servers

| Framework | Command | Default port | Takes the port from |
|---|---|---|---|
| Next.js | `next dev` | 3000 | `PORT` |
| Nuxt | `nuxi dev` | 3000 | `PORT` |
| Create React App | `react-scripts start` | 3000 | `PORT` |
| Vite (React, Vue, Svelte, SvelteKit, Solid, Remix on Vite) | `vite` | 5173 | `--port $$PORT --strictPort` |
| Astro | `astro dev` | 4321 | `--port $$PORT` |
| Angular | `ng serve` | 4200 | `--port $$PORT` |
| Storybook | `storybook dev` | 6006 | `-p $$PORT --ci` |
| Express, Fastify, Hono, Koa | the repo's own | varies | usually `process.env.PORT`; check the code |
| Rails | `bin/rails server`, `bin/dev` | 3000 | `PORT` (puma) |
| Django | `manage.py runserver` | 8000 | an argument: `runserver 127.0.0.1:$$PORT` |
| Flask | `flask run` | 5000 | `--port $$PORT` |
| FastAPI, Starlette | `uvicorn app:app --reload` | 8000 | `--port $$PORT` |
| Phoenix | `mix phx.server` | 4000 | `PORT` in generated `config/dev.exs`; check |
| Go | the repo's own | varies | check for `os.Getenv("PORT")` or a flag |
| Hugo | `hugo server` | 1313 | `--port $$PORT` |
| Postgres, MySQL, Redis, Mailpit | compose | 5432, 3306, 6379, 8025 (UI) | the compose port mapping |

When a script wraps the server (`"dev": "vite"`), pass the flag through the
runner: `"run": "npm run dev -- --port $$PORT --strictPort"`.
