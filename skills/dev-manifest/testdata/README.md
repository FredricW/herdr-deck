# Fixture repos

Made-up repos the skill was run on by hand. Each `.config/dev.json` is the
skill's output; `go test ./skills/...` checks that it validates and that
everything it points at exists. The questions are those the run asked.

| Repo | What it tests | Manifest | Open questions |
|---|---|---|---|
| `node-app` | One Vite app on npm | new: service `web` on a `base` 5173 port, passed as `--port $$PORT --strictPort` since Vite ignores `PORT`; `setup` from the lockfile; `test`, `lint`, `format` from scripts | Is `npm install` the right setup, or `npm ci`? |
| `monorepo` | pnpm workspaces (`apps/web` Next.js, `apps/api` Express), Postgres in compose, a justfile | new: services `db` (compose, port from `DB_PORT`), `api` (reads `PORT`, `/healthz`), `web` (reads `PORT`, gets `API_URL`); per-service `test`/`lint` with the root's turbo task as `*`; `migrate`/`seed` on `api`; `setup`/`teardown` from the justfile | Root `dev` (`turbo dev`) left out in favour of per-service runs: OK? Each worktree gets its own database (compose's per-folder project plus `DB_PORT`): OK, or one shared? `just teardown` deletes the worktree's data. |
| `legacy` | A `.herdr-deck/dev.json` with a state file and `up` | migrated (spec section 15): `state` ports, `$REPO/` state file, `up` → `commands.dev`; added `stop` from `scripts/dev-down`, `test`/`lint` from the Makefile, a `docs` service from `make docs` on the fixed 6060, logs and a `default` group without `docs` | Are the logs really `.dev/<name>/logs/<service>.log`? Delete `.herdr-deck/dev.json` once every tool reads the new file? |
