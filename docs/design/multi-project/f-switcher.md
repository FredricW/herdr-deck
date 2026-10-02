# F · Project switcher over today's deck

Today's deck, unchanged, for one project at a time, with a strip of project
tabs above it. `[` and `]` (or a click on a tab) switch projects, and `p`
opens a picker for when the strip is full. The strip carries each project's
counts, so a project you are not looking at can still turn red.

Shared sample data and conventions are in the [README](README.md#shared-conventions).

**Where it opens.** Where the deck opens today: next to a coordinator, on
that coordinator's project. Every deck gets the strip, so there is nothing
new to open. A global key could also open it as a herdr popup
(`placement = "popup"`), starting on the project that needs you most.

**How it relates to the per-coordinator deck.** It **replaces** it: same
deck, plus a strip. The deck has a *home* project (the coordinator's next to
it) and a *shown* project. When they differ, the header says so, and `esc`
or `h` returns home.

**Across projects.** Needs-you, inbox and links stay per project, exactly as
today, inside the shown project. Other projects only show up in the strip:
red with `●` when something there needs you, yellow `✉` for an unhandled
inbox item. A red band under the strip names the newest need elsewhere, so
you know which tab to press.

**Keys added.** `[`/`]` previous/next project, `p` picker, `h` home
project, `1`–`9` stay link digits (projects are not numbered).

## Normal use

80 columns, on *Admin rebuild* (home):

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Admin rebuild ◐2 ◇1 │ Billing export ◐1 ◇1 ↻1 │ Docs site ✉1 │ Search spike    │
│────────────────────────────────────────────────────────────────────────────────│
│    WORK                          THREAD  STATUS              PR      LINKS  DEV│
│  In progress                                                                   │
│ ▸◐ Users page                    t-0002  Building overview           L4        │
│  ◐ Templates page                t-0003  Writing tests               L F    ●●●│
│  ◇ Document select for summary   t-0004  review required     #2320   L F3   ●○ │
│  On hold until Monday 2026-10-05                                               │
│  ○ Subscriptions list            t-0001  idle                        L N       │
│  + Backlog (6)                                                                 │
│                                                                                │
│─ Users page ───────────────────────────────────────────────────────────────────│
│ Thread   t-0002 Members /admin/users · pane w1Z:p1                             │
│ Status   working · Building overview · ~60% · 4m                               │
│ Note     Phase 1 = layout + overview (ABC-1256); report on 1257/1250           │
│          before starting them.                                                 │
│ Linear   1 ABC-1246   2 ABC-1256   3 ABC-1257   4 ABC-1250                     │
│ Branch   hp/admin-rebuild/t-0002-members-admin-users                           │
│ Dev      no servers running (dev.json: frontend, api, pg)                      │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ [ ] project  p pick  1-9 link  l f n g o first  ↵ pane  z drawer  ? help       │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns:

```text
┌────────────────────────────────────────────────────────────┐
│ Admin rebuild ◐2 ◇1 │ Billing ◐1 ◇1 ↻1 │ Docs ✉1 │ +1      │
│────────────────────────────────────────────────────────────│
│    WORK                       STATUS           LINKS  DEV  │
│  In progress                                               │
│ ▸◐ Users page                 Building overv…  L4          │
│  ◐ Templates page             Writing tests    L F    ●●●  │
│  ◇ Document select for summ…  #2320 review     L F3   ●○   │
│  On hold until Monday 2026-10-05                           │
│  ○ Subscriptions list         idle             L N         │
│  + Backlog (6)                                             │
│                                                            │
│─ Users page · t-0002 ──────────────────────────────────────│
│ Status  working · Building overview · ~60% · 4m            │
│ Note    Phase 1 = layout + overview (ABC-1256); report     │
│         on 1257/1250 before starting them.                 │
│ Linear  1 ABC-1246  2 ABC-1256  3 ABC-1257  4 ABC-1250     │
│ Pane    w1Z:p1 · Members /admin/users                      │
│ Dev     none running                                       │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│────────────────────────────────────────────────────────────│
│ [ ] project  p pick  1-9 link  ↵ pane  ? help              │
└────────────────────────────────────────────────────────────┘
```

- **The strip replaces the header line.** The shown project's tab is bold on
  the selection grey; the others are dim, with their counts in the state
  colours. Tabs keep the order `herdr-projects list` prints (by slug), so
  they do not jump around when states change.
- **At 60 columns** other tabs shorten to the first word of the name, and
  tabs that do not fit fold into `+1` (a click or `p` opens the picker).
  Counts show for every tab that fits; a tab with `●` is never folded away
  (it takes the place of a quiet one).
- **The rest is today's deck**, row for row: the strip costs no row, because
  the project name moves into it.

## Something needs you in another project

*Billing export*'s t-0003 waits on you while the deck shows *Admin
rebuild*. 80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Admin rebuild ◐2 ◇1 │ Billing export ●1 ◇1 ↻1 │ Docs site ✉1 │ Search spike    │
│ ● Billing export · CSV export job needs you · 2m                       ] to go │
│────────────────────────────────────────────────────────────────────────────────│
│    WORK                          THREAD  STATUS              PR      LINKS  DEV│
│  In progress                                                                   │
│ ▸◐ Users page                    t-0002  Building overview           L4        │
│  ◐ Templates page                t-0003  Writing tests               L F    ●●●│
│  ◇ Document select for summary   t-0004  review required     #2320   L F3   ●○ │
│  On hold until Monday 2026-10-05                                               │
│  ○ Subscriptions list            t-0001  idle                        L N       │
│  + Backlog (6)                                                                 │
│─ Users page ───────────────────────────────────────────────────────────────────│
│ Thread   t-0002 Members /admin/users · pane w1Z:p1                             │
│ Status   working · Building overview · ~60% · 4m                               │
│ Linear   1 ABC-1246   2 ABC-1256   3 ABC-1257   4 ABC-1250                     │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ ] Billing export  [ ] project  p pick  1-9 link  ↵ pane  ? help                │
└────────────────────────────────────────────────────────────────────────────────┘
```

After `]`, 80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Admin rebuild ◐2 ◇1 │ Billing export ●1 ◇1 ↻1 │ Docs site ✉1 │ Search spike    │
│────────────────────────────────────────────────────────────────────────────────│
│    WORK                          THREAD  STATUS              PR      LINKS  DEV│
│  Needs you                                                                     │
│ ▸● CSV export job                t-0003  needs you · 2m              L      ●● │
│  ✉ inbox: t-0003 is now Waiting on you                                   2m    │
│  In progress                                                                   │
│  ◇ Invoice PDF fonts             t-0002  checks ✕ 1          #418    L         │
│  ↻ Ledger totals fix             t-0004  landing             #415    L2        │
│  + Backlog (3)                                                                 │
│─ CSV export job · not this tab's project (h home) ─────────────────────────────│
│ Thread   t-0003 CSV export /exports · pane w4K:p3                              │
│ Status   needs you · ~70% · since 14:31                                        │
│ Next     → Choose: one CSV per account, or one zip for all (BIL-210)           │
│          → Confirm a 50 000-row cap per file                                   │
│ Linear   1 BIL-210                                                             │
│ Branch   hp/billing-export/t-0003-csv-export-job                               │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ ↵ go to pane  h home  [ ] project  1-9 link  r report  ? help                  │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns, before and after `]`:

```text
┌────────────────────────────────────────────────────────────┐
│ Admin rebuild ◐2 ◇1 │ Billing ●1 ◇1 ↻1 │ Docs ✉1 │ +1      │
│ ● Billing · CSV export job needs you · 2m             ] go │
│────────────────────────────────────────────────────────────│
│    WORK                       STATUS           LINKS  DEV  │
│  In progress                                               │
│ ▸◐ Users page                 Building overv…  L4          │
│  ◐ Templates page             Writing tests    L F    ●●●  │
│  ◇ Document select for summ…  #2320 review     L F3   ●○   │
│  On hold until Monday 2026-10-05                           │
│  ○ Subscriptions list         idle             L N         │
│  + Backlog (6)                                             │
│─ Users page · t-0002 ──────────────────────────────────────│
│ Status  working · Building overview · ~60% · 4m            │
│ Linear  1 ABC-1246  2 ABC-1256  3 ABC-1257  4 ABC-1250     │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│────────────────────────────────────────────────────────────│
│ ] Billing  p pick  1-9 link  ↵ pane  ? help                │
└────────────────────────────────────────────────────────────┘
```

```text
┌────────────────────────────────────────────────────────────┐
│ Admin rebuild ◐2 ◇1 │ Billing ●1 ◇1 ↻1 │ Docs ✉1 │ +1      │
│────────────────────────────────────────────────────────────│
│    WORK                       STATUS           LINKS  DEV  │
│  Needs you                                                 │
│ ▸● CSV export job             needs you · 2m   L      ●●   │
│  ✉ inbox: t-0003 is now Waiting on you                  2m │
│  In progress                                               │
│  ◇ Invoice PDF fonts          #418 ✕ 1         L           │
│  ↻ Ledger totals fix          #415 landing     L2          │
│  + Backlog (3)                                             │
│─ CSV export job · t-0003 · away ───────────────────────────│
│ Status  needs you · ~70% · since 14:31                     │
│ Next    → Choose: one CSV per account, or one zip          │
│           for all (BIL-210)                                │
│         → Confirm a 50 000-row cap per file                │
│ Linear  1 BIL-210                                          │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│────────────────────────────────────────────────────────────│
│ ↵ go to pane  h home  [ ] project  ? help                  │
└────────────────────────────────────────────────────────────┘
```

- **The band** (bold red, one row) appears only when something needs you in
  a project other than the shown one, and names the newest such thread or
  inbox item; with several it says `● 3 in 2 projects · ] next`. `]` jumps
  straight to the next project with a need, not just the next tab, while the
  band is up.
- **Away from home**, the drawer title says so (`not this tab's project`,
  `away` at 60), so a deck beside one coordinator showing another project's
  rows is never mistaken for its own.
- The band costs a row while it is up, and the list shifts down by one.

## An empty project, and archived projects: the picker

`p` opens the picker over the deck. It lists every project with its counts;
`A` adds archived ones. 80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Admin rebuild ◐2 ◇1 │ Billing export ◐1 ◇1 ↻1 │ Docs site ✉1 │ Search spike    │
│────────────────────────────────────────────────────────────────────────────────│
│  ┌ Projects ─────────────────────────────────────────────────────────┐         │
│  │ > _                                                               │         │
│  │   Admin rebuild       home   ◐ 2  ◇ 1  ○ 1                        │         │
│  │   Billing export             ◐ 1  ◇ 1  ↻ 1                        │         │
│  │   Docs site                  ○ 1  ✉ 1                             │         │
│  │ ▸ Search spike               paused · no threads                  │         │
│  │   Archived                                                        │         │
│  │   Mobile onboarding          archived · resolved 6                │         │
│  └───────────────────────────────────────────────────────────────────┘         │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ type to filter  ↵ show  A hide archived  esc close                             │
└────────────────────────────────────────────────────────────────────────────────┘
```

↵ on *Search spike*, 60 columns:

```text
┌────────────────────────────────────────────────────────────┐
│ Admin rebuild ◐2 │ Billing ◐1 │ Docs ✉1 │ Search spike     │
│────────────────────────────────────────────────────────────│
│    WORK                       STATUS           LINKS  DEV  │
│                                                            │
│  Nothing yet. Rows appear when the coordinator writes      │
│  TASKS.md or starts a thread.                              │
│                                                            │
│─ Project · away (h home) ──────────────────────────────────│
│ Status  paused: the ticker skips it                        │
│ Goal    Try two search engines on the docs corpus          │
│         and write down which one to keep.                  │
│ Repos   ~/src/docs                                         │
│ Folder  ~/.herdr-projects/search-spike                     │
│ Coord   none running                                       │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│────────────────────────────────────────────────────────────│
│ h home  [ ] project  p pick  ? help                        │
└────────────────────────────────────────────────────────────┘
```

- **Archived projects** never get a tab; the picker shows them only after
  `A`, and showing one gives it a temporary tab until you switch away.
- **The empty project** is today's empty deck: the drawer shows the project,
  now with its paused status and whether a coordinator runs.
- The picker is a small overlay in the deck's own pane, not a herdr popup.

## A data source is missing

The herdr socket is gone, *Billing export* has no `.state/ticker.json`, and
*Docs site*'s thread list fails. The deck shows *Billing export*.
80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Admin rebuild ◐2 ◇1 │ Billing export ◐1 ◇1 ↻1 ! │ Docs site ! │ Search spike   │
│────────────────────────────────────────────────────────────────────────────────│
│    WORK                          THREAD  STATUS              PR      LINKS  DEV│
│  In progress                                                       as of 14:23 │
│ ▸◐ CSV export job                t-0003  Streaming rows              L      ●● │
│  ◇ Invoice PDF fonts             t-0002  ?                   #418    L         │
│  ↻ Ledger totals fix             t-0004  landing · ?         #415    L2        │
│  + Backlog (3)                                                                 │
│                                                                                │
│─ Sources · Billing export ─────────────────────────────────────────────────────│
│ ! ticker.json    missing: no PR review, comments or checks                     │
│ ! herdr socket   not found (all projects): ↵ cannot focus panes                │
│ ✓ PROJECT.md, TASKS.md, inbox/, thread list --json                  read 14:41 │
│ Other projects: Docs site has 1 problem (] to see it)                          │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ [ ] project  p pick  1-9 link  l f n g o first  ? help                         │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns:

```text
┌────────────────────────────────────────────────────────────┐
│ Admin rebuild ◐2 │ Billing ◐1 ◇1 ↻1 ! │ Docs ! │ +1        │
│────────────────────────────────────────────────────────────│
│    WORK                       STATUS           LINKS  DEV  │
│  In progress                                   as of 14:23 │
│ ▸◐ CSV export job             Streaming rows   L      ●●   │
│  ◇ Invoice PDF fonts          #418 ?           L           │
│  ↻ Ledger totals fix          #415 landing ?   L2          │
│  + Backlog (3)                                             │
│                                                            │
│─ Sources · Billing export ─────────────────────────────────│
│ ! ticker.json   missing: no PR checks                      │
│ ! herdr socket  missing everywhere: no pane focus          │
│ ✓ project files, thread list                    read 14:41 │
│ Docs site: 1 problem (] to see it)                         │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│────────────────────────────────────────────────────────────│
│ [ ] project  p pick  1-9 link  ? help                      │
└────────────────────────────────────────────────────────────┘
```

- **A tab with a problem** gets a yellow `!`. A project whose threads cannot
  be read shows only `!` (*Docs site*): its counts would be guesses.
- **The global sources** (herdr socket, the herdr-projects ticker) show in
  every project's *Sources* view, marked as affecting all projects.

## Good and bad

| | |
|---|---|
| Scanning | Good for one project, as today. Other projects are a few characters each in the strip. |
| Needs you | Fair: the band and a red tab tell you *that* and *where*, but the question itself is one key away. |
| Next to a coordinator | Very good: it opens on its own project and looks like today's deck until something happens elsewhere. |
| Many projects | Fair. Past three or four tabs the strip folds into `+N` and the picker takes over. |
| Narrow width | Good: the strip shortens names, the deck is today's 60-column layout. |
| Keys | Few: `[`/`]`, `p`, `h`. |
| Confusion risk | A deck beside one coordinator can show another project; the "away" marking has to be clear. |
| Data cost | Medium: full data for the shown project, counts for the others (`herdr-projects list` gives group counts per project in one call; inbox counts are a folder listing). |
| Build cost | Lowest: a strip, a picker, and one reader per project; the deck itself does not change. |
