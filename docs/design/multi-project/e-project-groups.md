# E · Projects as groups in design D

Today's deck with one more level on top: every project is a foldable group,
and TASKS.md's lists sit inside it as they do now. The rows, the drawer, the
numbered links and the keys are design D's. What needs you, in any project,
moves into one *Needs you* group above all projects, so a folded project
never hides it.

Shared sample data and conventions are in the [README](README.md#shared-conventions).

**Where it opens.** Wherever the deck opens today, and also on its own:

- Next to a coordinator (auto-open and the toggle, as now): that project
  starts open and every other project starts folded to its heading. So this
  direction **replaces** the per-coordinator deck: it is the same deck, with
  the other projects one line each below.
- A new action `herdr-deck.all` (bind it to a key) opens it as a herdr tab of
  its own (`placement = "tab"`), with every project open. `herdr-deck --all`
  does the same in any terminal.

**Across projects.** *Needs you* collects waiting threads and unhandled
inbox items from every project; each row starts with the project's short
name. `↵` focuses the thread's pane in whatever workspace it is. Links work as
today, from the drawer. The Linear workspace is still one setting, which
breaks down if two projects use different workspaces (see the README).

**Keys added.** `space` on a project heading folds it, `[`/`]` jump to the
previous/next project heading, `A` shows archived projects, and `↵` on a
project heading focuses its coordinator. Everything else is D's.

## Normal use

Opened next to *Admin rebuild*'s coordinator, after *Billing export* was
unfolded by hand. 80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ All projects                                           ◐ 3  ◇ 2  ↻ 1  ○ 2  ✉ 1 │
│────────────────────────────────────────────────────────────────────────────────│
│    WORK                          THREAD  STATUS              PR      LINKS  DEV│
│ Admin rebuild                                                    ◐ 2  ◇ 1  ○ 1 │
│  In progress                                                                   │
│ ▸◐ Users page                    t-0002  Building overview           L4        │
│  ◐ Templates page                t-0003  Writing tests               L F    ●●●│
│  ◇ Document select for summary   t-0004  review required     #2320   L F3   ●○ │
│  On hold until Monday 2026-10-05                                               │
│  ○ Subscriptions list            t-0001  idle                        L N       │
│ Billing export                                                   ◐ 1  ◇ 1  ↻ 1 │
│  In progress                                                                   │
│  ◐ CSV export job                t-0003  Streaming rows              L      ●● │
│  ◇ Invoice PDF fonts             t-0002  checks ✕ 1          #418    L         │
│  ↻ Ledger totals fix             t-0004  landing             #415    L2        │
│  + Backlog (3)                                                                 │
│ + Docs site                                                           ○ 1  ✉ 1 │
│ + Search spike · paused                                             no threads │
│─ Users page · Admin rebuild ───────────────────────────────────────────────────│
│ Status   working · Building overview · ~60% · 4m                               │
│ Linear   1 ABC-1246   2 ABC-1256   3 ABC-1257   4 ABC-1250                     │
│                                                                                │
│                                                                                │
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ 1-9 link  ↵ pane  space fold  [ ] project  A archived  ? help                  │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns:

```text
┌────────────────────────────────────────────────────────────┐
│ All projects                       ◐ 3  ◇ 2  ↻ 1  ○ 2  ✉ 1 │
│────────────────────────────────────────────────────────────│
│    WORK                       STATUS           LINKS  DEV  │
│ Admin rebuild                                ◐ 2  ◇ 1  ○ 1 │
│  In progress                                               │
│ ▸◐ Users page                 Building overv…  L4          │
│  ◐ Templates page             Writing tests    L F    ●●●  │
│  ◇ Document select for summ…  #2320 review     L F3   ●○   │
│  On hold until Monday 2026-10-05                           │
│  ○ Subscriptions list         idle             L N         │
│ Billing export                               ◐ 1  ◇ 1  ↻ 1 │
│  In progress                                               │
│  ◐ CSV export job             Streaming rows   L      ●●   │
│  ◇ Invoice PDF fonts          #418 ✕ 1         L           │
│  ↻ Ledger totals fix          #415 landing     L2          │
│  + Backlog (3)                                             │
│ + Docs site                                       ○ 1  ✉ 1 │
│ + Search spike · paused                         no threads │
│─ Users page · t-0002 ──────────────────────────────────────│
│ Status  working · Building overview · ~60% · 4m            │
│ Linear  1 ABC-1246  2 ABC-1256  3 ABC-1257  4 ABC-1250     │
│                                                            │
│                                                            │
│                                                            │
│────────────────────────────────────────────────────────────│
│ 1-9 link  ↵ pane  space fold  [ ] project  ? help          │
└────────────────────────────────────────────────────────────┘
```

- **Project headings** are bold at column 1, with the project's thread counts
  on the right; list headings stay dim at column 2. A folded project starts
  with `+` and keeps its counts, so `○ 1  ✉ 1` on *Docs site* still says
  there is an inbox item.
- **The header** sums every project that is not archived. A paused project
  counts, but its heading says `paused` (the ticker skips it, so its state can
  be old).
- **Folding is remembered** per project for the session, not on disk.

120 columns, as a herdr tab of its own (`herdr-deck.all`). The drawer moves
to the right, so the list keeps the full height and the THREAD column comes
back:

```text
┌────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│ All projects · 4 active, 1 paused                                                              ◐ 3  ◇ 2  ↻ 1  ○ 2  ✉ 1 │
│────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────│
│    WORK                         THREAD  STATUS            PR     LINKS   │─ Users page · Admin rebuild ────────────────│
│ Admin rebuild · acme/webshop                               ◐ 2  ◇ 1  ○ 1 │                                             │
│  In progress                                                             │ Thread   t-0002 Members /admin/users        │
│ ▸◐ Users page                   t-0002  Building overvi…         L4      │ Pane     w1Z:p1                             │
│  ◐ Templates page               t-0003  Writing tests            L F ●●● │ Status   working · Building overview        │
│  ◇ Document select for summary  t-0004  review required   #2320  L F3 ●○ │          ~60% · 4m                          │
│  On hold until Monday 2026-10-05                                         │ Note     Phase 1 = layout + overview        │
│  ○ Subscriptions list           t-0001  idle                     L N     │          (ABC-1256); report on 1257/1250    │
│ Billing export · acme/ledger                               ◐ 1  ◇ 1  ↻ 1 │          before starting them.              │
│  In progress                                                             │ Linear   1 ABC-1246   2 ABC-1256            │
│  ◐ CSV export job               t-0003  Streaming rows           L ●●    │          3 ABC-1257   4 ABC-1250            │
│  ◇ Invoice PDF fonts            t-0002  checks ✕ 1        #418   L       │ Branch   hp/admin-rebuild/t-0002-members-…  │
│  ↻ Ledger totals fix            t-0004  landing           #415   L2      │ Dev      none running (frontend, api, pg)   │
│  + Backlog (3)                                                           │                                             │
│ Docs site · acme/docs                                           ○ 1  ✉ 1 │                                             │
│  Inbox                                                                   │                                             │
│  ✉ inbox: t-0001 is now Idle: report written                          2d │                                             │
│  Other threads                                                           │                                             │
│  ○ Search page                  t-0001  idle                     L       │                                             │
│  + Resolved (1)                                                          │                                             │
│ + Search spike · paused                                       no threads │                                             │
│                                                                          │                                             │
│────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────│
│ j k move  1-9 link  l f n g o first  ↵ pane  e edit  space fold  [ ] project  A archived  z drawer  ? help             │
└────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

- At 120 columns the DEV dots join the LINKS column, and the drawer is a
  45-column side panel. Below about 110 columns the drawer goes back under
  the list.
- In a full tab, unhandled inbox items show inside their project (*Docs
  site* above) as well as in *Needs you* when something is pinned there.

## Something needs you in another project

Same deck, still opened next to *Admin rebuild*. *Billing export*'s t-0003
now waits on you, and *Billing export* is folded. 80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ All projects                            ● 1 needs you  ◐ 2  ◇ 2  ↻ 1  ○ 2  ✉ 2 │
│────────────────────────────────────────────────────────────────────────────────│
│    WORK                          THREAD  STATUS              PR      LINKS  DEV│
│  Needs you                                                                     │
│ ▸● Billing · CSV export job      t-0003  needs you · 2m              L      ●● │
│  ✉ Billing · inbox: t-0003 is now Waiting on you                         2m    │
│ Admin rebuild                                                    ◐ 2  ◇ 1  ○ 1 │
│  In progress                                                                   │
│  ◐ Users page                    t-0002  Building overview           L4        │
│  ◐ Templates page                t-0003  Writing tests               L F    ●●●│
│  ◇ Document select for summary   t-0004  review required     #2320   L F3   ●○ │
│  On hold until Monday 2026-10-05                                               │
│  ○ Subscriptions list            t-0001  idle                        L N       │
│ + Billing export                                                 ● 1  ◇ 1  ↻ 1 │
│ + Docs site                                                           ○ 1  ✉ 1 │
│ + Search spike · paused                                             no threads │
│─ CSV export job · Billing export ──────────────────────────────────────────────│
│ Thread   t-0003 CSV export /exports · pane w4K:p3                              │
│ Status   needs you · ~70% · since 14:31                                        │
│ Next     → Choose: one CSV per account, or one zip for all (BIL-210)           │
│          → Confirm a 50 000-row cap per file                                   │
│ Linear   1 BIL-210                                                             │
│                                                                                │
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ ↵ go to pane  1-9 link  r report  [ ] project  z drawer  ? help                │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns:

```text
┌────────────────────────────────────────────────────────────┐
│ All projects                  ● 1  ◐ 2  ◇ 2  ↻ 1  ○ 2  ✉ 2 │
│────────────────────────────────────────────────────────────│
│    WORK                       STATUS           LINKS  DEV  │
│  Needs you                                                 │
│ ▸● Billing · CSV export job   needs you · 2m   L      ●●   │
│  ✉ Billing · t-0003 is now Waiting on you               2m │
│ Admin rebuild                                ◐ 2  ◇ 1  ○ 1 │
│  In progress                                               │
│  ◐ Users page                 Building overv…  L4          │
│  ◐ Templates page             Writing tests    L F    ●●●  │
│  ◇ Document select for summ…  #2320 review     L F3   ●○   │
│  On hold until Monday 2026-10-05                           │
│  ○ Subscriptions list         idle             L N         │
│ + Billing export                             ● 1  ◇ 1  ↻ 1 │
│ + Docs site                                       ○ 1  ✉ 1 │
│ + Search spike · paused                         no threads │
│─ CSV export job · Billing export ──────────────────────────│
│ Status  needs you · ~70% · since 14:31                     │
│ Next    → Choose: one CSV per account, or one zip          │
│           for all (BIL-210)                                │
│         → Confirm a 50 000-row cap per file                │
│ Linear  1 BIL-210                                          │
│                                                            │
│                                                            │
│────────────────────────────────────────────────────────────│
│ ↵ go to pane  1-9 link  r report  ? help                   │
└────────────────────────────────────────────────────────────┘
```

- **One pinned group for everything.** A row in *Needs you* starts with the
  project's short name (`Billing`, bold, in the row's colour): the first word
  of PROJECT.md's `name`, longer when two projects would share it. The
  folded project heading turns its count red (`● 1`).
- **The cursor jumps** to the first pinned row, as in D, unless it was moved
  by hand. That means a deck next to *Admin rebuild*'s coordinator now shows
  another project's question in its drawer. That is the point of this
  direction, and also its main risk: see Good and bad.
- **`↵`** focuses t-0003's pane, which lives in *Billing export*'s workspace,
  so herdr switches workspace (`pane.focus` takes any pane id; switching
  workspaces this way is to be checked in herdr 0.9.3).

## An empty project, and archived projects

`A` shows archived projects at the bottom; the cursor is on *Search spike*,
which is paused and has no tasks or threads yet. 80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ All projects · 1 archived shown                        ◐ 3  ◇ 2  ↻ 1  ○ 2  ✉ 1 │
│────────────────────────────────────────────────────────────────────────────────│
│    WORK                          THREAD  STATUS              PR      LINKS  DEV│
│ + Admin rebuild                                                  ◐ 2  ◇ 1  ○ 1 │
│ + Billing export                                                 ◐ 1  ◇ 1  ↻ 1 │
│ + Docs site                                                           ○ 1  ✉ 1 │
│▸Search spike · paused                                               no threads │
│    Nothing yet. Rows appear when the coordinator writes TASKS.md               │
│    or starts a thread.                                                         │
│  Archived                                                                      │
│ + Mobile onboarding                                                 resolved 6 │
│─ Search spike ─────────────────────────────────────────────────────────────────│
│ Status   paused: the ticker skips it, and threads cannot start                 │
│ Goal     Try two search engines on the docs corpus and write down which        │
│          one to keep.                                                          │
│ Repos    ~/src/docs                                                            │
│ Coord    no coordinator running                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ ↵ coordinator  space fold  [ ] project  A hide archived  ? help                │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns:

```text
┌────────────────────────────────────────────────────────────┐
│ All projects · 1 archived          ◐ 3  ◇ 2  ↻ 1  ○ 2  ✉ 1 │
│────────────────────────────────────────────────────────────│
│    WORK                       STATUS           LINKS  DEV  │
│ + Admin rebuild                              ◐ 2  ◇ 1  ○ 1 │
│ + Billing export                             ◐ 1  ◇ 1  ↻ 1 │
│ + Docs site                                       ○ 1  ✉ 1 │
│▸Search spike · paused                           no threads │
│    Nothing yet. Rows appear when the coordinator           │
│    writes TASKS.md or starts a thread.                     │
│  Archived                                                  │
│ + Mobile onboarding                             resolved 6 │
│─ Search spike ─────────────────────────────────────────────│
│ Status  paused: the ticker skips it                        │
│ Goal    Try two search engines on the docs corpus          │
│         and write down which one to keep.                  │
│ Repos   ~/src/docs                                         │
│ Coord   none running                                       │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│────────────────────────────────────────────────────────────│
│ ↵ coordinator  space fold  A hide archived  ? help         │
└────────────────────────────────────────────────────────────┘
```

- **A selected project heading** fills the drawer with the project itself,
  like D's empty project: status, goal, repos, and its coordinator (from
  herdr's `hp_group` token `<slug>!0!<pane>`, else `.state/coordinator.json`).
- **Paused** projects stay in the list; **archived** ones are hidden, like
  `herdr-projects list` without `--all`, until `A`. They sit under a dim
  *Archived* heading and never count in the header.
- With no coordinator running, `↵` only says so. Starting one
  (`herdr-projects open <slug>`) is the user's call; see the README's open
  questions.

## A data source is missing

The herdr socket is gone, *Billing export* has no `.state/ticker.json`, and
`herdr-projects thread list docs-site --json` fails because its PROJECT.md
does not parse. 80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ All projects                                      ◐ 3  ◇ 1  ↻ 1  ○ 1  ✉ 1  ! 3 │
│────────────────────────────────────────────────────────────────────────────────│
│    WORK                          THREAD  STATUS              PR      LINKS  DEV│
│ Admin rebuild                                                    ◐ 2  ◇ 1  ○ 1 │
│  In progress                                                       as of 14:23 │
│ ▸◐ Users page                    t-0002  Building overview           L4        │
│  ◐ Templates page                t-0003  Writing tests               L F    ●●●│
│  ◇ Document select for summary   t-0004  review required     #2320   L F3   ●○ │
│  On hold until Monday 2026-10-05                                               │
│  ○ Subscriptions list            t-0001  idle                        L N       │
│ Billing export                                                   ◐ 1  ◇ 1  ↻ 1 │
│  In progress                                                                   │
│  ◐ CSV export job                t-0003  Streaming rows              L      ●● │
│  ◇ Invoice PDF fonts             t-0002  ?                   #418    L         │
│  ↻ Ledger totals fix             t-0004  landing · ?         #415    L2        │
│ ! Docs site                                            threads unreadable  ✉ 1 │
│ + Search spike · paused                                             no threads │
│─ Sources ──────────────────────────────────────────────────────────────────────│
│ ! herdr socket     not found: ↵ cannot focus panes, status from files only     │
│ ! Billing export   no .state/ticker.json: no PR review or checks               │
│ ! Docs site        thread list failed: PROJECT.md line 4 does not parse        │
│ ✓ everything else                                                   read 14:41 │
│                                                                                │
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ 1-9 link  l f n g o first  e edit  [ ] project  ? help                         │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns:

```text
┌────────────────────────────────────────────────────────────┐
│ All projects                  ◐ 3  ◇ 1  ↻ 1  ○ 1  ✉ 1  ! 3 │
│────────────────────────────────────────────────────────────│
│    WORK                       STATUS           LINKS  DEV  │
│ Admin rebuild                                ◐ 2  ◇ 1  ○ 1 │
│  In progress                                   as of 14:23 │
│ ▸◐ Users page                 Building overv…  L4          │
│  ◐ Templates page             Writing tests    L F    ●●●  │
│  ◇ Document select for summ…  #2320 review     L F3   ●○   │
│  On hold until Monday 2026-10-05                           │
│  ○ Subscriptions list         idle             L N         │
│ Billing export                               ◐ 1  ◇ 1  ↻ 1 │
│  In progress                                               │
│  ◐ CSV export job             Streaming rows   L      ●●   │
│  ◇ Invoice PDF fonts          #418 ?           L           │
│  ↻ Ledger totals fix          #415 landing ?   L2          │
│ ! Docs site                        threads unreadable  ✉ 1 │
│ + Search spike · paused                         no threads │
│─ Sources ──────────────────────────────────────────────────│
│ ! herdr socket    missing: no pane focus                   │
│ ! Billing export  no ticker.json: no PR checks             │
│ ! Docs site       PROJECT.md line 4 does not parse         │
│ ✓ everything else                               read 14:41 │
│                                                            │
│                                                            │
│────────────────────────────────────────────────────────────│
│ 1-9 link  l f n g o first  e edit  ? help                  │
└────────────────────────────────────────────────────────────┘
```

- **Global and per-project sources** share the *Sources* view: the herdr
  socket is one for all projects, and so is the herdr-projects ticker (when
  it stops, every project goes stale at once). Per-project problems are
  named by project.
- **An unreadable project** keeps its heading with a yellow `!` and what is
  still known: here the inbox count, which comes from `inbox/*.md` and not
  from the thread list.

## Good and bad

| | |
|---|---|
| Scanning | Very good for the projects you keep open; a folded project is one line with its counts. |
| Needs you | Very good: one pinned group across everything, and it can't be folded away. |
| Next to a coordinator | Mixed. It shows the other projects for free, but they cost rows (one per project) and the cursor may jump to another project's question. |
| Many projects | Fair. Ten folded projects are ten rows, the same as ten tasks; past that the list scrolls. |
| Narrow width | As D: fine at 60, with project short names eating into the WORK column in *Needs you*. |
| Keys | Few new ones (`[`/`]`, `A`); `space` already folds. |
| Data cost | Highest: every project's threads, ticker, inbox and TASKS.md on every refresh, also for folded projects (their counts need them). |
| Build cost | Low to medium: D's list gains one level; the reader runs once per project; the drawer is unchanged. |
