# Architecture tab: did the shape of the system change?

Written 2026-10-07 against herdr-deck at 0937435. This is an exploration
with a throwaway prototype, not a finished feature. The prototype is
[`exp/archdiff`](../../exp/archdiff), which the deck does not build in.
Tool facts were checked that day against the sources in [§13](#13-sources).
The mockups use the made-up `admin-rebuild` project of the
[design README](../design/README.md#sample-data): thread t-0002, *Users
page*, in the `webshop` monorepo. Its packages, imports and code are
invented. The prototype's output in [§11](#11-the-prototype) is real: it
comes from herdr-deck's own history, which is public.

A sibling exploration, the call-stack diff (which flows behave
differently?), runs in parallel. That tab looks at **behaviour**; this one
looks at **structure**.

**Recommendation in one paragraph.** Build it, small and Go-first. The
hard part is the per-commit dependency graph, and it turned out cheap:
the prototype reads both commits straight from git objects, with no
checkout and no `go list`. It parses Go with the standard library and
caches by blob SHA. herdr-deck's base graph takes about 0.1 s and the
head graph about 0.04 s, because only changed files are parsed again. On
real merges it found what a diff hides:
- a new third-party dependency;
- a package that started running `gh`;
- code moved between packages, with the moves told apart from the real
  changes;
- on a synthetic commit, an upward import, the component cycle it causes,
  and new env, SQL and HTTP touchpoints.

What a terminal draws well is **lists, lanes and a matrix**, not a graph.
So the tab opens on a risk-sorted **Look here first** list, with **lanes**
(one section per layer, edges listed under their package) and a
**lane matrix** for dense changes as the other views. A repo declares its
layers in a small `x-herdr-deck.architecture` block in the shared
`.config/dev.json`. Without it, lanes are inferred and only the
non-layer signals apply. TypeScript and Python come later through
pure-Go import scanners; tree-sitter and language servers are not needed
for this tab. No AI: rules cover every signal here, and the one fuzzy
spot (matching renamed declarations whose bodies also changed) is better
served by a similarity score. Phases and the decisions for the user are
in [§12](#12-recommendation).

Contents:

1. [What the tab answers](#1-what-the-tab-answers)
2. [What a terminal can draw](#2-what-a-terminal-can-draw)
3. [Mockups](#3-mockups)
4. [Interactions](#4-interactions)
5. [How a repo declares its layers](#5-how-a-repo-declares-its-layers)
6. [The data pipeline](#6-the-data-pipeline)
7. [Languages](#7-languages)
8. [Touchpoint rules](#8-touchpoint-rules)
9. [Fallbacks](#9-fallbacks)
10. [Where AI could help](#10-where-ai-could-help)
11. [The prototype](#11-the-prototype)
12. [Recommendation](#12-recommendation)
13. [Sources](#13-sources)

---

## 1. What the tab answers

A diff shows lines. It does not show that `UsersOverviewPage.tsx` now
imports the database package directly, or that the change made the
admin app read a new environment variable. Both are one line in a
200-line hunk. The tab answers one question: **did this change alter the
shape of the system, and where should I look first?**

It shows the change, not the system. It starts from the diff, keeps the
changed packages and the packages at either end of a changed edge, folds
unchanged neighbours into one line per lane, and counts the rest.

The signals, highest value first:

| Signal | What it is | Why a diff hides it |
|---|---|---|
| **Upward edge** `✕` | A package imports one in a higher lane (infra → pages) | One import line; the layering lives in people's heads |
| **New cycle** `↻` | A new edge closes a loop, shown with its whole path | No single file shows a cycle |
| **Layer skip** `⚠` | An edge jumps over a closed lane (pages → db, skipping api) | Looks like any other import |
| **New external dependency** | A new third-party module or package, or one newly imported by a package | Hidden in `go.mod` / `package.json` / lock-file churn |
| **New touchpoint** | env var, SQL table, HTTP host, process, CLI flag, file write | Scattered call sites |
| **New or removed internal edge** | Package A now depends on B | One import line |
| **Public surface** | `+2 −1 ~3` exported names per package | Spread over files; easy to miss a removed export |
| **Change weight** | `+263 −12` per package | The Files tab has it per file only |
| **Pure move vs real change** | "4 files moved, 0 edges changed" | A move looks like a big diff |

Go has one special case: it forbids import cycles between packages, so
in a Go repo a cycle can only show at a coarser zoom level, such as
components or lanes. TypeScript and Python allow cycles between modules,
so there they can show at every level.

## 2. What a terminal can draw

General graph layout (boxes, crossing edges, Sugiyama layers) needs
pixels and free 2-D placement. In 60–120 monospace columns it falls
apart past five or six nodes. Edges have to bend around labels, and the
crossings carry no meaning. Three shapes work well in a terminal, and the
tab uses them:

- **A ranked list.** Each row is one finding with its stable ID and its
  site (`file:line`). This is the default view (**first**). It reads at
  any width and any graph size, and it answers "where do I look".
- **Lanes.** There is one `── lane ──` section per layer, top to bottom.
  Under it are the shown packages, and under each package the edges it
  gains (`━▸`) or loses (`┄▸`), with the verdict. This is the layered
  mockup from the notes, made into a tree rather than a picture: the
  lane is the vertical position, and the edge list replaces the drawn
  line. A skip or an upward edge stands out by its tag, not its
  geometry.
- **A lane matrix.** Rows are importer lanes and columns are imported
  lanes. Each cell holds the unchanged edges plus the new ones (`12+4`).
  Cells below the diagonal point upwards and are marked `✕`. This is the
  fallback for dense changes (§9), and it stays readable at any package
  count, since it has one row and one column per lane.

Drawn boxes with connecting lines are only worth it for ≤ 5 shown nodes
at ≥ 100 columns. That is a possible later polish, not part of the
design.

**Zoom levels**, from coarse to fine, with `<` and `>`:

| Level | Go | TypeScript | Python |
|---|---|---|---|
| lane | the declared layers | same | same |
| component | `internal/source`, `cmd` | workspace package (`@acme/db`) | top-level package |
| **package** (default) | directory | folder | package (directory with `__init__`) or module |
| file | `.go` file | `.ts`/`.tsx` file | `.py` file |

Lane and component zoom aggregate edges. File zoom is for small changes
and shows only changed files plus their direct imports.

**Before / after / diff** (`b` cycles):
- **diff**, the default, shows both sides: new edges bold `━▸`, removed
  dim `┄▸`, unchanged counted but not drawn.
- **before** shows the base graph around the same nodes, so the edges
  that will go are visible in context.
- **after** shows the head graph only.

The list and the lanes keep their nodes across the toggle, so the cursor
stays put.

## 3. Mockups

Conventions follow [the drawer design](../design/drawer/README.md):

- The frames are 28 rows at 60 and 80 columns and 32 rows at 120, at the
  normal drawer height and at full height (`z`).
- Plain text can't show colour: the active tab (**Arch**) is dark text
  on blue. New edges and `+` rows are green, removed `−`/`┄▸` rows dim,
  `✕` red bold, `⚠` yellow, `≡` cyan.
- At full height the scrollbar runs in the content's last column (`┃`
  thumb, `│` track).
- The tab's label is `Arch N`, where N counts the *Look here first*
  items. It shows `Arch ≡` for a pure move, `Arch …` while reading, and
  a dim `Arch` with no data.

### 80 columns, normal height: Look here first

The summary line follows the Files tab's pattern: counts on the left, the
view and its toggle on the right. Each row has a glyph, the finding, a
short reason, and the site at the right edge. The cursor `▸` is on the
riskiest finding.

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
│  Overview   Files 11   Commits 4   Arch 7   Log 7                     ↓ 2 more │
│                                                                                │
│ 7 to look at  ⚠ 1 layer skip  edges +3 −1  vs origin/main      first · t lanes │
│▸1 ⚠ pages/users → @acme/db          skips api          UsersOverviewPage.tsx:4 │
│ 2 + dep  @tanstack/react-table 8.21  ← pages/users                package.json │
│ 3 + http GET /admin/users/overview   in api/users                  users.ts:12 │
│ 4 + env  VITE_USERS_PAGE_SIZE        in pages/users               columns.ts:3 │
│ 5 ~ api  api/users  +1 ~1            fetchUsers(params)            users.ts:20 │
│────────────────────────────────────────────────────────────────────────────────│
│ j k item  ↵ preview  t view  b before/after  < > zoom  m mark  esc list        │
└────────────────────────────────────────────────────────────────────────────────┘
```

### 60 columns, normal height

The site moves out, and the package moves to the right edge only when it
fits.

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
│  Overview   Files 11   Commits 4   Arch 7   Log 7 ↓ 2 more │
│                                                            │
│ 7 to look at  ⚠ 1 skip  edges +3 −1        first · t lanes │
│▸1 ⚠ pages/users → @acme/db  skips api                      │
│ 2 + dep  @tanstack/react-table 8.21                        │
│ 3 + http GET /admin/users/overview               api/users │
│ 4 + env  VITE_USERS_PAGE_SIZE                  pages/users │
│ 5 ~ api  api/users +1 ~1                                   │
│────────────────────────────────────────────────────────────│
│ j k item  ↵ preview  t view  b before  m mark  esc         │
└────────────────────────────────────────────────────────────┘
```

### 80 columns, full height: lanes

`●` changed package, `✚` new, `✕` removed, `◌` unchanged neighbour, `≡`
moved. Unchanged neighbours fold into one line per lane. A hub package
such as `routes.tsx` or Go's `cmd/` imports nearly everything, so "one
step away" would otherwise show the whole graph. The prototype showed
exactly that on herdr-deck before the fold was added.

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Admin rebuild                                               ◐ 2  ◇ 1  ○ 1  ✉ 0 │
│────────────────────────────────────────────────────────────────────────────────│
│ Users page                                                     ◐ working  ~60% │
│ webshop · t-0002-members-admin-users · Building overview            ▰▰▰▰▰▰▱▱▱▱ │
│                                                                                │
│  Overview   Files 11   Commits 4   Arch 7   Log 7                     ↓ 3 more │
│                                                                                │
│ 7 to look at  ⚠ 1 layer skip  edges +3 −1  vs origin/main       lanes · t list┃│
│ ── entry ──                                                                   ┃│
│ ● routes.tsx                                                             +6 −2┃│
│     ━▸ pages/users            ┄▸ pages/members                                ┃│
│ ── pages ──                                                                   ┃│
│▸● pages/users  ≡ from pages/members                        api +1 −1  +263 −12┃│
│     ━▸ @acme/db  ⚠ skips api                                                  ┃│
│     ━▸ @tanstack/react-table  new dep                                         ┃│
│ ◌ 2 unchanged: settings templates                                             ┃│
│ ── api (closed) ──                                                            ┃│
│ ● api/users                                                  api +1 ~1  +31 −9┃│
│ ◌ 1 unchanged: api/client                                                     ┃│
│ ── shared ──                                                                  ┃│
│ ◌ 2 unchanged: @acme/ui @acme/format                                          ┃│
│ ── infra ──                                                                   ┃│
│ ◌ @acme/db                                                                    ││
│ ⋯ 9 more packages, not next to the change                                     ││
│ ── Touchpoints ──                                                             ││
│ + http GET /admin/users/overview   in api/users                    users.ts:12││
│────────────────────────────────────────────────────────────────────────────────│
│ j k node  ↵ detail  t view  b before/after  < > zoom  m mark  esc list         │
└────────────────────────────────────────────────────────────────────────────────┘
```

### 120 columns, full height: lanes and the selection

At 120 columns the content splits, as the diff preview does: lanes on
the left, the selected node or edge on the right. The right side shows
the edge's stable ID, why it was flagged, the sites that create it
(import and first use, with the line), the verdict, and what `↵` does.

```text
┌────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│ Admin rebuild                                                                                       ◐ 2  ◇ 1  ○ 1  ✉ 0 │
│────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────│
│ Users page                                                                                             ◐ working  ~60% │
│ webshop · t-0002-members-admin-users · Building overview                                                    ▰▰▰▰▰▰▱▱▱▱ │
│                                                                                                                        │
│  Overview   Files 11   Commits 4   Arch 7   Log 7                                                                      │
│                                                                                                                        │
│ 7 to look at  ⚠ 1 layer skip  edges +3 −1          lanes  │ edge:pages/users→@acme/db                                  │
│ ── entry ──                                               │ ⚠ skips api: pages may reach infra only through api        │
│ ● routes.tsx                                       +6 −2  │ ── Created by ──                                           │
│     ━▸ pages/users   ┄▸ pages/members                     │ 1 UsersOverviewPage.tsx:4                                  │
│ ── pages ──                                               │   import { userCounts } from "@acme/db";                   │
│ ● pages/users  ≡ from pages/members             +263 −12  │ 2 UsersOverviewPage.tsx:31                                 │
│▸    ━▸ @acme/db  ⚠ skips api                              │   const counts = await userCounts(db, filters);            │
│     ━▸ @tanstack/react-table  new dep                     │ ── Other ways from pages to infra ──                       │
│ ◌ 2 unchanged: settings templates                         │ none before this change                                    │
│ ── api (closed) ──                                        │ ── Verdict ──                                              │
│ ● api/users                                       +31 −9  │ ○ not marked   m ✓ fine · ? ask · ✕ change it              │
│ ◌ 1 unchanged: api/client                                 │                                                            │
│ ── shared ──                                              │ ↵ opens 1 in the diff preview at line 4                    │
│ ◌ 2 unchanged: @acme/ui @acme/format                      │                                                            │
│ ── infra ──                                               │                                                            │
│ ◌ @acme/db                                                │                                                            │
│ ⋯ 9 more packages, not next to the change                 │                                                            │
│                                                           │                                                            │
│ ── Touchpoints ──                                         │                                                            │
│ + http GET /admin/users/overview                          │                                                            │
│ + env  VITE_USERS_PAGE_SIZE                               │                                                            │
│ + dep  @tanstack/react-table 8.21                         │                                                            │
│ + sql  users (insert)  scripts                            │                                                            │
│────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────│
│ j k node  ↵ preview  t view  b before/after  < > zoom  m mark  esc list                                                │
└────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

### Dense change: the lane matrix and edges by risk

At about 40 or more findings, or more than 25 shown packages, the tab
opens on the matrix. Selecting a cell filters the edge list to that pair
of lanes.

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Admin rebuild                                               ◐ 2  ◇ 1  ○ 1  ✉ 0 │
│────────────────────────────────────────────────────────────────────────────────│
│ Users page                                                     ◐ working  ~60% │
│ webshop · t-0002-members-admin-users · Building overview            ▰▰▰▰▰▰▱▱▱▱ │
│                                                                                │
│  Overview   Files 11   Commits 4   Arch 7   Log 7                    ↓ 36 more │
│                                                                                │
│ 41 to look at  ✕ 2 upward  ⚠ 5 skip  edges +38 −22            matrix · t lanes┃│
│ ── Lanes ──  (unchanged edges, +new; ✕ points up)                             ┃│
│  from \ to   entry  pages    api  shared  infra                               ┃│
│  entry           ·    6+2      ·       1      ·                               ┃│
│  pages           ·      3   12+4    9+3     +2⚠                               ││
│  api             ·      ·      5   7+11     8+6                               ││
│  shared          ·    ✕+2      ·     4+3      2                               ││
│  infra           ·      ·      ·       ·      1                               ││
│ ── Edges by risk ──                                                           ││
│▸1 ✕ shared/format → pages/users       upward                       format.ts:2││
│ 2 ✕ shared/format → pages/members     upward                        dates.ts:1││
│ 3 ⚠ pages/users → @acme/db            skips api        UsersOverviewPage.tsx:4││
│ 4 ⚠ pages/billing → @acme/db          skips api                 Invoices.tsx:7││
│ 5 + api/billing → @acme/stripe        new dep                     billing.ts:3││
│ ⋯ 36 more: 9 new internal edges, 22 removed, 5 touchpoints                    ││
│                                                                               ││
│                                                                               ││
│                                                                               ││
│                                                                               ││
│────────────────────────────────────────────────────────────────────────────────│
│ j k edge  ↵ preview  t view  enter cell filters  < > zoom  esc list            │
└────────────────────────────────────────────────────────────────────────────────┘
```

### Pure move (60 columns)

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
│  Overview   Files 11   Commits 4   Arch ≡   Log 7          │
│                                                            │
│ ≡ pure move                                first · t lanes │
│ 4 files moved, 0 edges changed, no API change              │
│ ≡ pages/members → pages/users     4 files                  │
│   renamed: MembersPage → UsersPage                         │
│   3 edges carried over: api/users @acme/ui routes          │
│                                                            │
│────────────────────────────────────────────────────────────│
│ j k item  ↵ preview  t view  esc list                      │
└────────────────────────────────────────────────────────────┘
```

### Reading, and a language with no reader

The base graph is usually cached from an earlier thread on the same
merge-base. The head fills in a moment later. Files in a language the
deck cannot read are named, not silently skipped.

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
│  Overview   Files 11   Commits 4   Arch …   Log 7                              │
│                                                                                │
│ reading head 3c41a9e…  base done (212 files, 0.3 s)            first · t lanes │
│ ⋯ graph diff appears when head is read; touchpoints follow                     │
│                                                                                │
│ ── Not covered ──                                                              │
│ 14 Kotlin files in apps/android: no reader for Kotlin; Files tab has them      │
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ t view  esc list                                                               │
└────────────────────────────────────────────────────────────────────────────────┘
```

## 4. Interactions

Keys apply when the tab has the focus (`tab`, or a click in the
drawer), the same way as Files and Commits:

| Key | Does |
|---|---|
| `j` `k`, wheel | move the cursor through findings, nodes or edges |
| `↵`, click | **jump to the code**. On an edge, open the diff preview at the import line that creates it (for a removed edge, the base side's line); `↵` again goes to the next site. On a touchpoint, its call site. On a package, its first changed hunk. On a public-surface item, the declaration |
| `d` | the same site in the external diff tool |
| `t` | cycle the view: first → lanes → matrix |
| `b` | cycle diff → before → after |
| `<` `>` | zoom out / in (lane, component, package, file) |
| `m` | cycle the verdict on the selected item: ✓ fine → ? ask → ✕ change it → none |
| `y` | copy the selected item's stable ID |
| `esc` | back to the list (as in every tab) |

New in the preview: it must open **at a line**. Today it opens a file's
diff at its top (PLAN.md, *Diff preview*). The import is usually in the
first hunk, but a first-use site often isn't. The patch reader already
knows each hunk's line numbers, so this needs only a target line and a
scroll offset.

**Stable IDs.** These don't depend on line numbers, so comments and
verdicts survive rebases:

| Thing | ID |
|---|---|
| node | `node:pages/users` |
| edge | `edge:pages/users→@acme/db` |
| cycle | `cycle:internal/source→internal/ui→internal/source` (rotated so it starts at its smallest node) |
| touchpoint | `touch:env:VITE_USERS_PAGE_SIZE@pages/users` |
| public surface | `api:api/users.fetchUsers` |
| dependency | `dep:@tanstack/react-table` |

**Verdicts** are ✓ / ? / ✕ per ID. They are stored per repo, branch and
ID, the same format the walkthrough and review-tracker flows use, so a
review can pick them up. The deck keeps them in its state folder
(`$XDG_STATE_HOME/herdr-deck`) and never in the repo. Exporting them as a
review comment is a later step. A verdict whose ID no longer exists after
a rebase is kept, but shown as stale.

## 5. How a repo declares its layers

Prior art agrees on the shape: **ordered layers, each a set of path
globs; higher may import lower, never the reverse.**

| Tool | Format |
|---|---|
| import-linter (Python) | `type = "layers"`, an ordered list high → low, with optional `containers`, and `a \| b` for independent peers on one level |
| go-arch-lint (Go) | YAML `components: {name: {in: glob}}` plus `deps: {name: {mayDependOn: […]}}` |
| dependency-cruiser (JS/TS) | `forbidden` rules of `from`/`to` path regexes |
| ArchUnit (Java) | `layeredArchitecture().layer(…).definedBy(…).whereLayer(…).mayOnlyBeAccessedByLayers(…)` |
| depguard (golangci-lint) | per-file-glob allow/deny lists, with no layer concept |

The deck takes import-linter's model plus one flag. It lives in the
shared dev manifest under the deck's own `x-` key, which the schema
already allows ("one tool's settings; other tools ignore them"):

```json
{
  "x-herdr-deck": {
    "architecture": {
      "layers": [
        { "name": "entry",  "paths": ["apps/admin/src/routes.tsx", "apps/admin/src/main.tsx"] },
        { "name": "pages",  "paths": ["apps/admin/src/pages/**"] },
        { "name": "api",    "paths": ["apps/admin/src/api/**"], "closed": true },
        { "name": "shared", "paths": ["packages/ui/**", "packages/format/**"] },
        { "name": "infra",  "paths": ["packages/db/**"] }
      ]
    }
  }
}
```

- A package may import its own lane and any lane below it. Importing
  upwards is `✕`.
- `closed: true` means callers from above must go through this lane.
  Jumping over it is `⚠ skip`. Most layers are open, which keeps a
  first config quiet.
- The first matching lane wins. Packages that match no lane go to
  `unassigned` at the bottom and never produce a violation.
- **Without a config**, the deck infers lanes from the graph: a
  package's lane is the longest import chain below it, the same as the
  prototype's `depth N` lanes. Inferred lanes cannot be violated by
  definition, so they help only to place nodes. Every other signal still
  works.
- **Conventions** the deck could guess later: `cmd/`, `internal/`, `pkg/`
  in Go; `apps/*` over `packages/*` in a monorepo; `api`/`routes` over
  `services`/`domain` over `db`/`infra`/`adapters` by folder name. A
  guess is shown as a hint ("lanes guessed from folder names"), never as
  violations.
- Reading an existing `.importlinter`, `.go-arch-lint.yml` or
  `.dependency-cruiser.js` config is possible later. This follows the
  manifest decision: read other tools' files only when they publish a
  spec. import-linter and go-arch-lint do.

## 6. The data pipeline

Everything runs off the UI goroutine. Results are cached and fill in as
they arrive, the same way the Files and PR tabs work. Costs are measured
on herdr-deck (467 tracked files, 25 packages) on an M-series Mac, warm
disk cache, best of three.

| # | Step | How | Cost here |
|---|---|---|---|
| 1 | Pick base and head | merge-base with the base branch (the Files tab already has it), and the thread worktree's `HEAD` | git, ~5 ms |
| 2 | List both trees | `git ls-tree -r -z <sha>` for each side. **No checkout, nothing written** | ~10 ms each |
| 3 | Read blobs | one long-lived `git cat-file --batch` per repo, fed the SHAs of source files whose blobs are not cached | part of 4 |
| 4 | Parse per blob | Go: `go/parser` (stdlib). TS/Python: import scanners (§7). Output: imports with lines, top-level declarations with a signature and two hashes, touchpoints. **Cached by blob SHA**, so head re-parses only changed files | base 0.09–0.14 s for 84–91 files; head 0.02–0.05 s for 2–23 new blobs |
| 5 | Build each graph | resolve imports to nodes (go.mod module paths; tsconfig `paths` and workspaces; Python source roots); edges keep their sites | < 10 ms |
| 6 | Diff | `git diff --raw --numstat -z -M base head` gives renames and weights. Then edges ±, components and cycles, lanes and verdicts, declarations matched by hash (moved) and name-blind hash (renamed), public surface, touchpoints, `go.mod`/`package.json` requirements | 20–35 ms |
| 7 | Rank and render | risk score per finding; the view at the pane's width | < 1 ms |
| 8 | Cache | blob facts by blob SHA; the result by `(base, head, config hash, analyser version)`, in memory (LRU) and on disk under `$XDG_CACHE_HOME/herdr-deck/arch/` | — |

For comparison, `go list -deps -json ./...` on a checked-out herdr-deck
takes 0.55 s, and needs a working tree per commit. go/packages calls
`go list` under the hood. The tab doesn't need type information for
anything above, so go/packages adds cost without changing the result.

When to compute:
- when the tab is shown, for the selected thread;
- again when the thread's `HEAD` moves, at the deck's reload, which
  costs only the changed blobs;
- for an uncommitted worktree, by hashing changed files with
  `git hash-object` into the same cache.

Big repos may need a size gate, such as reading only the packages under
changed files' components first. That is a later concern; herdr-deck and
a typical app repo take well under a second.

## 7. Languages

**Go first.** The deck is Go and so is its own repo. The standard
library parses Go fully with no dependencies (`go/parser`, `go/ast`,
`go/printer`). That is what the prototype uses. Known gaps:
- build constraints are ignored, so the graph is the union over
  platforms (fine for architecture);
- `go.work` is not read;
- generated files count like any other.

**TypeScript second**, since the sample projects are TS monorepos. The
options, all without CGO:

| Option | Pure Go | Imports | Type-only imports | Exports and touchpoints | Notes |
|---|---|---|---|---|---|
| **Own import scanner** (recommended first) | yes | `import … from`, `export … from`, `import()`, `require()` | kept | exports by regex on `export` declarations; touchpoints by rules on lines | Must resolve specifiers itself: relative, `index` files, tsconfig `paths`/`baseUrl`, workspace packages from `package.json` |
| esbuild Go API (`pkg/api`, v0.28.2) | yes (only `golang.org/x/sys`) | metafile lists each input's imports with kind and resolved path; `Packages: external` | **dropped**: esbuild removes unused and type-only imports before the metafile | no | Good resolver (tsconfig `paths` honoured). Bundles, so it reads files from disk: needs a checkout per commit, or an `OnLoad` plugin that serves git blobs (not tried). Large dependency for one feature |
| gotreesitter (pure-Go tree-sitter runtime, MIT) | yes | full syntax tree, query language | kept | yes, via queries | Ships go, python, typescript and tsx grammars. Young project; its speed and accuracy claims are the README's own. A spike should test it before relying on it |
| typescript-go (tsgo) | yes | — | — | — | Its parser is under `internal/`, so other modules cannot import it, and its API is "not ready" |
| typescript-language-server | Node | via LSP | — | — | Not installed here. Heavy for a graph |

Type-only imports matter for architecture: `import type { Row } from
"@acme/db"` in a page is still a dependency on infra's types. That rules
out esbuild as the only source. Start with a careful scanner (strip
comments and template strings first) plus a resolver. Move to gotreesitter
for exports and touchpoints once a spike shows it is fast and right on a
real repo.

**Python third.** The options:
- **Own import scanner** (`import a.b`, `from .x import y`, relative
  levels, `TYPE_CHECKING` blocks marked), with resolution against the
  source roots (`src/`, `pyproject` packages). This follows the same
  pattern as TS.
- **`ruff analyze graph`**: ruff 0.16.1 is installed here. It prints a
  JSON map file → files and has flags for string imports and
  type-checking imports. It is still marked *experimental and may change
  without warning*, and it needs files on disk. Use it as an optional
  cross-check, not as the source.
- **gotreesitter's python grammar**, the same deal as TS. Which Python
  version the grammar covers was not verified.
- gpython's parser is Python 3.4, and newer pure-Go Python parsers are
  very early, so neither is usable.

**Language servers.** None of gopls, pyright, basedpyright,
typescript-language-server or ty is installed on this machine. Node,
ruff and the VS Code Pylance extension are; Pylance's licence allows
use only inside VS Code, so the deck cannot drive it. All of them
support call hierarchy and references, which the **call-stack** sibling
needs. This tab doesn't: import edges, exports and touchpoints are
syntactic. An LSP is worth adding here only for precision on
re-exports, barrel files and dynamic imports, and only if it is already
installed.

**Unknown languages** are listed under *Not covered* with their file
count (see [the mockup](#reading-and-a-language-with-no-reader)). The
deck never guesses edges for them.

## 8. Touchpoint rules

Rules, not models. A touchpoint is a call whose function resolves
through the file's imports to one of these. Its target is a string
literal or a constant from the same file (the prototype's limit), else
`(dynamic)`. A touchpoint counts as new when its `kind target` occurs
more often in a package's changed files at head than at base. Unchanged
files can't change the count.

| Kind | Go | TypeScript | Python |
|---|---|---|---|
| env | `os.Getenv`, `LookupEnv`, `Setenv` | `process.env.X`, `process.env["X"]`, `import.meta.env.X` | `os.environ[…]`, `os.getenv`, `environ.get` |
| http | `http.Get`/`Post`/`NewRequest…`; host from a literal URL | `fetch(…)`, `axios.*`, `new URL(…)` with a literal | `requests.*`, `httpx.*`, `urllib.request.urlopen` |
| sql | string literals that start like SQL: tables after `FROM`/`INTO`/`UPDATE`/`JOIN` | same, plus tagged templates (`sql\`…\``) | same, plus SQLAlchemy `Table("name")` |
| exec | `exec.Command[Context]` binary | `child_process.*` | `subprocess.*`, `os.system` |
| config | `flag.*` names; repo rules, such as herdr-deck's `config.Specs` | `config.get("…")` | `settings.X` (Django) |
| fs write | `os.WriteFile`, `Create`, `MkdirAll`, `Remove…`, `Rename` | `fs.write*`, `fs.rm*` | `open(…, "w")`, `shutil.*` |
| queue | — (repo rules) | `new Queue("name")`, `.publish("topic")` | `celery` `@task`, `.delay` |
| dependency | `go.mod` require ±, version bumps; a new *direct* import of a module that was only indirect | `package.json` dependencies ± per workspace; a new package import | `pyproject` / `requirements*.txt` ± |

The standard library imports that are themselves touchpoints show as
edges with a kind:
- `net/http`, `net`: network;
- `os/exec`, `syscall`: process;
- `database/sql`: database;
- `unsafe`, `reflect`, `plugin`: unsafe.

The rule table is data. A repo can add its own entries in the same
`x-herdr-deck.architecture` block, for example `{"kind": "queue", "call":
"jobs.Enqueue", "arg": 0}`.

## 9. Fallbacks

| Case | What the tab does |
|---|---|
| Dense: ≥ 40 findings or > 25 shown packages | Opens on the lane matrix plus *edges by risk*; lanes stay one `t` away |
| Huge refactor where most changes are moves | Leads with `≡ N decls moved, M renamed` grouped by `from → to`, then the real changes. Moved edges pair up (`− old → dep` and `+ new → dep` become "dependency moved with its code"; see §11) |
| No layer config | Lanes inferred by depth, no `✕`/`⚠`; a dim hint "declare lanes in .config/dev.json" |
| Language with no reader | *Not covered* lists the files; the rest of the tab works |
| Parse error in a file | That file's last good parse is used if its blob was seen before, else it is skipped and named in *Not covered* |
| Dynamic imports, reflection, DI containers | Shown as `(dynamic)` touchpoints, never guessed |
| No base (orphan branch, shallow clone) | A one-line note, as the Files tab shows for a missing base |
| Monorepo with many apps | Component zoom by default when more than one workspace package changed |

## 10. Where AI could help

Measured against this tab's signals, rules are enough:

- Edges, cycles, lanes and the public surface are exact facts from
  syntax. A model can only add noise.
- Touchpoints are a rule table. The misses are dynamic values, and a
  model would guess those.
- Risk ranking is a fixed score per kind. It can be tuned by hand and
  explained ("upward > cycle > skip > new dep > touchpoint > edge"). A
  learned score would not explain itself.
- Matching declarations that moved **and** changed is the fuzzy edge.
  The prototype matches exact and name-blind hashes. The next step is a
  token-similarity score (for example, Jaccard over identifiers,
  threshold 0.8), still with no model.
- Summarising what a new dependency is for ("@tanstack/react-table: a
  headless table library") would be nice. A static list of well-known
  packages, or the registry's own description fetched once and cached,
  does it without a model.

Verdict: **no AI ingredient for this tab.** If one is ever added, the
place is a one-line, on-demand "what does this edge do" hint on the
selected edge, from the sites the tab already has. It would be opt-in,
local-first, cached by edge ID and marked as a hint. The call-stack tab
has more fuzzy edges (side effects of unknown calls) and is the better
place to try a model first.

## 11. The prototype

[`exp/archdiff`](../../exp/archdiff) is a standalone Go command of about
1800 lines, using only the standard library. It reads two commits from
git, builds both package graphs, diffs them and prints the tab's text
at a given width. It is not built into the deck and imports nothing from
it. Run it with:

```sh
go run ./exp/archdiff -base 9fc47b3^1 -head 9fc47b3^2 \
  -layers exp/archdiff/testdata/herdr-deck-layers.json -width 80 -time
```

`-view first|lanes|matrix|detail|all` picks the views, and `-tests`
counts `_test.go` files. The layers file is herdr-deck's own lanes,
taken from what its CLAUDE.md says: entry (`cmd`, `skills`,
`internal/plugin`) over ui over sources over core.

### Results on herdr-deck

**PR #52, the dev manifest phase 1** (merge 9fc47b3, 32 Go files):

```text
 25 packages · base 4354826 → head 09dad2a         f first · l lanes · m matrix
 edges +5 −2  touch +3  api +54 −12 ~6
 decls: 19 moved · 1 renamed · 24 changed · 136 new · 33 gone · 0 files moved

 ── Look here first ──
 1  + http (dynamic)  in internal/source/dev
 2  + internal/source/dev/manifest → github.com/santhosh-tekuri/jsonschema/v6  …
 3  + exec (dynamic)  in internal/source/dev
 4  + internal/source/dev → net/http  network
 5  ~ api  internal/config  −1 ~0 +1
 …
 ── Moved unchanged ──
 ≡ 1 decls internal/source/dev → internal/source/dev/manifest
 ≡ 19 decls skills/dev-manifest/validate → internal/source/dev/manifest
```

What was right:
- It found that dev servers now get an HTTP readiness probe (`net/http`,
  a new touchpoint).
- It found that the JSON-schema dependency moved from the skill's
  validator into the deck itself, so the deck binary now links it.
- It saw that the validator's 19 checker functions moved unchanged into
  the new `manifest` package. That is most of the "−900" on
  `skills/dev-manifest/validate`.

**PR #43, the GitHub integration** (merge 9e8c77a, at 120 columns):
- the new `internal/source/github` package, with `+ exec gh` (the deck
  now runs the GitHub CLI) and `+ os/exec`, each with its `file:line`;
- the `live` reader and `cmd` wiring it in;
- `~ api internal/ui type Options` as the only UI surface change.

**PR #31, the diff preview** (merge 71b0351):
- `internal/syntax → chroma/v2` was the top finding. Chroma had been an
  indirect dependency through glamour, so `go.mod` showed no new require
  line, only an edit to an existing one. The new edge is what reveals it.

**A synthetic commit** in a scratch clone: `internal/source/diff` imports
`internal/ui`, reads `ABC_EXPORT_TOKEN`, inserts into `export_jobs` and
calls `s3.example.com`; `internal/ui` imports `internal/source/github`.

```text
 edges +4 −0  ✕ 1 upward  ↻ 1 cycle  touch +3  api +1 −0 ~0

 ── Look here first ──
 1  + internal/source/diff → internal/ui  ✕ upward sources → ui
 2  ↻ cycle internal/source → internal/ui → internal/source
 3  + http s3.example.com  in internal/source/diff
 4  + sql  export_jobs  in internal/source/diff
 5  + internal/source/diff → database/sql  database
 6  + internal/source/diff → net/http  network
 7  + env  ABC_EXPORT_TOKEN  in internal/source/diff
 8  + internal/ui → internal/source/github

 ── Matrix ──
 from \ to entry    ui sour…  core
 entry       1     1    12    11
 ui          ·     2    +1     3
 sources     ·   ✕+1     8    10
 core        ·     ·     ·     2
```

A `git mv` of one file on its own gives `≡ pure move: no declaration or
dependency changed` and an empty *Look here first*.

**Timings** (`-time`; three runs): base graph 93–140 ms (84–91 files
parsed), head graph 22–48 ms (2–23 more blobs), diff and analysis
16–36 ms.

### What it got wrong or left out

- **Edges that moved with their code are counted as new.** In #52, the
  jsonschema edge ranks second as `+`, although its `−` twin (from the
  validator) is in the same change. The fix: when a removed edge `X → D`
  and an added edge `Y → D` coincide with declarations moving `X → Y`,
  show one "moved with its code" row, ranked low.
- **API counts rank above new internal edges.** A large `~ api` on a
  package that is being written is not news; a new edge between existing
  packages is. The ranking needs a "new package" discount.
- **`(dynamic)` targets.** Constants from other files of the package
  aren't resolved yet, and URLs built at run time can't be. Resolving
  package-wide constants is easy; the rest stays dynamic.
- **Rename detection is exact.** `ParseManifest → Parse` with an edited
  body was not paired; it needs the similarity score in §10.
- **Indirect → direct dependency promotions** are not reported as a
  `go.mod` change, only through the new edge.
- **Header line** overflows at 60 columns; the real tab would use the
  Files tab's layout code.
- At first, "changed plus one step" showed every package, because `cmd/`
  imports all of them. The fold of unchanged neighbours per lane fixed
  that. It is the main design lesson of the prototype.
- Not in the prototype: TS/Python, file zoom, before/after, the UI, IDs
  as data, verdicts, disk caching, uncommitted changes.

The prototype stays in `exp/`. It is clean enough to read, but its
rendering is plain text, and the real tab needs the UI's styles,
scrolling and goldens. The parsing and graph code (`goscan.go`,
`graph.go`, `diff.go`) is the part worth lifting into an
`internal/source/arch` package.

## 12. Recommendation

**Build it, in phases, Go first.** The cost is low because the
expensive-looking part (two dependency graphs per thread) is cheap with
blob caching. The value is highest exactly where agents work
unattended: a new dependency, a layer skip or a new env var is what a
reviewer skimming a long agent diff misses.

| Phase | Scope | Effort (thread-sized PRs) |
|---|---|---|
| 1 | `internal/source/arch` for Go (from the prototype, plus moved-edge pairing and ranking fixes); the **Arch** tab with *Look here first* and lanes; layers from `x-herdr-deck.architecture` or inferred; `↵` opens the preview at the site (new line target); cache in memory | 1 large PR, or 2 (reader, then tab) |
| 2 | Matrix view and the dense fallback; before/after; zoom; stable IDs; verdicts in the state folder; disk cache; uncommitted changes | 1 PR |
| 3 | TypeScript: import scanner and resolver (tsconfig paths, workspaces), `package.json` deps, TS touchpoint rules | 1 PR, plus a gotreesitter spike if exports are wanted |
| 4 | Python: import scanner, `pyproject`/requirements deps, rules; optional ruff cross-check | 1 PR |
| later | 120-column split with the selection; read import-linter / go-arch-lint configs; exporting verdicts to a review; drawn boxes for tiny graphs | — |

Phase 1 alone is useful on herdr-deck itself. Phases 3 and 4 depend on
which repos the user runs threads on.

### Decisions for the user

1. **Build it at all, and when.** Recommended: phase 1 after the current
   milestone work, and decide on phase 2+ after a couple of weeks of use.
2. **Languages and order.** Recommended: Go, then TypeScript, then
   Python. If most threads run on a TS or Python repo, swap 1 and 3.
3. **Static only, or traces too.** Recommended: static only for this
   tab. Traces help the call-stack tab, not import structure.
4. **AI.** Recommended: none here (see §10).
5. **Where layers are declared.** Recommended: `x-herdr-deck.architecture`
   in `.config/dev.json`. The alternative is a tool-neutral
   `x-architecture` key, if the user's Raycast extension or another tool
   might read it too.
6. **Tab name and place.** Recommended: `Arch N`, between Commits and
   Log, shown only for threads with a worktree. Alternatives: `Shape`,
   or folding it into the sibling call-stack tab as one "Impact" tab with
   two views.
7. **Verdict storage.** Recommended: the deck's state folder, keyed by
   repo, branch and ID, in the walkthrough's format. The alternative is
   writing into a walkthrough file in the repo when one exists.

## 13. Sources

All checked 2026-10-07.

- Go: [`go/parser` modes, including `ImportsOnly`](https://pkg.go.dev/go/parser#Mode);
  [`golang.org/x/tools/go/packages`](https://pkg.go.dev/golang.org/x/tools/go/packages)
  (default driver is `go list`; see
  [golist.go](https://github.com/golang/tools/blob/master/go/packages/golist.go)).
- [gotreesitter](https://github.com/odvcencio/gotreesitter): pure-Go
  tree-sitter runtime, MIT, latest tag v0.55.1 (2026-09-26).
  [smacker/go-tree-sitter](https://github.com/smacker/go-tree-sitter) and
  [tree-sitter/go-tree-sitter](https://github.com/tree-sitter/go-tree-sitter)
  both need CGO.
- esbuild: [Go API](https://pkg.go.dev/github.com/evanw/esbuild/pkg/api) v0.28.2,
  [metafile](https://esbuild.github.io/api/#metafile),
  [TypeScript caveats](https://esbuild.github.io/content-types/#typescript-caveats)
  (unused and type-only imports removed),
  [tsconfig support](https://esbuild.github.io/content-types/#tsconfig-json),
  [go.mod](https://github.com/evanw/esbuild/blob/main/go.mod).
- [typescript-go](https://github.com/microsoft/typescript-go): parser in
  `internal/`, API "not ready".
- ruff `analyze graph`: [settings](https://docs.astral.sh/ruff/settings/#analyze),
  [experimental notice](https://github.com/astral-sh/ruff/blob/main/crates/ruff/src/commands/analyze_graph.rs),
  [CLI args](https://github.com/astral-sh/ruff/blob/main/crates/ruff/src/args.rs);
  run locally with ruff 0.16.1.
- [gpython](https://github.com/go-python/gpython): Python 3.4.
- LSP call hierarchy:
  [gopls](https://github.com/golang/tools/blob/master/gopls/doc/features/navigation.md),
  [pyright](https://github.com/microsoft/pyright/blob/main/packages/pyright-internal/src/languageServerBase.ts),
  [ty](https://github.com/astral-sh/ruff/blob/main/crates/ty_server/src/capabilities.rs),
  [pyrefly](https://github.com/facebook/pyrefly/blob/main/pyrefly/lib/lsp/non_wasm/server.rs),
  [typescript-language-server](https://github.com/typescript-language-server/typescript-language-server/blob/master/src/lsp-server.ts).
- Layer declarations:
  [import-linter layers](https://github.com/seddonym/import-linter/blob/main/docs/contract_types/layers.md),
  [go-arch-lint](https://github.com/fe3dback/go-arch-lint),
  [dependency-cruiser rules](https://github.com/sverweij/dependency-cruiser/blob/main/doc/rules-reference.md),
  [ArchUnit](https://www.archunit.org/userguide/html/000_Index.html),
  [depguard](https://golangci-lint.run/docs/linters/configuration/#depguard).
- The idea's origin: notes from the user's conversation with another
  agent (2026-10-07). They were treated as ideas; every tool claim above
  was checked separately.
