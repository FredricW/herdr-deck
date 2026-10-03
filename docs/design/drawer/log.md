# Drawer · Log tab

A timeline of the selected thread, newest first, built only from what
herdr-projects writes down. Conventions are in the [README](README.md).

## Where the events come from

Checked against a real herdr-projects project folder (herdr-projects at the
version this deck targets): the thread file holds a handful of timestamps,
and the inbox holds a timestamped item for most things that happen to a
thread. Handled items move to `inbox/done/` and stay there, so they are
history the deck can read. Reading `done/` is read-only, like everything
else the deck does.

| Event | Glyph | Source |
|---|---|---|
| created | `+` dim | `threads/t-NNNN.toml`: `created`, `base`, `branch` |
| launched | `▶` dim | the thread file: `launched_at`, `pane_id`; `brief_seen_at` adds "brief read" |
| new report | `≡` plain | inbox item, event `new report` (kind `thread-state`); the latest also from `last_report_change` |
| waiting on you | `●` red | inbox item, event `waiting on you` |
| blocked on a prompt | `●` red | inbox item, event `blocked on a prompt` |
| PR opened / updated | `◇` magenta | inbox item of kind `pr`, events `PR opened` / `PR updated`; the summary holds the state and comment count |
| checks failing | `✕` red | inbox item, event `PR checks failing`; the summary names the checks |
| PR merged | `✓` green | inbox item, event `PR merged` |
| prompted by a routine | `»` dim | inbox item of kind `routine`, event `prompted its thread` |
| resolved | `✓` dim | inbox item, event `resolved`; `resolved_reason` in the thread file |

An item still in `inbox/` (not handled) ends its line with a yellow `✉`.
Items are matched to the thread by their `subject` (the thread id).

**Not available, so not shown:** earlier working/idle changes (the thread
file keeps only `last_state` and `last_state_change`, which the card
already shows), percent or activity over time, when a review was given or
a check started (ticker.json keeps the PR's current state only, plus
`last_pr_check`), and the coordinator's messages to the thread. Commits on
the branch would be real, dated events too, but they come from git, not
herdr-projects; see the [open questions](README.md#open-questions).

## A thread that needs you

80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Admin rebuild                                ● 1 needs you  ◐ 1  ◇ 1  ○ 1  ✉ 1 │
│────────────────────────────────────────────────────────────────────────────────│
│    WORK                         THREAD  STATUS              PR      LINKS  DEV │
│  Needs you                                                                     │
│ ▸● Users page                   t-0002  needs you                   L4         │
│  ✉ inbox: t-0002 is now Waiting on you                                   3m    │
│                                                                                │
│  In progress                                                                   │
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
│ Users page                                                   ● needs you  ~95% │
│ t-0002 · Members /admin/users · pane w1Z:p1                         ▰▰▰▰▰▰▰▰▰▱ │
│── Overview ──── Files 11 ──━┫ Log 7 ┣━────────────────────────────── ↓ 2 more ─│
│   3m  ● waiting on you · needs you in pane w1Z:p1                      14:38 ✉ │
│   5m  ≡ new report · r shows it                                        14:36   │
│   1h  ≡ new report                                                     13:40   │
│   5h  ● blocked on a prompt                                            09:12   │
│  21h  ≡ new report                                                 Thu 17:30   │
│────────────────────────────────────────────────────────────────────────────────│
│ r report  ↵ go to pane  [ ] tab  z drawer  ? help                       v9.9.9 │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns:

```text
┌────────────────────────────────────────────────────────────┐
│ Admin rebuild                      ● 1  ◐ 1  ◇ 1  ○ 1  ✉ 1 │
│────────────────────────────────────────────────────────────│
│    WORK                        STATUS           LINKS  DEV │
│  Needs you                                                 │
│ ▸● Users page                  needs you        L4         │
│  ✉ inbox: t-0002 is now Waiting on you               3m    │
│                                                            │
│  In progress                                               │
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
│ Users page                               ● needs you  ~95% │
│ t-0002 · Members /admin/users                   ▰▰▰▰▰▰▰▰▰▱ │
│── Overview ──── Files 11 ──━┫ Log 7 ┣━────────── ↓ 2 more ─│
│   3m  ● waiting on you · pane w1Z:p1                     ✉ │
│   5m  ≡ new report · r shows it                            │
│   1h  ≡ new report                                         │
│   5h  ● blocked on a prompt                                │
│  21h  ≡ new report                                         │
│────────────────────────────────────────────────────────────│
│ r report  ↵ go to pane  [ ] tab  z drawer  ? help   v9.9.9 │
└────────────────────────────────────────────────────────────┘
```

- **Columns.** Age right-aligned and dim (`now`, `3m`, `5h`, `2d`, as the
  list's inbox rows count), the event glyph in its colour, the text plain
  with its details dim. At 80 columns the clock time sits at the right edge,
  dim, with the weekday for anything before today.
- **Newest first**, so the top of the tab answers "what just happened"; the
  start of the thread is a `pgdn` away.

## Full height (`z`)

80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Admin rebuild                                ● 1 needs you  ◐ 1  ◇ 1  ○ 1  ✉ 1 │
│────────────────────────────────────────────────────────────────────────────────│
│ Users page                                                   ● needs you  ~95% │
│ t-0002 · Members /admin/users · pane w1Z:p1                         ▰▰▰▰▰▰▰▰▰▱ │
│── Overview ──── Files 11 ──━┫ Log 7 ┣━─────────────────────────────────────────│
│ ── Today · Fri 2 Oct ──                                                        │
│   3m  ● waiting on you · needs you in pane w1Z:p1                      14:38 ✉ │
│   5m  ≡ new report · r shows it                                        14:36   │
│   1h  ≡ new report                                                     13:40   │
│       │                                                                        │
│   5h  ● blocked on a prompt in pane w1Z:p1                             09:12   │
│       │                                                                        │
│ ── Yesterday · Thu 1 Oct ──                                                    │
│  21h  ≡ new report                                                     17:30   │
│  22h  ▶ launched in pane w1Z:p1 · brief read 16:12                     16:11   │
│  22h  + created · hp/admin-rebuild/t-0002-members-admin-users          16:10   │
│         from origin/main                                                       │
│                                                                                │
│ from threads/t-0002.toml and 5 inbox items, 1 unhandled                        │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ r report  ↵ go to pane  [ ] tab  z drawer  ? help                       v9.9.9 │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns:

```text
┌────────────────────────────────────────────────────────────┐
│ Admin rebuild                      ● 1  ◐ 1  ◇ 1  ○ 1  ✉ 1 │
│────────────────────────────────────────────────────────────│
│ Users page                               ● needs you  ~95% │
│ t-0002 · Members /admin/users                   ▰▰▰▰▰▰▰▰▰▱ │
│── Overview ──── Files 11 ──━┫ Log 7 ┣━─────────────────────│
│ ── Today · Fri 2 Oct ──                                    │
│   3m  ● waiting on you · pane w1Z:p1                     ✉ │
│   5m  ≡ new report · r shows it                            │
│   1h  ≡ new report                                         │
│       │                                                    │
│   5h  ● blocked on a prompt                                │
│       │                                                    │
│ ── Yesterday · Thu 1 Oct ──                                │
│  21h  ≡ new report                                         │
│  22h  ▶ launched in pane w1Z:p1 · brief read               │
│  22h  + created from origin/main                           │
│                                                            │
│ from threads/t-0002.toml and 5 inbox items                 │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│────────────────────────────────────────────────────────────│
│ r report  ↵ go to pane  [ ] tab  z drawer  ? help   v9.9.9 │
└────────────────────────────────────────────────────────────┘
```

- **The rail.** With room to spare, a dim `│` under the glyph column marks
  a gap of more than an hour between two events, and a dim rule heads each
  day. At the normal height neither is drawn: every line is an event.
- A dim line after the oldest event names the sources, so it is clear the
  log is only what herdr-projects recorded.

## A thread in review

80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Admin rebuild                                               ◐ 2  ◇ 1  ○ 1  ✉ 0 │
│────────────────────────────────────────────────────────────────────────────────│
│    WORK                         THREAD  STATUS              PR      LINKS  DEV │
│  In progress                                                                   │
│  ◐ Users page                   t-0002  Building overview           L4         │
│  ◐ Templates page               t-0003  Writing tests               L F    ●●● │
│ ▸◇ Document select for summary  t-0004  review required     #2320   L F3   ●○  │
│                                                                                │
│  On hold until Monday 2026-10-05                                               │
│  ○ Subscriptions list           t-0001  idle                        L N    ~○  │
│                                                                                │
│  + Backlog (3)                                                                 │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ Document select for summary                                           ◇ review │
│ t-0004 · Summary select documents · pane w21:p1              #2320 ✕ 2 failing │
│── Overview ──── Files 4 ──━┫ Log 9 ┣━─────────────────────────────── ↓ 4 more ─│
│   2m  ✕ checks failing: lint, test (ubuntu-latest)                     14:39 ✉ │
│  25m  ◇ [5 #2320] updated · open · 2 comments                          14:16   │
│   3h  ≡ new report · r shows it                                        11:05   │
│   3h  ◇ [5 #2320] opened                                               11:03   │
│   4h  ≡ new report                                                     10:40   │
│────────────────────────────────────────────────────────────────────────────────│
│ 1-9 link  g PR  r report  [ ] tab  z drawer  ? help                     v9.9.9 │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns:

```text
┌────────────────────────────────────────────────────────────┐
│ Admin rebuild                           ◐ 2  ◇ 1  ○ 1  ✉ 0 │
│────────────────────────────────────────────────────────────│
│    WORK                        STATUS           LINKS  DEV │
│  In progress                                               │
│  ◐ Users page                  Building overv…  L4         │
│  ◐ Templates page              Writing tests    L F    ●●● │
│ ▸◇ Document select for summa…  #2320 review     L F3   ●○  │
│                                                            │
│  On hold until Monday 2026-10-05                           │
│  ○ Subscriptions list          idle             L N    ~○  │
│                                                            │
│  + Backlog (3)                                             │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│────────────────────────────────────────────────────────────│
│ Document select for summary                       ◇ review │
│ t-0004 · Summary select documents                #2320 ✕ 2 │
│── Overview ──── Files 4 ──━┫ Log 9 ┣━─────────── ↓ 4 more ─│
│   2m  ✕ checks failing: lint, test (ubunt…               ✉ │
│  25m  ◇ [5 #2320] updated · 2 comments                     │
│   3h  ≡ new report · r shows it                            │
│   3h  ◇ [5 #2320] opened                                   │
│   4h  ≡ new report                                         │
│────────────────────────────────────────────────────────────│
│ 1-9 link  g PR  r report  [ ] tab  ? help           v9.9.9 │
└────────────────────────────────────────────────────────────┘
```

Full height, 80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Admin rebuild                                               ◐ 2  ◇ 1  ○ 1  ✉ 0 │
│────────────────────────────────────────────────────────────────────────────────│
│ Document select for summary                                           ◇ review │
│ t-0004 · Summary select documents · pane w21:p1              #2320 ✕ 2 failing │
│── Overview ──── Files 4 ──━┫ Log 9 ┣━──────────────────────────────────────────│
│ ── Today · Fri 2 Oct ──                                                        │
│   2m  ✕ checks failing: lint, test (ubuntu-latest)                     14:39 ✉ │
│  25m  ◇ [5 #2320] updated · open · 2 comments                          14:16   │
│       │                                                                        │
│   3h  ≡ new report · r shows it                                        11:05   │
│   3h  ◇ [5 #2320] opened                                               11:03   │
│   4h  ≡ new report                                                     10:40   │
│   4h  ● waiting on you                                                 10:12   │
│   5h  » prompted by routine pr-followup                                09:30   │
│   6h  ▶ launched in pane w21:p1 · brief read 08:56                     08:55   │
│   6h  + created · hp/admin-rebuild/t-0004-summary from origin/main     08:54   │
│                                                                                │
│ from threads/t-0004.toml and 7 inbox items, 1 unhandled                        │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ 1-9 link  g PR  r report  [ ] tab  z drawer  ? help                     v9.9.9 │
└────────────────────────────────────────────────────────────────────────────────┘
```

- PR events carry the PR's chip, `[5 #2320]`, with the same digit as on
  Overview, so `5` opens the PR from here too. Digits never mean anything
  on Log that they do not mean on Overview.
- `✕ checks failing` is red, the PR chips magenta, `waiting on you` red even
  when handled: the glyph says what happened; only `✉` says it is still
  open.

## An inbox item in its thread's log

With an inbox row selected, the Log tab is the subject thread's, and the
cursor starts on the item. 80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Admin rebuild                                ● 1 needs you  ◐ 1  ◇ 1  ○ 1  ✉ 1 │
│────────────────────────────────────────────────────────────────────────────────│
│    WORK                         THREAD  STATUS              PR      LINKS  DEV │
│  Needs you                                                                     │
│  ● Users page                   t-0002  needs you                   L4         │
│ ▸✉ inbox: t-0002 is now Waiting on you                                   3m    │
│                                                                                │
│  In progress                                                                   │
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
│ t-0002 is now Waiting on you                                       ✉ inbox  3m │
│ thread-state · 14:38 · inbox/20261002T143012Z-thread-state-t-0002-7.md         │
│── Overview ──── Files 11 ──━┫ Log 7 ┣━────────────────────────────── ↓ 2 more ─│
│▸  3m  ● waiting on you · needs you in pane w1Z:p1                      14:38 ✉ │
│   5m  ≡ new report · r shows it                                        14:36   │
│   1h  ≡ new report                                                     13:40   │
│   5h  ● blocked on a prompt                                            09:12   │
│  21h  ≡ new report                                                 Thu 17:30   │
│────────────────────────────────────────────────────────────────────────────────│
│ ↵ go to pane  r report  [ ] tab  z drawer  ? help                       v9.9.9 │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns:

```text
┌────────────────────────────────────────────────────────────┐
│ Admin rebuild                      ● 1  ◐ 1  ◇ 1  ○ 1  ✉ 1 │
│────────────────────────────────────────────────────────────│
│    WORK                        STATUS           LINKS  DEV │
│  Needs you                                                 │
│  ● Users page                  needs you        L4         │
│ ▸✉ inbox: t-0002 is now Waiting on you               3m    │
│                                                            │
│  In progress                                               │
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
│ t-0002 is now Waiting on you                   ✉ inbox  3m │
│ thread-state · 14:38 · inbox/20261002T1430…                │
│── Overview ──── Files 11 ──━┫ Log 7 ┣━────────── ↓ 2 more ─│
│▸  3m  ● waiting on you · pane w1Z:p1                     ✉ │
│   5m  ≡ new report · r shows it                            │
│   1h  ≡ new report                                         │
│   5h  ● blocked on a prompt                                │
│  21h  ≡ new report                                         │
│────────────────────────────────────────────────────────────│
│ ↵ go to pane  r report  [ ] tab  ? help             v9.9.9 │
└────────────────────────────────────────────────────────────┘
```

- The item's line has the selection background and `▸`, as the drawer
  cursor does on Files. It is the same item as the list row, so this
  shows what happened just before it without leaving the row.
- On Log, `↵` acts on the event under the drawer cursor once the drawer has
  the focus: a report event shows the report (`r`), a PR event opens the PR,
  other events focus the thread's pane.
