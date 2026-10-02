# D · Compact list + detail drawer

A dense list on top, one row per piece of work (a task joined with its
thread; a thread no task names gets its own row), and a fixed drawer at the
bottom that shows everything about the selected row: status, PR, `next[]`,
notes, every link numbered, servers. The list answers "what is going on"; the
drawer answers "what can I do about this one". There is no chooser: the
drawer already lists every link with a digit.

Shared colours, glyphs and sample data are in the [README](README.md#shared-conventions).
Each frame below is the pane itself: the box adds one column on each side.

**Keys.** `j`/`k` move in the list, the drawer follows. Link letters open the
first link of that kind; digits open the drawer's numbered links. `z`
cycles the drawer through its normal height (about 40 %), full height (for
long reports) and hidden.

## Normal use

80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Admin rebuild                                               ◐ 2  ◇ 1  ○ 1  ✉ 0 │
│────────────────────────────────────────────────────────────────────────────────│
│    WORK                          THREAD  STATUS              PR      LINKS  DEV│
│  In progress                                                                   │
│ ▸◐ Users page                    t-0002  Building overview          L4         │
│  ◐ Templates page                t-0003  Writing tests              L F    ●●● │
│  ◇ Document select for summary   t-0004  review required     #2320  L F3   ●○  │
│                                                                                │
│  On hold until Monday 2026-10-05                                               │
│  ○ Subscriptions list            t-0001  idle                       L N        │
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
│                                                                                │
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ 1-9 link  l f n g o first  ↵ pane  e edit  z drawer  ? help                    │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns:

```text
┌────────────────────────────────────────────────────────────┐
│ Admin rebuild                           ◐ 2  ◇ 1  ○ 1  ✉ 0 │
│────────────────────────────────────────────────────────────│
│    WORK                       STATUS           LINKS  DEV  │
│  In progress                                               │
│ ▸◐ Users page                 Building overv…  L4          │
│  ◐ Templates page             Writing tests    L F    ●●●  │
│  ◇ Document select for sum…   #2320 review     L F3   ●○   │
│                                                            │
│  On hold until Monday 2026-10-05                           │
│  ○ Subscriptions list         idle             L N         │
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
│                                                            │
│                                                            │
│────────────────────────────────────────────────────────────│
│ 1-9 link  l f n g o first  ↵ pane  z drawer  ? help        │
└────────────────────────────────────────────────────────────┘
```

- **List.** Column titles dim small caps; list headings dim and indented one
  column less than rows. Status column in the state colour; `#2320` magenta;
  `LINKS` letters bold in their kind's colour, a digit after a letter when
  there are several. `DEV` is one dot per port in dev.json order: green `●`
  listening, dim `○` not.
- **Selection.** A subtle grey background across the row (256-colour 237,
  254 on a light terminal); the row's text keeps its colours. The drawer's title rule names
  the selected row; drawer labels (`Thread`, `Status`, …) are dim, values
  normal, link digits bold.
- **At 60 columns** the `THREAD` and `PR` columns fold into `STATUS` (a PR
  shows as `#2320 review`), and the thread id moves to the drawer title.

## A thread needs you

80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Admin rebuild                                ● 1 needs you  ◐ 1  ◇ 1  ○ 1  ✉ 1 │
│────────────────────────────────────────────────────────────────────────────────│
│    WORK                          THREAD  STATUS              PR      LINKS  DEV│
│  Needs you                                                                     │
│ ▸● Users page                    t-0002  needs you · 3m             L4         │
│  ✉ inbox: t-0002 is now Waiting on you                                   3m    │
│                                                                                │
│  In progress                                                                   │
│  ◐ Templates page                t-0003  Writing tests              L F    ●●● │
│  ◇ Document select for summary   t-0004  review required     #2320  L F3   ●○  │
│                                                                                │
│  On hold until Monday 2026-10-05                                               │
│  ○ Subscriptions list            t-0001  idle                       L N        │
│─ Users page ───────────────────────────────────────────────────────────────────│
│ Thread   t-0002 Members /admin/users · pane w1Z:p1                             │
│ Status   needs you · ~95% · since 14:23                                        │
│ Next     → Approve phase 1 (ABC-1256 overview)                                 │
│          → Say whether to start ABC-1257 and ABC-1250                          │
│ Linear   1 ABC-1246   2 ABC-1256   3 ABC-1257   4 ABC-1250                     │
│ Report   threads/t-0002.md · changed 14:22                                     │
│ Branch   hp/admin-rebuild/t-0002-members-admin-users                           │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ ↵ go to pane  1-9 link  e edit  r report  z drawer  ? help                     │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns:

```text
┌────────────────────────────────────────────────────────────┐
│ Admin rebuild            ● 1 needs you  ◐ 1  ◇ 1  ○ 1  ✉ 1 │
│────────────────────────────────────────────────────────────│
│    WORK                       STATUS           LINKS  DEV  │
│  Needs you                                                 │
│ ▸● Users page                 needs you · 3m   L4          │
│  ✉ inbox: t-0002 is now Waiting on you                  3m │
│                                                            │
│  In progress                                               │
│  ◐ Templates page             Writing tests    L F    ●●●  │
│  ◇ Document select for sum…   #2320 review     L F3   ●○   │
│                                                            │
│  On hold until Monday 2026-10-05                           │
│  ○ Subscriptions list         idle             L N         │
│─ Users page · t-0002 ──────────────────────────────────────│
│ Status  needs you · ~95% · since 14:23                     │
│ Next    → Approve phase 1 (ABC-1256 overview)              │
│         → Say whether to start ABC-1257 and ABC-1250       │
│ Linear  1 ABC-1246  2 ABC-1256  3 ABC-1257  4 ABC-1250     │
│ Report  threads/t-0002.md · changed 14:22                  │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│────────────────────────────────────────────────────────────│
│ ↵ go to pane  1-9 link  r report  z drawer  ? help         │
└────────────────────────────────────────────────────────────┘
```

- **Pinned group.** Rows that need you, and unhandled inbox items, move into
  a red *Needs you* group at the top of the list (the row keeps its list name
  in the drawer). When the cursor has not been moved by hand, it jumps to the
  first such row so the drawer shows its `next[]`. Row text bold red, inbox
  rows yellow.
- **`r report`** shows the thread's whole report in the drawer at full height
  (same as `z`); `esc` returns.

## Unhandled inbox items

80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Admin rebuild                                ● 1 needs you  ◐ 1  ◇ 1  ○ 1  ✉ 2 │
│────────────────────────────────────────────────────────────────────────────────│
│    WORK                          THREAD  STATUS              PR      LINKS  DEV│
│  Needs you                                                                     │
│  ● Users page                    t-0002  needs you · 9m             L4         │
│  ✉ inbox: t-0002 is now Waiting on you                                   9m    │
│ ▸✉ inbox: t-0004 is now Ready for review                                 2m    │
│                                                                                │
│  In progress                                                                   │
│  ◐ Templates page                t-0003  Writing tests              L F    ●●● │
│  ◇ Document select for summary   t-0004  review required     #2320  L F3   ●○  │
│                                                                                │
│  On hold until Monday 2026-10-05                                               │
│  ○ Subscriptions list            t-0001  idle                       L N        │
│─ Inbox item ───────────────────────────────────────────────────────────────────│
│ Kind     thread-state · created 14:30                                          │
│ Subject  t-0004 Summary select documents                                       │
│ Summary  t-0004 "Summary select documents" is now Ready for review:            │
│          PR #2320 opened.                                                      │
│ GitHub   1 PR #2320 · review required · 2 comments · checks ✓                  │
│ File     inbox/20261002T143012Z-thread-state-t-0004-7.md                       │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ ↵ go to t-0004  1 PR  z drawer  ? help                                         │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns:

```text
┌────────────────────────────────────────────────────────────┐
│ Admin rebuild                      ● 1  ◐ 1  ◇ 1  ○ 1  ✉ 2 │
│────────────────────────────────────────────────────────────│
│    WORK                       STATUS           LINKS  DEV  │
│  Needs you                                                 │
│  ● Users page                 needs you · 9m   L4          │
│  ✉ inbox: t-0002 is now Waiting on you                  9m │
│ ▸✉ inbox: t-0004 is now Ready for review                2m │
│                                                            │
│  In progress                                               │
│  ◐ Templates page             Writing tests    L F    ●●●  │
│  ◇ Document select for sum…   #2320 review     L F3   ●○   │
│                                                            │
│  On hold until Monday 2026-10-05                           │
│  ○ Subscriptions list         idle             L N         │
│─ Inbox item · 14:30 ───────────────────────────────────────│
│ Kind     thread-state                                      │
│ Summary  t-0004 "Summary select documents" is now          │
│          Ready for review: PR #2320 opened.                │
│ GitHub   1 PR #2320 · review required · 2c · ✓             │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│────────────────────────────────────────────────────────────│
│ ↵ go to t-0004  1 PR  z drawer  ? help                     │
└────────────────────────────────────────────────────────────┘
```

- Inbox items are rows like any other, so `j`/`k` reach them without a
  section switch. The drawer shows the item's whole front matter and pulls in
  the subject thread's links, so `1` opens the PR the item is about.

## Several links of one kind: the drawer is the chooser

`l` on a row with several Linear links does not open a popup: it highlights
the drawer's Linear line (yellow `▶`, label on the selection's grey) and the footer asks for a
digit. A second `l` opens the first one.

80 columns, `l` on *Users page*:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Admin rebuild                                               ◐ 2  ◇ 1  ○ 1  ✉ 0 │
│────────────────────────────────────────────────────────────────────────────────│
│    WORK                          THREAD  STATUS              PR      LINKS  DEV│
│  In progress                                                                   │
│ ▸◐ Users page                    t-0002  Building overview          L4         │
│  ◐ Templates page                t-0003  Writing tests              L F    ●●● │
│  ◇ Document select for summary   t-0004  review required     #2320  L F3   ●○  │
│                                                                                │
│  On hold until Monday 2026-10-05                                               │
│  ○ Subscriptions list            t-0001  idle                       L N        │
│                                                                                │
│─ Users page ───────────────────────────────────────────────────────────────────│
│ Thread   t-0002 Members /admin/users · pane w1Z:p1                             │
│ Status   working · Building overview · ~60% · 4m                               │
│ Note     Phase 1 = layout + overview (ABC-1256); report on 1257/1250           │
│          before starting them.                                                 │
│▶Linear   1 ABC-1246   2 ABC-1256   3 ABC-1257   4 ABC-1250   a all             │
│ Branch   hp/admin-rebuild/t-0002-members-admin-users                           │
│ Dev      no servers running (dev.json: frontend, api, pg)                      │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ Linear: 1-4 open  a all  l first  esc cancel                                   │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns, `f` on *Document select for summary*:

```text
┌────────────────────────────────────────────────────────────┐
│ Admin rebuild                           ◐ 2  ◇ 1  ○ 1  ✉ 0 │
│────────────────────────────────────────────────────────────│
│    WORK                       STATUS           LINKS  DEV  │
│  In progress                                               │
│  ◐ Users page                 Building overv…  L4          │
│  ◐ Templates page             Writing tests    L F    ●●●  │
│ ▸◇ Document select for sum…   #2320 review     L F3   ●○   │
│                                                            │
│  On hold until Monday 2026-10-05                           │
│  ○ Subscriptions list         idle             L N         │
│                                                            │
│─ Document select for summary · t-0004 ─────────────────────│
│ Status  review required · 2 comments · checks ✓            │
│ Linear  1 ABC-1191                                         │
│▶Figma   2 598:48083   3 1138:88367   4 635:76529           │
│         d desktop app  a all                               │
│ GitHub  5 PR #2320                                         │
│ Dev     6 :5174 frontend ●   :8002 api ○                   │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│────────────────────────────────────────────────────────────│
│ Figma: 2-4 open  d desktop  a all  esc cancel              │
└────────────────────────────────────────────────────────────┘
```

- Digits number every link in the drawer in order (Linear, Figma, Notion,
  GitHub, localhost), so `5` here is the PR whether or not `f` was pressed.
- Each numbered link is a click target. Where a link was found is not shown,
  as in B.

## Empty project

80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Herdr Deck                                                     nothing running │
│────────────────────────────────────────────────────────────────────────────────│
│    WORK                          THREAD  STATUS              PR      LINKS  DEV│
│                                                                                │
│  Nothing yet. Rows appear when the coordinator writes TASKS.md or starts a     │
│  thread.                                                                       │
│                                                                                │
│─ Project ──────────────────────────────────────────────────────────────────────│
│ Goal     Build herdr-deck v1 (milestones 1-7 in docs/PLAN.md): a Go + Charm    │
│          status pane that runs next to each herdr-projects coordinator,        │
│          showing tasks, threads, links and dev servers …                       │
│ Repos    /home/dev/src/herdr-deck                                              │
│ Folder   ~/.herdr-projects/herdr-deck                                          │
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
│ ? help                                                                         │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns:

```text
┌────────────────────────────────────────────────────────────┐
│ Herdr Deck                                 nothing running │
│────────────────────────────────────────────────────────────│
│    WORK                       STATUS           LINKS  DEV  │
│                                                            │
│  Nothing yet. Rows appear when the coordinator writes      │
│  TASKS.md or starts a thread.                              │
│                                                            │
│─ Project ──────────────────────────────────────────────────│
│ Goal    Build herdr-deck v1 (milestones 1-7 in             │
│         docs/PLAN.md): a Go + Charm status pane that       │
│         runs next to each herdr-projects coordinator …     │
│ Repos   ~/src/herdr-deck                                   │
│ Folder  ~/.herdr-projects/herdr-deck                       │
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
│                                                            │
│                                                            │
│────────────────────────────────────────────────────────────│
│ ? help                                                     │
└────────────────────────────────────────────────────────────┘
```

- No reload key: `r` shows the report, and file changes refresh the deck
  anyway.
- With nothing selected the drawer shows the project: goal and repos from
  `PROJECT.md`. That is also what it shows when the cursor is on the header.

## A data source is missing

80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Admin rebuild                                          ◐ 2  ◇ 1  ○ 1  ✉ 0  ! 3 │
│────────────────────────────────────────────────────────────────────────────────│
│    WORK                          THREAD  STATUS              PR      LINKS  DEV│
│  In progress                                                       as of 14:23 │
│ ▸◐ Users page                    t-0002  Building overview          L4         │
│  ◐ Templates page                t-0003  Writing tests              L F    ●~  │
│  ◇ Document select for summary   t-0004  ?                   #2320  L F3   ○~  │
│                                                                                │
│  On hold until Monday 2026-10-05                                               │
│  ○ Subscriptions list            t-0001  idle                       L N        │
│                                                                                │
│─ Sources ──────────────────────────────────────────────────────────────────────│
│ ! ticker.json    last check 12:38 (2h): no PR review, comments or checks       │
│ ! herdr socket   not found: ↵ cannot focus panes, status from files only       │
│ ! dev.json       not in webshop: ports from the workspace port token (~)       │
│ ✓ PROJECT.md, TASKS.md, inbox/, thread list --json                  read 14:41 │
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
│ 1-9 link  l f n g o first  e edit  z drawer  ? help                            │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns:

```text
┌────────────────────────────────────────────────────────────┐
│ Admin rebuild                      ◐ 2  ◇ 1  ○ 1  ✉ 0  ! 3 │
│────────────────────────────────────────────────────────────│
│    WORK                       STATUS           LINKS  DEV  │
│  In progress                                   as of 14:23 │
│ ▸◐ Users page                 Building overv…  L4          │
│  ◐ Templates page             Writing tests    L F    ●~   │
│  ◇ Document select for sum…   #2320 ?          L F3   ○~   │
│                                                            │
│  On hold until Monday 2026-10-05                           │
│  ○ Subscriptions list         idle             L N         │
│                                                            │
│─ Sources ──────────────────────────────────────────────────│
│ ! ticker.json   2h old: no PR review or checks             │
│ ! herdr socket  missing: no pane focus                     │
│ ! dev.json      missing: port token used (~)               │
│ ✓ project files, thread list                    read 14:41 │
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
│ 1-9 link  l f n g o first  e edit  z drawer  ? help        │
└────────────────────────────────────────────────────────────┘
```

- **`! 3`** in the header (yellow) counts missing sources; clicking it or
  pressing `!` puts the *Sources* page in the drawer, as here. It comes up by
  itself once at start-up when something is missing.
- **Fallback ports** show as a dot followed by `~` (port token, not
  dev.json). PR status without the ticker is `?`.

## Good and bad

| | |
|---|---|
| Scanning | Very good. One line per piece of work; the pinned *Needs you* group is at the top of the list. |
| Density | Highest: 4 tasks in 6 rows, and the drawer gives depth only for the row you are on. 15+ rows of work fit beside the drawer on a normal screen. |
| Narrow width | Fair. Columns need about 58 characters; at 60 the THREAD and PR columns must fold into STATUS and titles truncate at ~25. Below 60 the table would need another fold. |
| Keys | Few and uniform: `j`/`k` plus a letter or a digit; no chooser state to learn, no sections. |
| Mouse | Good: rows select, every drawer link is a click target. Column headers could sort, though that is not needed for v1. |
| Weakness | Notes and `next[]` are visible for one row at a time. The drawer always takes ~40 % of the height, even when you only want the list (`z` hides it). |
| Build cost | Medium: a table, a drawer, and the task ↔ thread join (shared with B). |
