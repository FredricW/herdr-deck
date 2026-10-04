---
name: dev-manifest
description: Writes or updates a repository's dev manifest, `.config/dev.json` (services, ports, groups, standard commands, links), from what the repo already has, without running its code, and validates it against schema/v1. Use when asked to set up, configure, check or migrate a dev manifest or `.config/dev.json` for one repo, a list of repos or a folder of them, including migrating a legacy `.herdr-deck/dev.json`.
---

# Dev manifest

A dev manifest is the committed, tool-neutral file `.config/dev.json` that
says how to work a checkout: its services and their ports, the standard
commands (`setup`, `dev`, `stop`, `test`, …) and the pages to open. The
**spec** is the single source of truth for every field and rule:

- `../../docs/dev-manifest.md`, relative to this skill's base directory, when
  the skill is linked from a herdr-deck checkout;
- else <https://github.com/FredricW/herdr-deck/blob/main/docs/dev-manifest.md>.

Read the spec's sections 3 to 7, 11, 12 and 15 before the first repo. Its
examples (section 14) are the shapes to copy.

The manifest **describes**; it points at the scripts and task runners the
repo already has. You are a reader here: configuring never runs the repo's
dev servers, migrations, seeds, installs or scripts. Parse files instead.

## Prefer the deck's own tools

Run `herdr-deck dev --help` first. When it lists `init` (seeding) or a drift
check, use them: `herdr-deck dev init` (print, don't write, unless asked)
gives the draft, and the drift check adds to the validator in step 4. Then
review their output against steps 2 and 3. Until they exist, do every step by
hand.

## Steps, per repo

Work through the repos **one at a time**: finish a repo's steps before
opening the next. For a folder, the repos are its immediate subfolders that
hold a `.git`.

1. **Inspect.** Go through every source in [INSPECT.md](INSPECT.md) and note
   what each one offers: tasks per verb, services, ports, env files, layout.
   Done when every source in its checklist is marked found or absent.
2. **Map** each finding onto the spec, using its detection rules (section
   12.1 to 12.3) and the extra rules in INSPECT.md:
   - **Services** for long-running processes (`dir`, `run`, `ports`,
     `needs`, `ready`, `env`, `log`); **groups** when there are several and
     the everyday set is smaller than all of them (`default`).
   - **Commands**: each standard name the repo has a task for, from the
     highest-priority source; `null` for one the repo deliberately lacks
     (only when you know, e.g. a README saying there are no migrations).
     Other useful tasks become free-form commands.
   - **Ports**: `{ "base": N }` from the port store, one per listening
     service, with the run command made to listen on it. When the repo
     already hands out its own ports through a state file, use `state` ports
     instead (spec 5.3). Fixed ports only for things shared on purpose.
   - **Links**: one per web-facing port, `http://localhost:$PORT_<name>`.
   Prefer the invocation the repo already uses (its `just dev` over the
   `npm run dev` it wraps). Done when every finding is in the draft or on
   the list of open questions.
3. **Write** `<repo>/.config/dev.json`:
   - An existing file: keep every field and every `x-<tool>` key; change
     only what the findings contradict, and add what is missing.
   - A legacy `.herdr-deck/dev.json` and no new file: convert it as spec
     section 15 says, then add the findings it lacks. Leave the legacy file
     in place and say it can be deleted once every tool reads the new one.
   - Start with
     `"$schema": "https://raw.githubusercontent.com/FredricW/herdr-deck/main/schema/v1/dev.schema.json"`
     and `"version": 1`, then the spec's key order: `name`, `env`, `path`,
     `ports`, `state`, `services`, `groups`, `commands`, `links`, `detect`.
     Two-space indent, small objects on one line, as in the spec's examples.
4. **Validate** with the validator next to this skill (step below). Done
   when it prints `ok` and every warning is fixed or explained.
5. **Report** the repo: what you wrote and the source of each choice, then
   the **open questions**, each with your default (see below).

After the last repo, give a summary table: repo, manifest (new / updated /
migrated / unchanged), services, ports, standard commands set, open
questions.

## Validate

The validator checks the schema, the spec's rules the schema cannot express
(variables, `needs`, groups, port ownership, cycles), and, as `dangling:`
warnings, that every `dir`, script path and task-runner target exists. It
reads files only. With the skill linked from a herdr-deck checkout (Go
1.27+), using absolute paths:

```sh
go -C "$(cd <skill base dir> && pwd -P)" run ./validate <repo>/.config/dev.json
```

From a copied skill: `go run github.com/FredricW/herdr-deck/skills/dev-manifest/validate@main <file>`.
`-root <repo>` sets the repo when the file is elsewhere; `-dry=false` skips
the existence checks. Without Go, use the repo's own JSON Schema check if it
has one, and ask before installing anything.

## Open questions

Ask what the files cannot tell, with the default you chose, e.g.:

- a service whose port you could not find, or a dev server that ignores
  `PORT` and needs a flag you are unsure of;
- whether a database runs per worktree (own port, compose project per
  `$DIRNAME`) or is shared (fixed port);
- which services belong in the `default` group;
- a verb with two candidate tasks, and which one you picked;
- a `start` script that may be a production server.

Offer one **dry check** of the result: the validator's `dangling:` warnings
are it. Starting anything to see whether it works is the user's call, after
the file is reviewed.
