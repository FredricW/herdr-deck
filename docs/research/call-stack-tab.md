# Flows tab: did the behaviour of a flow change?

Written 2026-10-07 against herdr-deck at 0937435. This is an exploration
with a throwaway prototype, not a finished feature. The prototype is
[`experimental/callstack`](../../experimental/callstack), a module of its
own that the deck does not build in. Tool, model and paper claims were
checked that day against the sources in [§13](#13-sources).

The mockups use the made-up `admin-rebuild` project of the
[design README](../design/README.md#sample-data): thread t-0002, *Users
page*, in the `webshop` monorepo, here with an invented `apps/api`
backend. All of that code is invented. The prototype's output in
[§11](#11-the-prototype) is real. It comes from herdr-deck's own history,
which is public.

A sibling exploration, the [architecture tab](architecture-tab.md) (did
the shape of the system change?), ran in parallel. That tab looks at
**structure**, which is imports, layers and touchpoints. This one looks at
**behaviour**, which is what each entry point now calls, in what order,
with what effects.

**Recommendation in one paragraph.** Build it, but after the
architecture tab, and Go only at first. A script-only pipeline goes much
further than expected:
- **No AI was needed for any signal.** The prototype is rules plus
  go/types.
- **Cost:** it reads both revisions of herdr-deck, about 1,000 functions,
  in **1.2 s warm** and 4–5 s on a cold Go build cache.
- **What it found on two real merged PRs**, all untouched by any model:
  - a new PR-detail read reaches `gh` only through a callback field;
  - that read stays off the UI goroutine;
  - a 63-frame new flow behind one key;
  - a function rewrite that drops five error paths;
  - on a synthetic commit, a file read added to the render path, which
    blocks the UI in 38 flows.

Two findings shape the design:
1. **Entry points are framework-specific.** In a Bubble Tea app they are
   `Update`'s message cases and key bindings. In a web app they are
   routes.
2. **TUIs, and most apps, have hubs.** A hub is a frame almost every flow
   passes through, such as the layout and render path. Hubs have to be
   shown once, not inside every flow, or the tab drowns.

The tab opens on a risk-ranked **Look here first** list. Its main view is
the **unified call-tree diff** (mockup A), with **who reaches this** (C)
as a per-frame toggle. The sequence diff (B) is left for later.

The hard parts are not graphs or AI:
- dynamic dispatch through callback fields and interfaces;
- stable identities for closures;
- dense rewrites.

TypeScript and Python need a different reader: a language server or a
pure-Go tree-sitter port. That should be its own spike once the user
names the repos. Phases and decisions are in
[§12](#12-recommendation).

Contents:

1. [What the tab answers](#1-what-the-tab-answers)
2. [Which view fits a terminal](#2-which-view-fits-a-terminal)
3. [Mockups](#3-mockups)
4. [Interactions](#4-interactions)
5. [Entry points](#5-entry-points)
6. [The data pipeline](#6-the-data-pipeline)
7. [Languages](#7-languages)
8. [Signals and their rules](#8-signals-and-their-rules)
9. [Fallbacks](#9-fallbacks)
10. [Where AI could help](#10-where-ai-could-help)
11. [The prototype](#11-the-prototype)
12. [Recommendation](#12-recommendation)
13. [Sources](#13-sources)

---

## 1. What the tab answers

A diff shows lines. A reviewer of an agent's branch wants to know which
**user-facing flows** behave differently, and how. The tab starts from the
changed functions and walks up their callers to entry points: routes, CLI
commands, jobs, consumers, and in a TUI, messages and keys. Then it draws
each affected entry point's call tree as a diff.

Per frame (a function in a flow), it shows:

| Signal | Example | Why a diff hides it |
|---|---|---|
| Calls added, removed, reordered | `+ audit.record` | The call is one line; that it now runs on every list request is not |
| Calls moved into or out of a branch, loop, `defer` or goroutine | `~ cache.get  moved into branch` | Indentation changes look like noise |
| Side effects, as icons, gained and lost | `⛁✉ new write` | Effects live two or three frames down |
| Error paths | `− ↯ catch NotFoundError → []` | A removed `catch` or `if err != nil` is easy to miss |
| Cost | `⛁ ↻ N+1`, `blocks UI` | A loop or the UI goroutine is far from the call |
| Blast radius | `3 flows` | How many entry points reach a changed frame |
| Refactors | `≈ toUserRow  was formatUser` | Folded, so they don't hide the real changes |

It is a navigation aid, not a diff replacement. Every row jumps to its
hunk in the deck's diff preview.

## 2. Which view fits a terminal

The notes proposed three views. I checked each against a 60–120-column
pane and against the prototype's real output:

| View | Fits a pane? | Verdict |
|---|---|---|
| **A. Unified call-tree diff** (per entry point, `+ − ~ ≈` marks, icons right-aligned) | Yes. A tree is the one graph shape a terminal draws well. Indentation of two per level, breadcrumbs (`syncPreview › … › scrollPreview`) for unchanged chains, `⋯` for depth. Works at 60 | **Main view** |
| **C. Who reaches this** (inverted tree from one frame up to its entry points) | Yes, same tree shape | **Per-frame toggle** (`r`). It is the blast radius made concrete |
| **B. Sequence diff** (lifelines per module, arrows, `+` rows) | Only at ≥ 100 columns, and only for 4–6 lifelines. Rows are wide, and long flows need horizontal scrolling. It adds little over A once A shows effects and order | **Later, maybe**: an extra view at ≥ 100 columns for one flow |

The tab opens on **Look here first**, a list ranked by risk, as the
architecture tab does. `t` cycles *first → tree* (→ *sequence* at wide
widths, if built).

The prototype showed that a plain per-flow tree is not enough. Two
additions are needed:

1. **Hubs.** In herdr-deck, `ui.Model.layout` is reached from all 39
   flows, because every message relayouts. A change under it showed up in
   every flow tree. A frame reached by at least half the flows (and at
   least six) is a hub:
   - flow trees stop at it with a `◇`, and the flow header says
     `via ◇ layout, curTab`;
   - its own diff is shown once, under *Shared frames*;
   - a flow that reaches the change only through hubs is counted and
     hidden ("6 reach it only through shared frames").
   Web apps have the same thing in middleware and auth wrappers.
2. **Summaries for new code.** A brand-new function's subtree has nothing
   to diff against. It becomes one row:
   `+ ui.Model.stopDev  ▤▦⚙⧗◷ new · 63 frames`. `↵` expands it.

## 3. Mockups

Frames use the 28-row layout of the [drawer designs](../design/drawer/README.md).
The box is not part of the pane. Plain text can't show colour, so as an
aid: `+` rows are green, `−` red, `~` yellow, `≈` and `·` dim. `⚠` is the
warning colour, and the cursor row (`▸`) has the dark-grey background of
every list. The tab is called **Flows**, with the number of rows in
*Look here first* (see [decision 6](#decisions-for-the-user)).

Icons are single-cell glyphs. Emoji are double-width in most terminals
and break alignment:

| Icon | Effect | Icon | Effect |
|---|---|---|---|
| `⛁` | database | `⧗` | lock or wait |
| `⇄` | network | `◷` | sleep |
| `▤` | file read | `✉` | event, message, queue |
| `▦` | file write | `↻` | inside a loop |
| `⚙` | process (exec, signals) | `↯` | error path (return error, raise, panic) |

### 80 columns, normal height: Look here first

The summary line counts what is there. Each row has a glyph, the frame,
the reason, and either its one flow or the number of flows on the right.
`⚠` marks rules that fire on risk: a blocking call in a loop, a removed
error path, a blocking call on a UI goroutine, a new write.

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Admin rebuild                                               ◐ 2  ◇ 1  ○ 1  ✉ 0 │
│────────────────────────────────────────────────────────────────────────────────│
│    WORK                         THREAD  STATUS              PR      LINKS  DEV │
│  In progress                                                                   │
│ ▸◐ Users page                   t-0002  Building overview           L4         │
│  ◐ Templates page               t-0003  Writing tests               L F    ●●● │
│  ◇ Document select for summary  t-0004  review required     #2320   L F3   ●○  │
│                                                                                │
│  On hold until Monday 2026-10-05                                               │
│  ○ Subscriptions list           t-0001  idle                        L N    ~○  │
│                                                                                │
│  + Backlog (3)                                                                 │
│                                                                                │
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ Users page                                                     ◐ working  ~60% │
│ webshop · t-0002-members-admin-users · Building overview            ▰▰▰▰▰▰▱▱▱▱ │
│                                                                                │
│  Overview   Files 11   Commits 4   Flows 5   Log 7                    ↓ 2 more │
│                                                                                │
│ 5 to look at  ⚠ 2  4 flows  +1 new  ≈ 3 refactors               first · t tree │
│▸1 ⚠ countByStatus    ⛁ read in a loop: N+1                    GET /admin/users │
│ 2 ⚠ getUser          catch NotFoundError gone → 500       GET /admin/users/:id │
│ 3 + audit.record     ⛁✉ new write                                      3 flows │
│ 4 ~ listUsers        ⇄ cache.get moved into a branch                   2 flows │
│────────────────────────────────────────────────────────────────────────────────│
│ j k item  ↵ code  t view  r who reaches  m mark  y copy id  esc list           │
└────────────────────────────────────────────────────────────────────────────────┘
```

### 60 columns, normal height

The reason shortens, and the right column falls back to a flow count.

```text
┌────────────────────────────────────────────────────────────┐
│ Admin rebuild                           ◐ 2  ◇ 1  ○ 1  ✉ 0 │
│────────────────────────────────────────────────────────────│
│    WORK                        STATUS           LINKS  DEV │
│  In progress                                               │
│ ▸◐ Users page                  Building overv…  L4         │
│  ◐ Templates page              Writing tests    L F    ●●● │
│  ◇ Document select for summa…  #2320 review     L F3   ●○  │
│                                                            │
│  On hold until Monday 2026-10-05                           │
│  ○ Subscriptions list          idle             L N    ~○  │
│                                                            │
│  + Backlog (3)                                             │
│                                                            │
│                                                            │
│────────────────────────────────────────────────────────────│
│ Users page                                 ◐ working  ~60% │
│ webshop · t-0002-members-admin-users            ▰▰▰▰▰▰▱▱▱▱ │
│                                                            │
│  Overview   Files 11   Flows 5   Log 7            ↓ 2 more │
│                                                            │
│ 5 to look at  ⚠ 2  4 flows                  first · t tree │
│▸1 ⚠ countByStatus  ⛁ in a loop: N+1                 1 flow │
│ 2 ⚠ getUser  catch NotFoundError gone               1 flow │
│ 3 + audit.record  ⛁✉ new write                     3 flows │
│ 4 ~ listUsers  ⇄ moved into a branch               2 flows │
│────────────────────────────────────────────────────────────│
│ j k item  ↵ code  t view  r reach  esc list                │
└────────────────────────────────────────────────────────────┘
```

### 80 columns, full height: the tree

`n` / `N` step through flows (`‹1/4›`). The header shows the flow's
synchronous effects before → after. Unchanged frames that only lead
somewhere fold into breadcrumbs. Pure calls with no effect, such as
string helpers, fold into a `·` count.

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Users page                                                     ◐ working  ~60% │
│ webshop · t-0002-members-admin-users · Building overview            ▰▰▰▰▰▰▱▱▱▱ │
│                                                                                │
│  Overview   Files 11   Commits 4   Flows 5   Log 7                             │
│                                                                                │
│ GET /admin/users  ‹1/4›                               sync ⛁⇄ → ⛁⇄✉   tree · t │
│   apps/api/src/routes/users.ts:12 · via ◇ withAuth                             │
│    ~ listUsers                                                                 │
│      ~ cache.get                                           ⇄ moved into branch │
│      ~ usersRepo.list                                                          │
│        + db.users.countByStatus                                      ⛁ ↻ ⚠ N+1 │
│        · +1 pure call                                                          │
│      ≈ toUserRow  was formatUser                                    same calls │
│▸     + audit.record                                          ⛁✉ new · 2 frames │
│      − ↯ catch NotFoundError → []                              error path gone │
│      · +2 −1 pure calls                                                        │
│                                                                                │
│ GET /admin/users/:id  ‹2/4›                                         sync ⛁ → ⛁ │
│   apps/api/src/routes/users.ts:40                                              │
│    ~ getUser                                                                   │
│      − ↯ catch NotFoundError → 404                                   now → 500 │
│      ~ usersRepo.byId                                                          │
│        ≈ toUserRow  was formatUser                                  same calls │
│────────────────────────────────────────────────────────────────────────────────│
│ j k frame  ↵ code  n N flow  r who reaches  c fold  m mark  esc list           │
└────────────────────────────────────────────────────────────────────────────────┘
```

### Who reaches this (`r` on a frame)

The inverted tree from the selected frame up to every entry point. `◆`
marks an entry point, and changed callers are labelled. Callers from
tests are counted, not listed.

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Who reaches ▸ db.users.countByStatus                          2 flows · r back │
│   apps/api/src/db/users.ts:58 · ⛁ read                                         │
│    ↑ usersRepo.list                                                    changed │
│      ↑ listUsers                                                       changed │
│        ◆ GET /admin/users                                                      │
│      ↑ usersOverview                                                       new │
│        ◆ GET /admin/users/overview                                    new flow │
│    ↑ refreshStats                                                              │
│      ◆ job nightly-user-stats                                      ↻ in a loop │
│    ⋯ 2 more callers in tests                                                   │
│────────────────────────────────────────────────────────────────────────────────│
│ j k frame  ↵ code  r back  esc list                                            │
└────────────────────────────────────────────────────────────────────────────────┘
```

### 120 columns: tree and the selected frame

At 120 columns there is room for the selection's detail beside the tree:
- its file and line;
- its effects in words;
- the hunk that creates the call;
- the flows that reach it;
- its stable ID and verdict.

`↵` opens the diff preview at that hunk.

```text
┌────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│ Users page                                                                                             ◐ working  ~60% │
│ webshop · t-0002-members-admin-users · Building overview                                                    ▰▰▰▰▰▰▱▱▱▱ │
│                                                                                                                        │
│  Overview   Files 11   Commits 4   Flows 5   Log 7                                                                     │
│                                                                                                                        │
│ GET /admin/users  ‹1/4›                        sync ⛁⇄ → ⛁⇄✉ │ + audit.record                              ? unmarked  │
│   routes/users.ts:12 · via ◇ withAuth                        │ apps/api/src/services/audit.ts:12 · new                 │
│    ~ listUsers                                               │ ⛁ db write (audit_log)  ✉ event user.listed             │
│      ~ cache.get                         ⇄ moved into branch │                                                         │
│      ~ usersRepo.list                                        │ @@ services/users.ts:27 @@                              │
│        + db.users.countByStatus                    ⛁ ↻ ⚠ N+1 │    const rows = await usersRepo.list(q);                │
│      ≈ toUserRow  was formatUser                  same calls │  + await audit.record(ctx.user, "user.listed");         │
│▸     + audit.record                        ⛁✉ new · 2 frames │    return rows.map(toUserRow);                          │
│      − ↯ catch NotFoundError → []            error path gone │                                                         │
│      · +2 −1 pure calls                                      │ reached by 3 flows: GET /admin/users, GET …/:id,        │
│                                                              │   job nightly-user-stats                                │
│ GET /admin/users/:id  ‹2/4›                       sync ⛁ → ⛁ │ id frame:GET /admin/users>listUsers>audit.record        │
│────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────│
│ j k frame  ↵ open hunk  d diff tool  n N flow  r who reaches  m mark  y copy id  esc list                              │
└────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

### Sequence diff (120 columns, not recommended for phase 1)

What B would look like. It reads well for one short flow and badly for
anything with more than six modules, which is why it isn't in phase 1.

```text
┌────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│ GET /admin/users  ‹1/4›   sequence                                                                              t view │
│   route            listUsers         cache            usersRepo        db               audit                          │
│     │                 │                │                 │                │                │                           │
│     ├─ listUsers ────▶│                │                 │                │                │                           │
│  ~  │                 ├─ get ─────────▶│  ⇄ moved into branch            │                │                            │
│     │                 ├─ list ─────────────────────────▶│                 │                │                           │
│  +  │                 │                │                 ├─ countByStatus▶│ ⛁ ↻ N+1        │                           │
│  +  │                 ├─ record ──────────────────────────────────────────────────────────▶│ ⛁✉                        │
│  −  │                 ├╌ catch NotFoundError → [] ╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌ │                        │
└────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

### Dense change: a rewrite and a new flow (real prototype data)

A frame whose calls mostly changed (here 56 of 59 rows) is a rewrite, and
an LCS alignment of it is noise. The row says so. Under it go only the
rows that carry an effect, an error path or a new subtree, and `↵` shows
all of them. This is `dev.Reader.Up` from herdr-deck PR #52; the
prototype printed all 59, and this fallback is the fix (§11).

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ handleKey ▸ key DevUp  ‹2/9›                                          tree · t │
│   internal/ui/model.go:970                                                     │
│    ~ startDev › ƒ start                                               +◷ async │
│▸     ~ dev.Reader.Up  rewritten: 56 of 59 rows                  +◷ −5↯ · ↵ all │
│        + dev.Reader.upCommand                           ▤▦⚙⇄⧗◷ new · 29 frames │
│        + dev.Reader.waitReady                           ▤⚙⇄⧗ ↻ new · 15 frames │
│        + dev.Reader.startRecorded                      ▤▦⚙⧗◷ ↻ new · 13 frames │
│        − dev.startDetached                                           ▤▦⚙ async │
│        − os.WriteFile                                                        ▦ │
│        ⋯ 51 more rows · ↵ all                                                  │
│                                                                                │
│ handleKey ▸ key DevStop  ‹3/9›                                        new flow │
│    + ui.Model.stopDev                                    ▤▦⚙⧗◷ new · 63 frames │
└────────────────────────────────────────────────────────────────────────────────┘
```

### Reading, no reader, broken base

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Flows  reading the base (a1b2c3d)… 0.6 s                                     ⣾ │
│   Files and Commits are ready; this fills in when the call graph is.           │
└────────────────────────────────────────────────────────────────────────────────┘
┌────────────────────────────────────────────────────────────────────────────────┐
│ Flows  apps/admin is TypeScript: no call reader for it yet                     │
│   1 Go module was read: services/api · 3 flows                                 │
└────────────────────────────────────────────────────────────────────────────────┘
┌────────────────────────────────────────────────────────────────────────────────┐
│ Flows  the base doesn't build: go list: missing go.sum entry                   │
│   showing head only: changed frames and their callers, no diff                 │
└────────────────────────────────────────────────────────────────────────────────┘
```

## 4. Interactions

Keys apply when the tab has the focus, as in Files and Commits. They
follow the architecture tab where the meaning is the same:

| Key | Does |
|---|---|
| `j` `k`, wheel | move through rows or frames |
| `↵`, click | **jump to the code**. On a call row, the diff preview opens at the hunk that adds, removes or moves that call (for a removed call, the base side). On a frame header, its first changed hunk. On a `⋯` or a new-subtree row, expand it |
| `d` | the same place in the external diff tool |
| `t` | cycle the view: first → tree (→ sequence at ≥ 100 columns, if built) |
| `n` `N` | next / previous flow in the tree view |
| `r` | who reaches this: the inverted tree for the selected frame; `r` again goes back |
| `c` | fold or unfold the selected frame |
| `m` | cycle the verdict: ✓ fine → ? ask → ✕ change it → none |
| `y` | copy the selected row's stable ID |
| `esc` | back to the list |

The diff preview must be able to open **at a line**. The architecture tab
needs the same thing; one change serves both. Every site carries its line
(`Site.Line` in the prototype), and the patch reader knows hunk ranges.

**Stable IDs** are built from names, never line numbers, so verdicts and
comments survive rebases:

| Thing | ID |
|---|---|
| flow | `flow:GET /admin/users`, `flow:handleKey ▸ key DevStop` |
| frame | `frame:GET /admin/users>listUsers>audit.record`, the call path from the entry |
| call edge | `call:services/users.listUsers→audit.record#2`, where `#2` is the second call to it in that function, by source order |
| function | `fn:apps/api/src/services/users.listUsers` (Go: `fn:<import path>.<Type>.<Method>`) |
| finding | `finding:<function ID>:<rule>`, such as `…:n-plus-one` |

A frame ID is a path, so the same function in two flows gets two IDs. A
verdict on the function ID covers all of them. Verdicts are stored as the
architecture tab proposes: in the deck's state folder, per repo, branch
and ID, in the walkthrough's format, never in the repo. A verdict whose
ID disappears after a rebase is kept and shown as stale.

Closures need care. The prototype named callback closures
`run$DetailPR`, after the field they are stored in, and that name is
stable. Unnamed closures numbered by position (`overview$body#2`) were
not stable and showed up as removed and added (§11). The ID for a
closure must be its enclosing function plus the field or variable it is
bound to plus its index **among closures bound to the same name**.

## 5. Entry points

Entry points are the one thing a rule set can't guess for every repo. The
deck ships defaults per framework and lets a repo add its own. Following
the architecture tab's choice, a repo declares them in its shared
`.config/dev.json` under an `x-herdr-deck` key:

```json
{
  "x-herdr-deck": {
    "flows": {
      "entries": [
        { "match": "main.main" },
        { "match": "(*Model).Update", "split": "cases" },
        { "match": "(*Model).handleKey", "split": "cases" },
        { "route": "apps/api/src/routes/**", "framework": "express" }
      ],
      "ui": ["(*Model).Update", "(*Model).View"],
      "ignore": ["**/*_test.go", "**/testdata/**"]
    }
  }
}
```

- **`split: "cases"`** turns each case of a function's top-level `switch`
  into its own entry point: `Update ▸ diffMsg`, `handleKey ▸ key Quit`.
  This is how a Bubble Tea app's flows appear. The prototype does exactly
  this, and a key label is taken from `m.keys.X`.
- **`ui`** marks entries that run on a UI goroutine (Bubble Tea's
  `Update` and `View`, a GUI event loop). A blocking effect reached from
  them synchronously is a `⚠`.
- **Built-in defaults:**
  - Go: `main.main`; `http.HandleFunc` / `mux.Handle` registrations;
    cobra `RunE`; Bubble Tea `Update` / `View`.
  - TypeScript: Express, Fastify and Next.js route files.
  - Python: FastAPI / Flask decorators, Celery tasks, click commands.
  These are patterns over the syntax tree, not types.
- **Without any config**, the fallback roots are exported functions with
  no callers in the module, which is what CHA-style analysis treats as
  reachable from outside. The tab says "no entry config: guessing
  roots".

## 6. The data pipeline

Everything runs off the UI goroutine and is cached, as Files and PR do.
It may also run as a subprocess (`herdr-deck flows …`) so the deck keeps
its memory small; see "Memory" below. Costs were measured with the
prototype on herdr-deck: about 40,000 lines of Go with tests, and 974
functions plus split cases in the non-test build. That was on an
M-series Mac, warm disk cache, PR #53 (+2,653 −249 lines in 72 files).

| # | Step | How | Cost here |
|---|---|---|---|
| 1 | Base and head | merge-base with the thread's base (the Files tab has it) and the worktree | git, ~5 ms |
| 2 | Changed files | `git diff --name-only` plus hunk line ranges (`-U0`) | ~10 ms |
| 3 | Trees on disk | head: the thread's worktree as it is (uncommitted included). Base: `git archive <sha> \| tar -x` into `$XDG_CACHE_HOME/herdr-deck/flows/<sha>/`, once per base SHA. **The repo is never written to**, and no `git worktree add` | base 0.15–0.25 s, once |
| 4 | Load and type-check | `golang.org/x/tools/go/packages` (syntax and types for the module; dependencies from export data). It runs `go list`, which compiles dependencies into the Go build cache the first time | **0.5–0.6 s per side warm; 4.0 s for the first side with an empty build cache** |
| 5 | Per-function summary | walk each body in source order: call sites with context (loop, branch, error branch, defer, goroutine, async closure), resolved callees, error returns, a body hash with comments stripped. Interface calls resolve to the module's implementations (CHA). Calls through func-typed struct fields resolve to whatever is assigned to that field | part of 4 |
| 6 | Diff | function identity by ID; changed = body hash differs; renames by equal body, then by call-set similarity (as a hint); same-shape refactor check; per-frame LCS of call labels; effect propagation (memoised); reverse reachability to entries; hubs; risk score | **3 ms** |
| 7 | Render | the view at the pane width | < 1 ms |
| 8 | Cache | per-function summaries per **package content hash** (its files' blob SHAs plus its imports' hashes), on disk under `$XDG_CACHE_HOME/herdr-deck/flows/`. The result is cached by `(base, head tree, entry config, analyser version)` | — |

Per refresh, this is what can be skipped:
- **The base is computed once** per base SHA and kept as summaries.
- **The head is recomputed only when the worktree changes**, which the
  deck already detects for the Files tab. Then only packages whose files
  changed, and the packages that import them, need type-checking again.
  go/packages can load a subset of patterns, with export data for the
  rest.
- On a typical PR, that keeps a refresh under a second. Without the
  package cache it is about 1.2 s warm.

**Memory.** Loading both sides peaked at **135 MB** RSS warm and 300 MB
cold. That is too much to keep in the deck between refreshes, but fine
in a short-lived subprocess or a goroutine whose results are reduced to
summaries (a few hundred KB) right away.

**Requirements.** The `go` command must be on `PATH` and the module must
resolve offline (its dependencies in the module cache). The prototype
runs with `GOPROXY=off`, so it never downloads. When the base or head
doesn't load, the tab falls back (§9).

**Read-only.** go/packages compiles dependencies into the Go build cache,
which is shared by every Go command and harmless. It runs no tests, no
`go generate`, and no repo code. A repo with cgo would make `go list`
invoke the C toolchain (and `pkg-config` directives), which is worth a
note in the settings, not a block.

**Alternatives I measured or checked:**
- **gopls `callHierarchy`.** gopls is not installed on this machine.
  Its own docs say dynamic calls are not included, and nested function
  literals fold into their enclosing function. That would lose exactly
  the callback and closure edges that mattered in §11. It is also one
  request per function per side. For Go, go/types directly is both
  cheaper and more precise.
- **No type information** (`go/parser` only, as the architecture tab
  does). This can't resolve `m.foo()` to a method without knowing `m`'s
  type, and loses interface and field dispatch. Not enough for this tab.
  A hybrid could type-check from git blobs with export data from the
  worktree, which removes step 3's archive. That is an optimisation for
  later.
- **x/tools `callgraph` (static, CHA, RTA, VTA).** These are SSA-based
  call graphs. VTA is the most precise for function values and
  interfaces, but needs SSA of the whole program, and is still marked
  experimental. The prototype's AST walk plus two cheap resolutions
  (CHA for interfaces, field binding for callbacks) found every edge
  that mattered on these PRs. It keeps source-order and context, such
  as loop, branch and async, which SSA blurs. VTA is the upgrade if
  dynamic calls turn out to matter more.

## 7. Languages

**Go first**, because herdr-deck and the deck's own users' repos are Go
and the prototype shows it works with nothing but x/tools. The deck would
gain one dependency, `golang.org/x/tools` (go/packages); go/types is
stdlib.

For **TypeScript and Python** the honest answer is that it needs a spike.
The options, checked on 2026-10-07:

| Option | TS | Python | Notes |
|---|---|---|---|
| **LSP `callHierarchy`** (LSP 3.16+: `prepareCallHierarchy`, `incomingCalls`, `outgoingCalls` with `fromRanges`) | typescript-language-server implements it (TS ≥ 3.8); vtsls wraps VS Code's TS support (support likely, not confirmed in its docs) | pyright implements all three; basedpyright is a fork | One request per function and direction, on two workspaces (base and head). Gives call sites, not context (loop, branch), so a syntax pass is still needed. Server start-up and indexing on a large repo is the unknown cost |
| **gotreesitter** (pure-Go tree-sitter runtime, MIT, 206 grammars including TS, TSX, JS and Python, pre-1.0: v0.55.1 on 2026-09-26) | yes | yes | No CGO, so it fits the deck. Syntax only: calls resolve by name and import, which is imprecise for methods. Young project; full-parse speed and fidelity unverified |
| tree-sitter via WASM (wazero) | — | — | Only experiments exist, not a maintained library |
| esbuild's parser | its parser is `internal/` | — | Not importable; a third-party fork exposes it |
| typescript-go | `internal/`, "API: not ready", archived 2026-09-01 | — | No |
| gpython parser | — | Python 3.4 grammar only | No |

**Language servers on this machine** (not installed by me):
- **On `PATH`:** `clangd` (Xcode's). `gopls`, `pyright`,
  `typescript-language-server`, `pylsp` and `rust-analyzer` are not.
- **Bundled with editors:** Zed's language folder has `basedpyright`,
  `vtsls`, `eslint` and others. VS Code's app bundle has `tsserver.js`,
  which speaks tsserver's own protocol, not LSP. Pylance is
  closed-source and licensed for VS Code only.
- So the deck could only use a server it finds, never ship one. It
  would say which one it used.

Recommended path: Go in phase 1. For TS or Python, a spike that takes
the user's real repo and compares:
- gotreesitter with name resolution plus a per-framework entry rule set;
- basedpyright / vtsls call hierarchy for the changed functions only.

Measure precision of the edges that matter and start-up cost. Pick one,
not both.

## 8. Signals and their rules

All of these are rules in the prototype (`effects.go`, `diff.go`).

**Side effects.**
- A table maps a qualified callee prefix to an effect: `os.ReadFile` → `▤`,
  `os/exec.` → `⚙`, `net/http.` → `⇄`, `database/sql.` → `⛁`,
  `time.Sleep` → `◷`, `sync.Mutex.Lock` → `⧗`, `os.Getenv` → env.
- A second list marks pure packages (`strings.`, `sort.`, Charm's
  libraries, …) so they fold away.
- Anything else outside the module is *unknown*.
- Effects propagate up through callers (memoised). They are tracked
  twice: all effects, and **synchronous** ones, which don't cross a `go`
  statement or a Bubble Tea `tea.Cmd` closure.
- On the two PRs, the changed frames made **3 unknown external calls in
  total**: `os.Hostname`, `os.Link` and `sync.WaitGroup.Go`. All three
  were added to the table in a line each.
- For TS and Python the table is per library (`pg`, `prisma`, `axios`,
  `fetch`, `requests`, `sqlalchemy`, `boto3`, …). It is shared across
  repos, so it grows once.

**Error paths.**
- Go: `return …, <non-nil error>` and `panic` are pseudo-calls in the
  sequence (`↯`), so they diff like calls: added, removed, moved into or
  out of an `if err != nil`.
- TS / Python: `throw` / `raise`, and `try` / `except` clauses as
  context, so a removed `catch` shows as `− ↯ catch X`.
- Where an error ends up ("→ 500") needs a framework rule for the
  entry: which error types map to which status. That is a phase-3
  nicety.

**Cost.**
- A call inside a loop whose callee has a blocking effect is
  `↻ … blocking call in a loop`. A database call in a loop is `N+1`.
- A blocking effect newly reachable **synchronously** from a `ui`
  entry is `⚠ blocks UI`. This is the deck's own rule ("fetching runs
  off the UI goroutine") turned into a check.
- A removed cache is visible as `− cache.get`, plus a rule that names
  cache libraries.

**Blast radius.** The number of entry points that reach the frame,
walking callers and stopping at the nearest entry. A hub's radius is
shown, but ranks lower: a change to `layout` matters, yet "39 flows"
says little.

**Refactors.**
- A changed function whose call sequence, contexts and error returns
  are identical is `≈ same calls`, folded, and ranked last.
- This is *not* proof of behaviour preservation: changed arguments, such
  as a different timeout constant, also look the same. So the UI says
  "same calls", never "safe".
- A removed and an added function with the same body are a rename
  (fact). With call-set Jaccard ≥ 0.6 in the same package, they are a
  rename *hint*. The prototype found `devGroup → devGroups` this way.

**Ranking** (prototype weights, to be tuned):
- base score 1;
- +60 for blocks UI;
- +25 per blocking call newly in a loop;
- +20 per new effect class;
- +10 per removed error path;
- +8 for lost effects;
- +6 per new error path;
- +5 per call moved in or out of a branch.

Multiply by `1 + log2(1 + flows)`. Refactors score 0.1, and frames no
entry reaches get ×0.3.

## 9. Fallbacks

| Case | What the tab does |
|---|---|
| Huge PR (hundreds of changed functions) | *Look here first* stays capped and ranked. Flows are ordered by their best finding, and the tree view loads one flow at a time |
| Rewrite of one function (most rows changed) | one row with counts and effect and error deltas, plus effectful rows only; `↵` for all (mockup above) |
| New flow or new subtree | one row with icons and a frame count; `↵` expands |
| Hubs | shown once under *Shared frames*; flows link to them with `◇` |
| Unresolved dynamic call (a func value nobody assigns in the module) | a `ƒ name` row that never expands, counted in the summary ("4 dynamic calls not followed") |
| Changed frame no entry reaches | listed with "no entry reaches it", ranked ×0.3. It's usually test-only, dead, or an entry config gap, and the tab says which entry config is in use |
| Base or head doesn't build | head only, with changed frames and their callers and no diff, and the reason (mockup above) |
| Language without a reader | the tab says so and points at Files; it never guesses |
| No entry config | roots guessed (exported, uncalled) with a note |

## 10. Where AI could help

The short answer: nowhere that matters for phase 1. Rules were enough for
every signal on real PRs. If AI is ever added, it must be:
- optional;
- local-first or clearly opt-in;
- shown as a hint (`?ai`);
- cached per symbol.

The candidates, honestly assessed:

| Use | Rules first | Would a model help? | Verdict |
|---|---|---|---|
| **Classify unknown external calls** | the table plus pure lists; 3 unknown calls across two PRs | For npm and PyPI repos the unknown set is bigger. A small local LLM with constrained JSON output could label `somelib.flush()` as network or file. Qwen2.5-Coder 0.5B and 1.5B are Apache-2.0. Ollama's `format` accepts a JSON schema; llama.cpp has JSON-schema grammars | **Maybe later**, opt-in, per *library function*, cached globally and shown as `?` until a person confirms. A shared rule pack is better value |
| **Match renamed or moved functions** | equal body hash (fact), then call-set Jaccard (hint) | Embeddings help only for a rename *plus* a heavy rewrite. Local code-embedding models exist: CodeSage-small-v2 (130M, Apache-2.0) and jina-embeddings-v2-base-code (Apache-2.0); jina's newer code models are CC-BY-NC. Research on semantic clone detection shows learned detectors lean on lexical shortcuts. No head-to-head study of embeddings against AST hashing for renames was found | **No.** An unmatched pair just shows as − and +, which is honest |
| **Risk scoring** | the weights above | A model would be opaque and unstable | **No** |
| **One-line flow summary** ("now writes an audit row on every list request") | — | This is the one thing rules can't write | **Optional, on demand** (a key, not on refresh), opt-in, the user's configured model; cached per (flow, head) and marked as a hint |

## 11. The prototype

[`experimental/callstack`](../../experimental/callstack) is about 2,200
lines of Go in its own module, so `golang.org/x/tools` stays out of the
deck's `go.mod` and CI. It:
- archives two revisions into a temp folder;
- loads both with go/packages;
- builds the per-function summaries;
- diffs them and prints *Look here first*, the top flow trees, shared
  frames and one *who reaches this* tree, at the widths asked for.

```sh
cd experimental/callstack
go run . -repo ../.. -base 0937435^1 -head 0937435 -width 60,80,120
```

Saved outputs are in
[`experimental/callstack/samples`](../../experimental/callstack/samples).

### Results on herdr-deck

**PR #53, the PR tab** (+2,653 −249, 56 changed frames, 39 flows):

```text
 Look here first                                           56 frames · 43 flows 
 1 + herdr-deck.run$DetailPR                                            4 flows 
      +⚙ exec, ⧗ lock, $ env
 2 + github.Reader.Detail                                               4 flows 
      +⚙ exec, ⧗ lock, $ env
 3 ~ ui.Model.overview$body                                            39 flows 
      +$ env · 2 call moved in/out of a branch
 4 + ui.Model.showPRTab                                                  1 flow 
      +▤ fs read, ⚙ exec, ⧗ lock, $ env
 5 + ui.Model.comment                                                  39 flows 
      +⧗ lock
 6 + ui.Model.prTab                                                    39 flows 
      +⧗ lock
 7 + ui.Model.openComment                                                1 flow 
      +▤ fs read, ⚙ exec, $ env
 8 + ui.Model.prOnlyKey                                                  1 flow 
      +⧗ lock, $ env
   ⋯ 38 more · ≈ 10 refactors collapsed

 ▸ Update ▸ snapshotMsg
   internal/ui/model.go:514
   via ◇ ensureVisible, curTab, layout
      syncPreview › relayoutPreview › ui.Model.scrollPreview                 if 
      ~ ui.Model.layout                                                       ◇ 
    ~ ui.Model.watchPR
      + ui.Model.curTab                                                    if ◇ 
      + ƒ DetailPR                                         ⚙⧗ if new · 3 frames 
      ~ ƒ FocusPR                                             moved into branch 
          github.Reader.Focus                                                if 
          ≈ github.Reader.want                                               if 
              github.Reader.work                                        async ⋯ 
      ui.Model.refresh                                                       if 
```

The real story of this PR is found and ranked first:
- The PR tab asks the GitHub reader for a PR's details through a new
  `opt.DetailPR` callback set in `main.run`. That ends in `gh`
  (`⚙ exec`).
- The callback is reached from `Update ▸ snapshotMsg` and three other
  flows via `watchPR`.
- The exec happens in the reader's worker goroutine (`async`), so it is
  **not** flagged as blocking the UI. That is right: `Detail` only takes
  a lock and queues.
- A static call graph without field binding missed this completely
  ("no entry reaches it"). Resolving func-typed fields to what `main`
  assigns them was the single most important fix.

**PR #52, dev manifest phase 1** (+4,417 −1,287, 150 changed frames):
- `handleKey ▸ key DevStop` is a new 63-frame flow with every effect
  class.
- `dev.Store.Lock` ranks first for 8 blocking calls in a loop, a
  lock-file retry with `◷ sleep`.
- `dev.Reader.Up` is rewritten: +◷, −5 error paths, 5 calls moved in or
  out of branches.
- The call path from the key to `Up` goes through a local alias
  (`start := m.opt.StartDev`) inside a `tea.Cmd`. It resolves only after
  aliases of fields were tracked.

**Synthetic commit** (in a scratch clone): an `os.ReadFile` in a loop,
added to `ui.Model.diffThread`. That function is on the render path.

```text
 Look here first                                            1 frames · 39 flows 
 1 ⚠ ui.Model.diffThread                                               39 flows 
      +▤ fs read · blocks UI (▤) in 38 flows · ↻ 1 blocking call in a loop

 ▸ Update ▸ diffMsg                                                 sync ⧗ → ▤⧗ 
   internal/ui/model.go:527
   via ◇ curTab, diff, layout, diffThread
      ui.Model.readDiff                                                      if 
      ~ ui.Model.diffThread                                                +▤ ◇ 

 ▸ Update ▸ snapshotMsg                                             sync ⧗ → ▤⧗ 
   internal/ui/model.go:516
   via ◇ ensureVisible, curTab, diff, layout, diffThread
    ~ readDiff › ui.Model.diffThread                                       +▤ ◇ 
 ⋯ 31 more flows · 6 reach it only through shared frames

 ◇ ~ ui.Model.diffThread                                     shared by 39 flows 
   internal/ui/model.go:624
```

### What it got right

- Callback and interface dispatch, once fields and aliases were bound.
  Async boundaries (`go`, `tea.Cmd` closures) came out right, so the
  "blocks UI" rule fired on the synthetic case and stayed quiet on both
  real PRs.
- Bubble Tea entry points by splitting `Update` and `handleKey` cases
  gave meaningful flow names (`handleKey ▸ key DevStop`).
- **Hubs** turned 39 near-identical trees into one shared section.
  Breadcrumbs and pruning made trees fit 60 columns.
- The rule table needed three new lines across two PRs.
- **Speed:** 1.2 s warm end to end, 3 ms for the diff itself.

### What it got wrong or left out

- **Closure IDs.** Unnamed closures in composite literals were numbered
  by position (`overview$body#2`), so the same closure showed up as `~`,
  `−` and `+`. Fix: the naming rule in §4.
- **Dense rewrites** printed as 59 rows, 56 of them `+`, `−` or `~`. Fix: the rewrite
  fallback in §3, not built.
- **Repeated calls of the same function** (`Manifest.Service` ×5) align
  arbitrarily in the LCS. Fix: add the argument shape to the label, or
  pair by nearest line.
- **`⧗ lock` is everywhere**, from the markdown renderer's cache mutex on
  the render path. It is now shown but not counted as blocking. Env reads
  are hidden from trees.
- **Not built:** effect cycles have no SCC fixpoint (a recursive cycle
  under-reports); value flow is not tracked, so "≈ same calls" can hide
  a changed argument; there is no "→ 500" error mapping and no N+1 rule
  beyond "blocking call in a loop".
- **Interactions are not built:** no stable-ID output, no jump to the
  hunk (the site lines are there), no verdicts.
- **Entries** come from a hard-coded config for herdr-deck. `main.main`
  shows up as a flow whose tree is mostly `run`'s wiring, which is noise
  that an entry rule ("main only for CLI subcommands") would remove.
- Only Go. Only committed revisions (the worktree's uncommitted state is
  the obvious next input).

## 12. Recommendation

**Build it, Go first, after the architecture tab.**
- **Why after the architecture tab:** that tab is cheaper (no type
  checking). It catches the structural surprises (new dependency, new
  touchpoint, layer skip), and it builds shared pieces this tab needs:
  open the preview at a line, verdict storage, the `x-herdr-deck` block
  in `.config/dev.json`, ranking rows.
- **What this tab adds** is the behavioural layer an agent's diff hides
  best: effects moving across goroutines, error paths dropped in a
  rewrite, work added to a hot path, and which keys or routes are
  affected.

| Phase | Scope | Effort (thread-sized PRs) |
|---|---|---|
| 1 | `internal/source/flows` for Go from the prototype, with the closure-ID fix, the rewrite fallback, package-hash cache, and entry config from `.config/dev.json` (Bubble Tea and `net/http` defaults). The **Flows** tab with *Look here first* and the tree; `↵` to the hunk; runs in a subprocess or a reduced goroutine | 2 large PRs (reader, then tab) |
| 2 | *Who reaches this*; stable IDs and `y`; verdicts (shared with the architecture tab); uncommitted worktree as head; 120-column side panel | 1 PR |
| 3 | TS or Python spike on the user's real repo (gotreesitter vs. basedpyright/vtsls); then a reader for the winner, with framework entry rules and a library effect table | 1 spike, then 1–2 PRs |
| later | sequence view at ≥ 100 columns; "→ 500" error mapping; per-test coverage as a "no test reaches this changed frame" marker (opt-in, it runs tests); optional AI flow summary | — |

Phase 1 is useful on herdr-deck itself, and on any Go repo with a known
framework.

### Decisions for the user

1. **Build it, and when.** Recommended: yes, phase 1 after the
   architecture tab's phase 1. Re-decide on phase 3 once the target repos
   are named.
2. **Languages.** Recommended: Go first. For phase 3, which repos the
   threads run on decides TS or Python. Answering "which repo, which
   framework" picks the spike.
3. **Static only, or traces.** Recommended: static only. Go's coverage is
   statement-level and per test run, with no call edges, so traces would
   cost a test run per refresh for little gain. Tests also break the
   deck's read-only rule. An opt-in "reached by tests" marker is a
   possible later extra.
4. **AI.** Recommended: none in phases 1–3. Later, at most an on-demand,
   opt-in flow summary and a shared rule pack instead of a classifier
   (§10).
5. **Where entry points are declared.** Recommended:
   `x-herdr-deck.flows` in `.config/dev.json`, next to the architecture
   tab's `x-herdr-deck.architecture`.
6. **Tab name and place.** Recommended: **Flows N**, after Commits. If
   both tabs are built, the tab bar gets long at 60 columns. The
   alternative is one **Impact** tab with two views (`t`: flows /
   architecture), sharing *Look here first*. I lean to one Impact tab,
   since both open on the same kind of ranked list.
7. **Subprocess or in-process.** Recommended: a `herdr-deck flows`
   subcommand the deck runs, so the 135–300 MB of type information never
   lives in the deck. The deck already re-execs itself for updates, so
   the binary is always at hand.

## 13. Sources

Checked 2026-10-07.

- LSP 3.17 specification: call hierarchy since 3.16.0, `CallHierarchyItem`, `IncomingCall` / `OutgoingCall` with `fromRanges`. https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification/
- gopls navigation docs: call hierarchy; "dynamic calls are not included"; function literals folded into the enclosing function. https://go.googlesource.com/tools/+/refs/heads/master/gopls/doc/features/navigation.md
- pyright's language server registers call hierarchy handlers. https://github.com/microsoft/pyright/blob/main/packages/pyright-internal/src/languageServerBase.ts
- typescript-language-server's call hierarchy (TS ≥ 3.8). https://github.com/typescript-language-server/typescript-language-server/blob/master/src/lsp-server.ts
- clangd 20.1.0 release notes: outgoing calls added. https://releases.llvm.org/20.1.0/tools/clang/tools/extra/docs/ReleaseNotes.html
- gotreesitter, a pure-Go tree-sitter runtime. https://github.com/odvcencio/gotreesitter, https://pkg.go.dev/github.com/odvcencio/gotreesitter
- esbuild's parser is internal; Go's internal-package rule since Go 1.5. https://go.dev/doc/go1.5
- typescript-go (archived, API not ready). https://github.com/microsoft/typescript-go
- gpython (Python 3.4 grammar). https://github.com/go-python/gpython
- x/tools call graphs: https://pkg.go.dev/golang.org/x/tools/go/callgraph, …/cha, …/rta, …/vta; go/pointer removed: https://golang.org/issue/59676
- Falleri et al., "Fine-grained and accurate source code differencing" (GumTree), ASE 2014. https://www.labri.fr/perso/xblanc/data/papers/ASE14.pdf
- Zhang & Shasha, "Simple Fast Algorithms for the Editing Distance Between Trees and Related Problems", SIAM J. Comput. 1989. https://doi.org/10.1137/0218082
- Zhuang et al., "PerfDiff", CGO 2008 (aligning calling-context trees across runs). https://research.ibm.com/publications/perfdiff-a-framework-for-performance-difference-analysis-in-a-virtual-machine-environment
- Person et al., "Differential Symbolic Execution", FSE 2008. https://ntrs.nasa.gov/archive/nasa/casi.ntrs.nasa.gov/20090026214.pdf
- RefactoringMiner (Java, Python, Kotlin, TS, JS, C, C++; not Go). https://github.com/tsantalis/RefactoringMiner. RefDiff (Java, JS, C). https://github.com/aserg-ufmg/RefDiff. RefDiff4Go. https://sol.sbc.org.br/index.php/sbcars/article/view/29036
- Baxter et al., "Clone Detection Using Abstract Syntax Trees", ICSM 1998. https://www.semanticdesigns.com/Products/Clone/HowCloneDRWorks.html. "Semantic Code Clone Detection: Are We There Yet?" https://arxiv.org/abs/2606.25272
- Models: CodeSage-small-v2 https://huggingface.co/codesage/codesage-small-v2; jina-embeddings-v2-base-code https://jina.ai/models/jina-embeddings-v2-base-code/; jina code embeddings (CC-BY-NC) https://jina.ai/news/jina-code-embeddings-sota-code-retrieval-at-0-5b-and-1-5b; Qwen2.5-Coder-0.5B/1.5B-Instruct https://huggingface.co/Qwen/Qwen2.5-Coder-1.5B-Instruct
- Ollama structured outputs (`format` with a JSON schema). https://github.com/ollama/ollama/blob/main/docs/api.md. llama.cpp grammars. https://github.com/ggml-org/llama.cpp/blob/master/grammars/README.md
- Go coverage flags and `go test -json`. https://pkg.go.dev/cmd/go/internal/test. `runtime/trace`. https://pkg.go.dev/runtime/trace. PEP 669 (`sys.monitoring`). https://peps.python.org/pep-0669/

Not verified: vtsls's and basedpyright's call hierarchy support in their
own docs, gotreesitter's full-parse speed, and language-server start-up
costs on large repos.
