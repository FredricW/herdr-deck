# H · A needs-you inbox across everything

Not a project view at all: one feed of the things that want you, from every
project, sorted by how much they want you. Threads waiting on you, unhandled
inbox items, PRs ready for review or with failing checks, and threads
landing. Everything else is folded into one *Quiet* line per project at the
bottom. It is a triage list: work down it, and each `↵` takes you to the pane
that wants you.

Shared sample data and conventions are in the [README](README.md#shared-conventions).

**Where it opens.** From a global key, as a herdr popup over whatever you
are doing (`placement = "popup"`, about 80 % of the screen; `esc` closes
it), or as a herdr tab of its own if you keep it open. `herdr-deck --inbox`
runs it in any terminal. It never auto-opens.

**How it relates to the per-coordinator deck.** It **complements** it and
shares nothing with it on screen: the deck next to a coordinator stays
today's deck. The feed's drawer is D's drawer, so a row shows the same
detail in both.

**Across projects.** This is the direction that is *only* about across
projects. Needs-you and inbox items from every project are the list itself.
Links work from the drawer, as in D. Quiet projects are reachable (`↵` on a
*Quiet* line focuses that project's coordinator), but their tasks are not
shown.

**Keys added.** `↵` focus the row's pane (thread, or the inbox item's
subject thread, or the coordinator), `c` focus the row's coordinator, `q`
show or hide the *Quiet* lines.

## Normal use

Nothing waits on you; two PRs are ready, one is landing. 80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ For you                                             ◇ 2  ↻ 1  ✉ 1   3 projects │
│────────────────────────────────────────────────────────────────────────────────│
│    PROJECT          WHAT                             STATE           AGE       │
│  Ready for review                                                              │
│ ▸◇ Billing export   Invoice PDF fonts                #418 ✕ 1        20m       │
│  ◇ Admin rebuild    Document select for summary      #2320 ✓         1h        │
│                                                                                │
│  Landing                                                                       │
│  ↻ Billing export   Ledger totals fix                #415 ⋯          5m        │
│                                                                                │
│  Inbox                                                                         │
│  ✉ Docs site        t-0001 is now Idle: report written               2d        │
│                                                                                │
│  Quiet                                                                         │
│   Admin ◐ 2  ○ 1 · Billing ◐ 1 · Docs ○ 1 · Search paused                      │
│─ Invoice PDF fonts · Billing export · t-0002 ──────────────────────────────────│
│ Status   ready for review · PR #418 · review required · 1 comment              │
│ Checks   ✕ pdf-render (failing)                                                │
│ Next     → Fix the font embedding the pdf-render check flags                   │
│ Linear   1 BIL-198                                                             │
│ GitHub   2 PR #418                                                             │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ ↵ go to pane  c coordinator  1-9 link  q quiet  esc close  ? help              │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns:

```text
┌────────────────────────────────────────────────────────────┐
│ For you                                      ◇ 2  ↻ 1  ✉ 1 │
│────────────────────────────────────────────────────────────│
│    PROJECT    WHAT                       AGE               │
│  Ready for review                                          │
│ ▸◇ Billing    Invoice PDF fonts          #418 ✕ · 20m      │
│  ◇ Admin      Document select for summ…  #2320 ✓ · 1h      │
│                                                            │
│  Landing                                                   │
│  ↻ Billing    Ledger totals fix          #415 ⋯ · 5m       │
│                                                            │
│  Inbox                                                     │
│  ✉ Docs       t-0001 is now Idle: repo…  2d                │
│                                                            │
│  Quiet                                                     │
│   Admin ◐ 2 ○ 1 · Billing ◐ 1 · Docs ○ 1 · Search paused   │
│─ Invoice PDF fonts · t-0002 ───────────────────────────────│
│ Status  review required · 1 comment                        │
│ Checks  ✕ pdf-render (failing)                             │
│ Next    → Fix the font embedding the pdf-render            │
│           check flags                                      │
│ Linear  1 BIL-198                                          │
│ GitHub  2 PR #418                                          │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│────────────────────────────────────────────────────────────│
│ ↵ go to pane  1-9 link  q quiet  esc close                 │
└────────────────────────────────────────────────────────────┘
```

- **Groups, in this order:** *Needs you* (threads waiting on you), *Inbox*
  (unhandled items), *Failing checks*, *Ready for review*, *Landing*. A
  thread appears once, in its most urgent group. A ready-for-review PR with a
  failing check stays under *Ready for review* but sorts first and shows `✕`.
- **Checks** come from ticker.json's `failing_checks[]`: the deck knows the
  names of the failing checks, not how many passed.
- **Quiet** is one dim line with, per project, the threads that are not in
  the feed (working and idle). Archived projects are left out.

120 columns, as a tab: the drawer moves right, and THREAD and LINKS columns
appear.

```text
┌────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│ For you · 3 projects                                                                   ◇ 2  ↻ 1  ✉ 1 · refreshed 14:41 │
│────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────│
│    PROJECT         WHAT                      THREAD  STATE       AGE   │─ Invoice PDF fonts · t-0002 ──────────────────│
│  Ready for review                                                      │ Project  Billing export · pane w4K:p2         │
│ ▸◇ Billing export  Invoice PDF fonts         t-0002  #418 ✕      20m   │ Status   ready for review · PR #418           │
│  ◇ Admin rebuild   Document select for su…   t-0004  #2320 ✓     1h    │          review required · 1 comment          │
│                                                                        │                                               │
│  Landing                                                               │ Checks   ✕ pdf-render (failing)               │
│  ↻ Billing export  Ledger totals fix         t-0004  #415 ⋯      5m    │ Next     → Fix the font embedding the         │
│                                                                        │                                               │
│  Inbox                                                                 │            pdf-render check flags             │
│  ✉ Docs site       t-0001 is now Idle: report written            2d    │ Linear   1 BIL-198                            │
│                                                                        │ GitHub   2 PR #418                            │
│  Quiet                                                                 │ Branch   hp/billing-export/t-0002-invoice-…   │
│   Admin rebuild  ◐ 2  ○ 1   last 4m                                    │ Dev      none running                         │
│   Billing export ◐ 1        last 1m                                    │                                               │
│   Docs site      ○ 1        last 2d                                    │                                               │
│   Search spike   paused · no threads                                   │                                               │
│                                                                        │                                               │
│                                                                        │                                               │
│                                                                        │                                               │
│                                                                        │                                               │
│                                                                        │                                               │
│                                                                        │                                               │
│                                                                        │                                               │
│                                                                        │                                               │
│────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────│
│ j k move  ↵ go to pane  c coordinator  1-9 link  l f n g o first  q quiet  r report  esc close  ? help                 │
└────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

## Something needs you in another project

There is no "other project" here: a new need goes to the top of the feed.
*Billing export*'s t-0003 waits on you. 80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ For you                                           ● 1 needs you  ◇ 2  ↻ 1  ✉ 2 │
│────────────────────────────────────────────────────────────────────────────────│
│    PROJECT          WHAT                             STATE           AGE       │
│  Needs you                                                                     │
│ ▸● Billing export   CSV export job                   needs you ~70%  2m        │
│                                                                                │
│  Inbox                                                                         │
│  ✉ Billing export   t-0003 is now Waiting on you                     2m        │
│  ✉ Docs site        t-0001 is now Idle: report written               2d        │
│                                                                                │
│  Ready for review                                                              │
│  ◇ Billing export   Invoice PDF fonts                #418 ✕ 1        20m       │
│  ◇ Admin rebuild    Document select for summary      #2320 ✓         1h        │
│                                                                                │
│  Landing                                                                       │
│  ↻ Billing export   Ledger totals fix                #415 ⋯          5m        │
│                                                                                │
│  Quiet                                                                         │
│   Admin ◐ 2  ○ 1 · Docs ○ 1 · Search paused                                    │
│─ CSV export job · Billing export · t-0003 ─────────────────────────────────────│
│ Status   needs you · ~70% · since 14:31 · pane w4K:p3                          │
│ Next     → Choose: one CSV per account, or one zip for all (BIL-210)           │
│          → Confirm a 50 000-row cap per file                                   │
│ Linear   1 BIL-210                                                             │
│                                                                                │
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ ↵ go to pane  c coordinator  1-9 link  r report  esc close  ? help             │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns:

```text
┌────────────────────────────────────────────────────────────┐
│ For you                                 ● 1  ◇ 2  ↻ 1  ✉ 2 │
│────────────────────────────────────────────────────────────│
│    PROJECT    WHAT                       AGE               │
│  Needs you                                                 │
│ ▸● Billing    CSV export job             ~70% · 2m         │
│                                                            │
│  Inbox                                                     │
│  ✉ Billing    t-0003 is now Waiting on…  2m                │
│  ✉ Docs       t-0001 is now Idle: repo…  2d                │
│                                                            │
│  Ready for review                                          │
│  ◇ Billing    Invoice PDF fonts          #418 ✕ · 20m      │
│  ◇ Admin      Document select for summ…  #2320 ✓ · 1h      │
│                                                            │
│  Landing                                                   │
│  ↻ Billing    Ledger totals fix          #415 ⋯ · 5m       │
│                                                            │
│  Quiet                                                     │
│   Admin ◐ 2 ○ 1 · Docs ○ 1 · Search paused                 │
│─ CSV export job · t-0003 ──────────────────────────────────│
│ Status  needs you · ~70% · since 14:31                     │
│ Next    → Choose: one CSV per account, or one zip          │
│           for all (BIL-210)                                │
│         → Confirm a 50 000-row cap per file                │
│ Linear  1 BIL-210                                          │
│                                                            │
│────────────────────────────────────────────────────────────│
│ ↵ go to pane  1-9 link  r report  esc close                │
└────────────────────────────────────────────────────────────┘
```

- **A thread and its inbox item** are both listed: the item is what
  herdr-projects wrote, the thread row is where you act. Collapsing the two
  into one row is possible (match the item's subject thread), but then the
  inbox count no longer matches herdr-projects' own.
- **Billing export left *Quiet*:** its only thread outside the feed
  (t-0003) now needs you, so nothing of it is quiet.
- **The popup can tell you without being open.** Nothing in this direction
  needs it, but a herdr notification on a new *Needs you* row (herdr has
  notifications; the toggle already uses one) would pair well with it.

## Nothing for you, an empty project, and archived projects

Nothing needs you anywhere, and `q` has unfolded *Quiet*. 80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ For you                                                   nothing   3 projects │
│────────────────────────────────────────────────────────────────────────────────│
│                                                                                │
│    Nothing needs you. 3 threads working, 2 idle.                               │
│                                                                                │
│  Quiet                                                                         │
│ ▸  Admin rebuild    ◐ 2  ○ 1 · coordinator idle                      4m        │
│    Billing export   ◐ 1 · coordinator working                        1m        │
│    Docs site        ○ 1 · coordinator idle                           2d        │
│    Search spike     paused · no tasks or threads · no coordinator              │
│                                                                                │
│  + Archived (1)                                                                │
│─ Admin rebuild ────────────────────────────────────────────────────────────────│
│ Goal     Rebuild the admin pages on the new design system, one page per        │
│          thread.                                                               │
│ Coord    w1Z:p1 · idle                                                         │
│ Repos    ~/src/webshop                                                         │
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
│ ↵ go to coordinator  q quiet  A archived  esc close  ? help                    │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns:

```text
┌────────────────────────────────────────────────────────────┐
│ For you                                            nothing │
│────────────────────────────────────────────────────────────│
│                                                            │
│    Nothing needs you. 3 working, 2 idle.                   │
│                                                            │
│  Quiet                                                     │
│ ▸  Admin rebuild   ◐ 2  ○ 1 · idle               4m        │
│    Billing export  ◐ 1 · working                 1m        │
│    Docs site       ○ 1 · idle                    2d        │
│    Search spike    paused · no threads                     │
│                                                            │
│  + Archived (1)                                            │
│─ Admin rebuild ────────────────────────────────────────────│
│ Goal    Rebuild the admin pages on the new design          │
│         system, one page per thread.                       │
│ Coord   w1Z:p1 · idle                                      │
│ Repos   ~/src/webshop                                      │
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
│ ↵ coordinator  q quiet  A archived  esc close              │
└────────────────────────────────────────────────────────────┘
```

- **Unfolded *Quiet*** has one row per project with its counts and its
  coordinator's agent state; `↵` focuses the coordinator. The drawer shows
  the project's goal, as D's empty project does.
- **Empty and paused** projects are only *Quiet* rows. Archived projects
  sit under a folded heading and never contribute to the feed.
- The empty feed says how much is going on, so "nothing" does not read as
  "broken".

## A data source is missing

The herdr socket is gone, *Billing export* has no `.state/ticker.json`, and
*Docs site*'s thread list fails. This hurts H most, because the feed *is*
the needs-you signal. 80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ For you                                                          ◇ 1  ✉ 1  ! 3 │
│────────────────────────────────────────────────────────────────────────────────│
│ ! Feed may be incomplete: live agent state off, 2 projects partly unread       │
│    PROJECT          WHAT                             STATE           AGE       │
│  Ready for review                                                              │
│ ▸◇ Admin rebuild    Document select for summary      #2320 ✓         1h        │
│  ◇ Billing export   Invoice PDF fonts                #418 ?          20m       │
│                                                                                │
│  Inbox                                                                         │
│  ✉ Docs site        t-0001 is now Idle: report written               2d        │
│                                                                                │
│  Quiet                                                                         │
│   Admin ◐ 2  ○ 1 · Billing ◐ 1 ↻ 1 · Search paused                             │
│─ Sources ──────────────────────────────────────────────────────────────────────│
│ ! herdr socket     not found: needs-you only from herdr-projects' groups       │
│                    (as of 14:23), no pane focus                                │
│ ! Billing export   no .state/ticker.json: no PR checks, so no                  │
│                    Failing checks or Landing rows                              │
│ ! Docs site        thread list failed: PROJECT.md line 4 does not parse;       │
│                    only its inbox is shown                                     │
│ ✓ Admin rebuild, Search spike                                       read 14:41 │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ 1-9 link  ! sources  esc close  ? help                                         │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns:

```text
┌────────────────────────────────────────────────────────────┐
│ For you                                      ◇ 1  ✉ 1  ! 3 │
│────────────────────────────────────────────────────────────│
│ ! Feed may be incomplete (! for details)                   │
│    PROJECT    WHAT                       AGE               │
│  Ready for review                                          │
│ ▸◇ Admin      Document select for summ…  #2320 ✓ · 1h      │
│  ◇ Billing    Invoice PDF fonts          #418 ? · 20m      │
│                                                            │
│  Inbox                                                     │
│  ✉ Docs       t-0001 is now Idle: repo…  2d                │
│                                                            │
│  Quiet                                                     │
│   Admin ◐ 2 ○ 1 · Billing ◐ 1 ↻ 1 · Search paused          │
│─ Sources ──────────────────────────────────────────────────│
│ ! herdr socket    missing: no live state or focus          │
│ ! Billing export  no ticker.json: no PR checks             │
│ ! Docs site       unreadable; inbox only                   │
│ ✓ Admin rebuild, Search spike                   read 14:41 │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│────────────────────────────────────────────────────────────│
│ 1-9 link  ! sources  esc close                             │
└────────────────────────────────────────────────────────────┘
```

- **A yellow band** under the header says the feed may be missing rows,
  because an empty or short feed is otherwise read as "all clear".
- Without the ticker, *Billing export*'s landing thread drops to *Quiet*
  (`↻ 1`): the feed cannot tell its checks, and it does not guess.

## Good and bad

| | |
|---|---|
| Scanning | Very good for "what wants me": one sorted list, a few rows. Poor for "what is going on in project X": that is one *Quiet* line. |
| Needs you | Best of the four: it is the whole view, sorted, with `next[]` in the drawer. |
| Next to a coordinator | Not meant for it; the per-coordinator deck stays as it is. |
| Many projects | Very good: the feed grows with needs, not with projects; *Quiet* is one line until unfolded. |
| Narrow width | Good: four columns, project names shorten to one word. |
| Keys | Fewest: `↵` goes to the pane, `q` quiet. |
| Trust | Depends on the sources most: a missing ticker or socket silently shortens the feed, hence the band. |
| Data cost | Medium: every project's thread list and ticker.json (to find the needs), but no TASKS.md parsing except for the selected row's links. |
| Build cost | Low to medium: a new list over the existing readers; the drawer is D's. |
