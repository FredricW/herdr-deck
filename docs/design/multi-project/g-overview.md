# G · Overview rows that drill into today's deck

A new home page with one row per project: what needs you, what is working,
PRs, inbox, last activity. `↵` on a row opens that project's deck, which is
today's deck unchanged; `esc` comes back. Anything that needs you, in any
project, is pinned above the project rows, so the home page answers "where
do I go next" without opening anything.

Shared sample data and conventions are in the [README](README.md#shared-conventions).

**Where it opens.** On its own, from a global key: a new action
`herdr-deck.overview` opens it as a herdr tab of its own (`placement =
"tab"`), or, if preferred, as a popup over everything (`placement =
"popup"`, `esc` closes it). `herdr-deck --all` runs it in any terminal. The
per-coordinator deck can also reach it: `backspace` in a project deck goes
up to the overview, and `esc` from there goes back down.

**How it relates to the per-coordinator deck.** It **complements** it. Next
to a coordinator nothing changes: the deck opens on its project, as today.
The overview is a level above, and its drill-down *is* the per-coordinator
deck, so there is one deck design to keep up.

**Across projects.** Needs-you and inbox items from every project are pinned
on top of the overview (project name, then the thread). `↵` on one of them
focuses the thread's pane directly. Links live one level down, in the
project deck's drawer; on the overview, `g` opens the selected project's
newest PR and the drawer numbers the project's PRs.

**Keys added.** `↵` open the project's deck (or, on a pinned row, the
thread's pane), `esc`/`backspace` up, `c` focus the project's coordinator,
`A` show archived.

## Normal use

80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Projects                                                   4 active · 1 paused │
│────────────────────────────────────────────────────────────────────────────────│
│    PROJECT            NEEDS  THREADS          PRS              INBOX  LAST     │
│ ▸  Admin rebuild             ◐ 2  ◇ 1  ○ 1    #2320 ✓                 4m       │
│    Billing export            ◐ 1  ◇ 1  ↻ 1    #418 ✕  #415 ⋯          1m       │
│    Docs site                 ○ 1                               ✉ 1    2d       │
│    Search spike       paused · no threads                                      │
│                                                                                │
│  + Archived (1)                                                                │
│                                                                                │
│─ Admin rebuild ────────────────────────────────────────────────────────────────│
│ Goal     Rebuild the admin pages on the new design system, one page per        │
│          thread.                                                               │
│ Lists    In progress 3 · On hold 1 · Backlog 6                                 │
│ Coord    w1Z:p1 · idle                                                         │
│ PRs      1 #2320 Document select for summary · review required · ✓             │
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
│ ↵ open deck  c coordinator  1-9 PR  A archived  ? help                         │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns:

```text
┌────────────────────────────────────────────────────────────┐
│ Projects                               4 active · 1 paused │
│────────────────────────────────────────────────────────────│
│    PROJECT         NEEDS  THREADS        INBOX  LAST       │
│ ▸  Admin rebuild          ◐ 2  ◇ 1  ○ 1         4m         │
│    Billing export         ◐ 1  ◇ 1  ↻ 1         1m         │
│    Docs site              ○ 1            ✉ 1    2d         │
│    Search spike    paused · no threads                     │
│                                                            │
│  + Archived (1)                                            │
│                                                            │
│─ Admin rebuild ────────────────────────────────────────────│
│ Goal    Rebuild the admin pages on the new design          │
│         system, one page per thread.                       │
│ Lists   In progress 3 · On hold 1 · Backlog 6              │
│ Coord   w1Z:p1 · idle                                      │
│ PRs     1 #2320 review required ✓                          │
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
│ ↵ open deck  c coordinator  1-9 PR  ? help                 │
└────────────────────────────────────────────────────────────┘
```

- **Columns.** NEEDS is red `● N` (threads waiting on you); THREADS is the
  project's other group counts in their colours; PRS lists open PRs with
  their checks (`✓` `✕` `⋯`, magenta numbers); INBOX is yellow; LAST is the
  newest thread `last_state_change`. At 60 columns PRS moves to the drawer.
- **The drawer** shows the selected project: goal, list sizes from TASKS.md,
  its coordinator (pane and agent state from herdr), numbered PRs, repos.
- **Rows keep `herdr-projects list` order** (by slug); a project with a need
  is pinned above (next section) rather than re-sorted.

120 columns, as a herdr tab:

```text
┌────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│ Projects                                                                         4 active · 1 paused · refreshed 14:41 │
│────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────│
│    PROJECT             NEEDS  WORKING  REVIEW     IDLE  INBOX  PRS               DEV      COORDINATOR       LAST       │
│ ▸  Admin rebuild              ◐ 2      ◇ 1        ○ 1          #2320 ✓           ●●● ●○   w1Z:p1 idle       4m         │
│    Billing export             ◐ 1      ◇ 1  ↻ 1                #418 ✕  #415 ⋯    ●●       w4K:p1 working    1m         │
│    Docs site                                      ○ 1   ✉ 1                               w7Q:p1 idle       2d         │
│    Search spike        paused · no threads                                                none                         │
│                                                                                                                        │
│  + Archived (1)                                                                                                        │
│                                                                                                                        │
│─ Admin rebuild · acme/webshop ─────────────────────────────────────────────────────────────────────────────────────────│
│ Goal     Rebuild the admin pages on the new design system, one page per thread.                                        │
│ Lists    In progress 3 · On hold until Monday 2026-10-05 1 · Backlog 6                                                 │
│ Threads  ◐ Users page t-0002 · Building overview ~60%      ◐ Templates page t-0003 · Writing tests                     │
│          ◇ Document select for summary t-0004 · #2320       ○ Subscriptions list t-0001 · idle                         │
│ PRs      1 #2320 Document select for summary · review required · 2 comments · ✓                                        │
│ Dev      2 :5181 frontend ●  3 :8011 api ●  4 :5441 pg ●  (t-0003)    5 :5174 frontend ●  :8002 api ○  (t-0004)        │
│ Repos    ~/src/webshop                                                                                                 │
│                                                                                                                        │
│                                                                                                                        │
│                                                                                                                        │
│                                                                                                                        │
│                                                                                                                        │
│                                                                                                                        │
│                                                                                                                        │
│                                                                                                                        │
│────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────│
│ j k move  ↵ open deck  c coordinator  1-9 link  g PR  o localhost  A archived  ? help                                  │
└────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

- **The wide page** splits the counts into their own columns and adds the
  dev servers (one dot per port, grouped per thread) and the coordinator's
  pane and agent state. The drawer can afford a line per thread.
- **The overview's drawer numbers** PRs and localhost links for the whole
  project; Linear, Figma and Notion links stay in the project deck, where
  there is a row to hang them on.

## Something needs you in another project

*Billing export*'s t-0003 waits on you. 80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Projects                                                    ● 1 needs you  ✉ 2 │
│────────────────────────────────────────────────────────────────────────────────│
│  Needs you                                                                     │
│ ▸● Billing export      CSV export job · t-0003                       2m        │
│  ✉ Billing export      inbox: t-0003 is now Waiting on you           2m        │
│  ✉ Docs site           inbox: t-0001 is now Idle: report written     2d        │
│                                                                                │
│    PROJECT            NEEDS  THREADS          PRS              INBOX  LAST     │
│    Admin rebuild             ◐ 2  ◇ 1  ○ 1    #2320 ✓                 4m       │
│    Billing export     ● 1    ◇ 1  ↻ 1         #418 ✕  #415 ⋯   ✉ 1    2m       │
│    Docs site                 ○ 1                               ✉ 1    2d       │
│    Search spike       paused · no threads                                      │
│                                                                                │
│  + Archived (1)                                                                │
│─ CSV export job · Billing export ──────────────────────────────────────────────│
│ Thread   t-0003 CSV export /exports · pane w4K:p3                              │
│ Status   needs you · ~70% · since 14:31                                        │
│ Next     → Choose: one CSV per account, or one zip for all (BIL-210)           │
│          → Confirm a 50 000-row cap per file                                   │
│ Linear   1 BIL-210                                                             │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ ↵ go to pane  o open Billing export  1-9 link  r report  ? help                │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns:

```text
┌────────────────────────────────────────────────────────────┐
│ Projects                                ● 1 needs you  ✉ 2 │
│────────────────────────────────────────────────────────────│
│  Needs you                                                 │
│ ▸● Billing        CSV export job · t-0003        2m        │
│  ✉ Billing        t-0003 is now Waiting on you   2m        │
│  ✉ Docs           t-0001 is now Idle: report…    2d        │
│                                                            │
│    PROJECT         NEEDS  THREADS        INBOX  LAST       │
│    Admin rebuild          ◐ 2  ◇ 1  ○ 1         4m         │
│    Billing export  ● 1    ◇ 1  ↻ 1       ✉ 1    2m         │
│    Docs site              ○ 1            ✉ 1    2d         │
│    Search spike    paused · no threads                     │
│                                                            │
│  + Archived (1)                                            │
│─ CSV export job · Billing export ──────────────────────────│
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
│────────────────────────────────────────────────────────────│
│ ↵ go to pane  o open project  1-9 link  ? help             │
└────────────────────────────────────────────────────────────┘
```

`o` (or `↵` on the *Billing export* row) drills into the project's deck,
which is today's deck with a breadcrumb. 80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ ‹ Projects / Billing export                       ● 1 needs you  ◇ 1  ↻ 1  ✉ 1 │
│────────────────────────────────────────────────────────────────────────────────│
│    WORK                          THREAD  STATUS              PR      LINKS  DEV│
│  Needs you                                                                     │
│ ▸● CSV export job                t-0003  needs you · 2m              L      ●● │
│  ✉ inbox: t-0003 is now Waiting on you                                   2m    │
│                                                                                │
│  In progress                                                                   │
│  ◇ Invoice PDF fonts             t-0002  checks ✕ 1          #418    L         │
│  ↻ Ledger totals fix             t-0004  landing             #415    L2        │
│                                                                                │
│  + Backlog (3)                                                                 │
│─ CSV export job ───────────────────────────────────────────────────────────────│
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
│ esc projects  ↵ go to pane  1-9 link  r report  z drawer  ? help               │
└────────────────────────────────────────────────────────────────────────────────┘
```

120 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│ Projects                                                                          ● 1 needs you  ✉ 2 · refreshed 14:41 │
│────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────│
│  Needs you                                                                                                             │
│ ▸● Billing export      CSV export job · t-0003 · needs you · ~70%                              since 14:31 · 2m        │
│  ✉ Billing export      inbox: t-0003 is now Waiting on you                                     14:31 · 2m              │
│  ✉ Docs site           inbox: t-0001 is now Idle: report written                               2d                      │
│                                                                                                                        │
│    PROJECT             NEEDS  WORKING  REVIEW     IDLE  INBOX  PRS               DEV      COORDINATOR       LAST       │
│    Admin rebuild              ◐ 2      ◇ 1        ○ 1          #2320 ✓           ●●● ●○   w1Z:p1 idle       4m         │
│    Billing export      ● 1             ◇ 1  ↻ 1         ✉ 1    #418 ✕  #415 ⋯    ●●       w4K:p1 idle       2m         │
│    Docs site                                      ○ 1   ✉ 1                               w7Q:p1 idle       2d         │
│    Search spike        paused · no threads                                                none                         │
│                                                                                                                        │
│  + Archived (1)                                                                                                        │
│─ CSV export job · t-0003 · Billing export ─────────────────────────────────────────────────────────────────────────────│
│ Thread   CSV export /exports · pane w4K:p3 · branch hp/billing-export/t-0003-csv-export-job                            │
│ Status   needs you · ~70% · since 14:31                                                                                │
│ Next     → Choose: one CSV per account, or one zip for all (BIL-210)                                                   │
│          → Confirm a 50 000-row cap per file                                                                           │
│ Linear   1 BIL-210                                                                                                     │
│ Dev      2 :5190 frontend ●  3 :8020 api ●                                                                             │
│                                                                                                                        │
│                                                                                                                        │
│                                                                                                                        │
│                                                                                                                        │
│                                                                                                                        │
│────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────│
│ j k move  ↵ go to pane  o open Billing export  1-9 link  r report  c coordinator  ? help                               │
└────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

- **Pinned rows** are threads (red) and inbox items (yellow) from every
  project, newest first. The project row also turns its NEEDS red, so the
  table alone tells you which project.
- **The cursor jumps** to the first pinned row unless moved by hand, as in
  D. Here that is safe: the overview is its own place, not a deck beside some
  other coordinator.
- **The pinned row's drawer** is D's drawer for that thread, so its
  `next[]` and links are one glance away without drilling in.

## An empty project, and archived projects

`A` unfolds *Archived*; the cursor is on *Search spike*. 80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Projects                                      4 active · 1 paused · 1 archived │
│────────────────────────────────────────────────────────────────────────────────│
│    PROJECT            NEEDS  THREADS          PRS              INBOX  LAST     │
│    Admin rebuild             ◐ 2  ◇ 1  ○ 1    #2320 ✓                 4m       │
│    Billing export            ◐ 1  ◇ 1  ↻ 1    #418 ✕  #415 ⋯          1m       │
│    Docs site                 ○ 1                               ✉ 1    2d       │
│ ▸  Search spike       paused · no threads                                      │
│                                                                                │
│  Archived                                                                      │
│    Mobile onboarding  archived · resolved 6                           3w       │
│                                                                                │
│─ Search spike ─────────────────────────────────────────────────────────────────│
│ Status   paused: the ticker skips it, and threads cannot start                 │
│ Goal     Try two search engines on the docs corpus and write down which        │
│          one to keep.                                                          │
│ Lists    TASKS.md has no tasks yet                                             │
│ Coord    no coordinator running                                                │
│ Repos    ~/src/docs                                                            │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ ↵ open deck  A hide archived  ? help                                           │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns:

```text
┌────────────────────────────────────────────────────────────┐
│ Projects                  4 active · 1 paused · 1 archived │
│────────────────────────────────────────────────────────────│
│    PROJECT         NEEDS  THREADS        INBOX  LAST       │
│    Admin rebuild          ◐ 2  ◇ 1  ○ 1         4m         │
│    Billing export         ◐ 1  ◇ 1  ↻ 1         1m         │
│    Docs site              ○ 1            ✉ 1    2d         │
│ ▸  Search spike    paused · no threads                     │
│                                                            │
│  Archived                                                  │
│    Mobile onboar…  archived · resolved 6        3w         │
│                                                            │
│─ Search spike ─────────────────────────────────────────────│
│ Status  paused: the ticker skips it                        │
│ Goal    Try two search engines on the docs corpus          │
│         and write down which one to keep.                  │
│ Lists   no tasks yet                                       │
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
│────────────────────────────────────────────────────────────│
│ ↵ open deck  A hide archived  ? help                       │
└────────────────────────────────────────────────────────────┘
```

- **Empty and paused** projects are one dim row each. `↵` still opens the
  deck (today's empty state), which is where its goal and repos are.
- **Archived** projects are hidden behind a folded heading, as
  `herdr-projects list` hides them without `--all`. They never count in the
  header. LAST for an archived project is its newest thread change.

## A data source is missing

The herdr socket is gone, *Billing export* has no `.state/ticker.json`, and
*Docs site*'s thread list fails. 80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Projects                                              4 active · 1 paused  ! 3 │
│────────────────────────────────────────────────────────────────────────────────│
│    PROJECT            NEEDS  THREADS          PRS              INBOX  LAST     │
│ ▸  Admin rebuild             ◐ 2  ◇ 1  ○ 1    #2320 ✓                 4m       │
│    Billing export            ◐ 1  ◇ 1  ↻ 1    #418 ?  #415 ?          1m       │
│    ! Docs site        threads unreadable                       ✉ 1             │
│    Search spike       paused · no threads                                      │
│                                                                                │
│  + Archived (1)                                                                │
│                                                                                │
│─ Sources ──────────────────────────────────────────────────────────────────────│
│ ! herdr socket     not found: no live agent state, no coordinator or           │
│                    pane focus; needs-you as of the last ticker run 14:23       │
│ ! Billing export   no .state/ticker.json: no PR review or checks               │
│ ! Docs site        thread list failed: PROJECT.md line 4 does not parse        │
│ ✓ herdr-projects list, inboxes, TASKS.md                            read 14:41 │
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
│ ↵ open deck  ! sources  ? help                                                 │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns:

```text
┌────────────────────────────────────────────────────────────┐
│ Projects                          4 active · 1 paused  ! 3 │
│────────────────────────────────────────────────────────────│
│    PROJECT         NEEDS  THREADS        INBOX  LAST       │
│ ▸  Admin rebuild          ◐ 2  ◇ 1  ○ 1         4m         │
│    Billing export         ◐ 1  ◇ 1  ↻ 1         1m         │
│    ! Docs site     threads unreadable    ✉ 1               │
│    Search spike    paused · no threads                     │
│                                                            │
│  + Archived (1)                                            │
│                                                            │
│─ Sources ──────────────────────────────────────────────────│
│ ! herdr socket    missing: no live state or focus          │
│ ! Billing export  no ticker.json: no PR checks             │
│ ! Docs site       PROJECT.md line 4 does not parse         │
│ ✓ project list, inboxes, TASKS.md               read 14:41 │
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
│                                                            │
│────────────────────────────────────────────────────────────│
│ ↵ open deck  ! sources  ? help                             │
└────────────────────────────────────────────────────────────┘
```

- **The overview survives a broken project.** Its counts come from
  `herdr-projects list` (one call for all projects) where it can, so a
  project whose own `thread list` fails still has a row, with `!` and the
  inbox count read from its folder.
- `! 3` and the *Sources* drawer work as in D, with one line per affected
  project plus the global ones.

## Good and bad

| | |
|---|---|
| Scanning | Very good across projects: one row each, columns line up, the pinned group on top. |
| Needs you | Very good: pinned across projects with the thread's `next[]` in the drawer, and `↵` goes straight to the pane. |
| Next to a coordinator | Unchanged: the per-coordinator deck is today's, plus `backspace` to go up. |
| Many projects | Very good: ten projects are ten rows, and the pinned group stays short. |
| Narrow width | Good at 80; at 60 PRs move to the drawer and names truncate around 15 characters. Best as a full tab. |
| Keys | Few: `↵` down, `esc` up, `c` coordinator. Two levels to learn. |
| Data cost | Low on the overview (`herdr-projects list`, inbox folders, one herdr snapshot, PRs from each ticker.json); full data only for the drilled-into project and pinned threads. |
| Build cost | Medium: a new page and a navigation stack, but the project deck is reused as it is. |
