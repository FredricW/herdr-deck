# Drawer · Overview tab

The tab the drawer opens on. Titled sections replace today's label column,
in a fixed order: **Next**, **PR**, **Note**, **Links**, **Dev**, **Thread**.
A section with nothing to show is left out. Conventions, colours and keys are
in the [README](README.md); every frame is the whole pane, 28 rows, and the
box adds one column on each side.

At the normal drawer height five lines are left for the tab's content. The
tab bar's right end names the sections that did not fit (`↓ Note · Thread`);
`pgdn`, the wheel or `z` reach them. Sections are separated by a blank line
only when the whole tab fits.

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
│━┫ Overview ┣━── Files 11 ──── Log 7 ─────────────────── ↓ Note · Dev · Thread ─│
│ ── Next ──                                                                     │
│ → Approve phase 1 (ABC-1256 overview)                                          │
│ → Say whether to start ABC-1257 and ABC-1250                                   │
│ ── Links ──                                                                    │
│ [1 ABC-1246 done] [2 ABC-1256 in progress] [3 ABC-1257 todo] [4 ABC-1250]      │
│────────────────────────────────────────────────────────────────────────────────│
│ ↵ go to pane  1-9 link  [ ] tab  r report  z drawer  ? help             v9.9.9 │
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
│━┫ Overview ┣━── Files 11 ──── Log 7 ──────────── ↓ 4 more ─│
│ ── Next ──                                                 │
│ → Approve phase 1 (ABC-1256 overview)                      │
│ → Say whether to start ABC-1257 and ABC-1250               │
│ ── Links ──                                                │
│ [1 ABC-1246 done] [2 ABC-1256 in progress]                 │
│────────────────────────────────────────────────────────────│
│ ↵ go to pane  1-9 link  [ ] tab  z drawer  ? help   v9.9.9 │
└────────────────────────────────────────────────────────────┘
```

- **Card.** Title bold; the pill `● needs you` bold red; `~95%` and the bar
  red too (the bar is the status colour, filled cells `▰`, empty `▱` dim).
  The second line is dim: thread id, the thread's own title when the row is a
  task, and the pane. At 60 columns the pane goes (it is in *Thread*).
- **Tab bar.** The active tab bold in a heavy bracket `━┫ … ┣━` in the
  status colour; other tabs dim on a dim rule. The `↓` hint is dim; at 60 columns it counts
  sections instead of naming them.
- **Next** has a red rule title while the thread needs you (dim otherwise);
  `→` bold. **Links** are chips: digit bold, Linear ID blue, the state in its
  Linear colour (*done* dim, *in progress* cyan, *todo* plain). The 60-column
  chip line wraps; its second line (`[3 ABC-1257 todo] [4 ABC-1250]`) is one
  `pgdn` away.

## Full height (`z`)

80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Admin rebuild                                ● 1 needs you  ◐ 1  ◇ 1  ○ 1  ✉ 1 │
│────────────────────────────────────────────────────────────────────────────────│
│ Users page                                                   ● needs you  ~95% │
│ t-0002 · Members /admin/users · pane w1Z:p1                         ▰▰▰▰▰▰▰▰▰▱ │
│━┫ Overview ┣━── Files 11 ──── Log 7 ───────────────────────────────────────────│
│                                                                                │
│ ── Next ──                                                                     │
│ → Approve phase 1 (ABC-1256 overview)                                          │
│ → Say whether to start ABC-1257 and ABC-1250                                   │
│                                                                                │
│ ── Note ──                                                                     │
│ Phase 1 = layout + overview (ABC-1256); report on 1257/1250 before starting    │
│ them.                                                                          │
│                                                                                │
│ ── Links ──                                                                    │
│ [1 ABC-1246 done] [2 ABC-1256 in progress] [3 ABC-1257 todo] [4 ABC-1250]      │
│                                                                                │
│ ── Dev ──                                                                      │
│ not started: no .dev/t-0002/state.json · u starts it                           │
│                                                                                │
│ ── Thread ──                                                                   │
│ pane     w1Z:p1 · claude done                                                  │
│ branch   hp/admin-rebuild/t-0002-members-admin-users                           │
│ report   threads/t-0002.md · changed 14:22 · r shows it                        │
│ base     origin/main · merge-base 4f1c2a9                                      │
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ ↵ go to pane  1-9 link  [ ] tab  r report  z drawer  ? help             v9.9.9 │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns:

```text
┌────────────────────────────────────────────────────────────┐
│ Admin rebuild                      ● 1  ◐ 1  ◇ 1  ○ 1  ✉ 1 │
│────────────────────────────────────────────────────────────│
│ Users page                               ● needs you  ~95% │
│ t-0002 · Members /admin/users                   ▰▰▰▰▰▰▰▰▰▱ │
│━┫ Overview ┣━── Files 11 ──── Log 7 ───────────────────────│
│                                                            │
│ ── Next ──                                                 │
│ → Approve phase 1 (ABC-1256 overview)                      │
│ → Say whether to start ABC-1257 and ABC-1250               │
│                                                            │
│ ── Note ──                                                 │
│ Phase 1 = layout + overview (ABC-1256); report on          │
│ 1257/1250 before starting them.                            │
│                                                            │
│ ── Links ──                                                │
│ [1 ABC-1246 done] [2 ABC-1256 in progress]                 │
│ [3 ABC-1257 todo] [4 ABC-1250]                             │
│                                                            │
│ ── Dev ──                                                  │
│ not started: no .dev/t-0002/state.json · u starts it       │
│                                                            │
│ ── Thread ──                                               │
│ pane    w1Z:p1 · claude done                               │
│ branch  hp/admin-rebuild/t-0002-members-admin-users        │
│ report  threads/t-0002.md · changed 14:22 · r              │
│                                                            │
│────────────────────────────────────────────────────────────│
│ ↵ go to pane  1-9 link  [ ] tab  z drawer  ? help   v9.9.9 │
└────────────────────────────────────────────────────────────┘
```

- The list is gone, as today; the card takes the drawer's title rule's place
  under the header. Blank lines now separate the sections, because
  everything fits.
- **Thread** is the old label column, kept for metadata only: dim labels,
  plain values. It is where the branch went (it does not fit in the card).
  The agent's live state (`claude done`) is coloured as today.

## A working thread with dev servers

80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Admin rebuild                                               ◐ 2  ◇ 1  ○ 1  ✉ 0 │
│────────────────────────────────────────────────────────────────────────────────│
│    WORK                         THREAD  STATUS              PR      LINKS  DEV │
│  In progress                                                                   │
│  ◐ Users page                   t-0002  Building overview           L4         │
│ ▸◐ Templates page               t-0003  Writing tests               L F    ●●● │
│  ◇ Document select for summary  t-0004  review required     #2320   L F3   ●○  │
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
│ Templates page                                                 ◐ working  ~40% │
│ t-0003 · Templates /templates · Writing tests · pane w20:p1         ▰▰▰▰▱▱▱▱▱▱ │
│━┫ Overview ┣━── Files 23 ──── Log 4 ──────────────────────────────── ↓ Thread ─│
│ ── Links ──                                                                    │
│ [1 ABC-1051 in progress] [2 Figma Templates]                                   │
│ ── Dev ──                                                                      │
│ :5181 frontend ●   :8011 api ●   :5441 pg ●                                    │
│ [3 Frontend ●]                                                                 │
│────────────────────────────────────────────────────────────────────────────────│
│ 1-9 link  l f n g o first  ↵ pane  [ ] tab  z drawer  ? help            v9.9.9 │
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
│ ▸◐ Templates page              Writing tests    L F    ●●● │
│  ◇ Document select for summa…  #2320 review     L F3   ●○  │
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
│ Templates page                             ◐ working  ~40% │
│ t-0003 · Writing tests                          ▰▰▰▰▱▱▱▱▱▱ │
│━┫ Overview ┣━── Files 23 ──── Log 4 ──────────── ↓ Thread ─│
│ ── Links ──                                                │
│ [1 ABC-1051 in progress] [2 Figma Templates]               │
│ ── Dev ──                                                  │
│ :5181 frontend ●  :8011 api ●  :5441 pg ●                  │
│ [3 Frontend ●]                                             │
│────────────────────────────────────────────────────────────│
│ 1-9 link  o open  ↵ pane  [ ] tab  z drawer  ? help v9.9.9 │
└────────────────────────────────────────────────────────────┘
```

- Pill, percent and bar cyan. The activity (`Writing tests`) sits on the
  card's second line, dim, since a working thread's state line says only
  "working".
- **Dev**: one entry per dev.json server, `●` green when listening, `○` dim
  when not, `~` before a fallback port. The chips under it are the manifest's
  localhost links, numbered after every other link; a chip whose server is
  down is dim. While `u` starts the servers the line ends in yellow
  `starting…`, and the log path moves into *Thread* (`log  ~/.local/state/…`).
- No *Next* or *Note*: the thread has no `next[]` and the task no notes.

## A thread in review, with failing checks

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
│━┫ Overview ┣━── Files 4 ──── Log 9 ─────────────────────────── ↓ Dev · Thread ─│
│ ── PR ──                                                                       │
│ [5 #2320] open · review required · 2 comments (sam, alex)                      │
│ ✕ 2 failing: lint, test (ubuntu-latest)                                        │
│ ── Links ──                                                                    │
│ [1 ABC-1191 in review] [2 Figma 598-48083] [3 1138-88367] [4 635-76529]        │
│────────────────────────────────────────────────────────────────────────────────│
│ 1-9 link  l f n g o first  ↵ pane  [ ] tab  z drawer  ? help            v9.9.9 │
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
│━┫ Overview ┣━── Files 4 ──── Log 9 ───────────── ↓ 3 more ─│
│ ── PR ──                                                   │
│ [5 #2320] open · review required · 2c                      │
│ ✕ 2 failing: lint, test (ubuntu-latest)                    │
│ ── Links ──                                                │
│ [1 ABC-1191 in review] [2 Figma 598-48083]                 │
│────────────────────────────────────────────────────────────│
│ 1-9 link  g PR  ↵ pane  [ ] tab  z drawer  ? help   v9.9.9 │
└────────────────────────────────────────────────────────────┘
```

Full height, 80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Admin rebuild                                               ◐ 2  ◇ 1  ○ 1  ✉ 0 │
│────────────────────────────────────────────────────────────────────────────────│
│ Document select for summary                                           ◇ review │
│ t-0004 · Summary select documents · pane w21:p1              #2320 ✕ 2 failing │
│━┫ Overview ┣━── Files 4 ──── Log 9 ────────────────────────────────────────────│
│                                                                                │
│ ── PR ──                                                                       │
│ [5 #2320] open · review required · 2 comments (sam, alex)                      │
│ ✕ 2 failing: lint, test (ubuntu-latest)                                        │
│ checked 1m ago                                                                 │
│                                                                                │
│ ── Links ──                                                                    │
│ [1 ABC-1191 in review] [2 Figma 598-48083] [3 1138-88367] [4 635-76529]        │
│                                                                                │
│ ── Dev ──                                                                      │
│ :5174 frontend ●   :8002 api ○                                                 │
│ [6 Frontend ●] [7 API docs ○]                                                  │
│                                                                                │
│ ── Thread ──                                                                   │
│ pane     w21:p1 · claude idle                                                  │
│ branch   hp/admin-rebuild/t-0004-summary                                       │
│ report   threads/t-0004.md · changed 11:05 · r shows it                        │
│ base     origin/main · merge-base 9d02e71                                      │
│                                                                                │
│                                                                                │
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ 1-9 link  l f n g o first  ↵ pane  [ ] tab  z drawer  ? help            v9.9.9 │
└────────────────────────────────────────────────────────────────────────────────┘
```

- No percent, so no bar: the card's right edge shows the PR instead, `#2320`
  magenta and `✕ 2 failing` red (green `✓` when nothing fails, nothing
  while the PR state is unknown).
- **PR** comes first when there is no *Next*: it is what a review row asks
  of you. The PR's own chip lives here, not in *Links*, so it is never shown
  twice; its digit still follows the kind order (Linear, Figma, Notion,
  GitHub, localhost). Review text: *review required* magenta, *approved*
  green, *changes requested* red. Commenter logins come from ticker.json's
  `commenters[]`, check names from `failing_checks[]`; ticker.json has no
  passing or pending checks, so the line is either `✕ N failing: …` (red) or
  `✓ no failing checks` (green). `checked 1m ago` is `last_pr_check`.
- **Chips of one kind in a row** drop the kind word after the first
  (`[2 Figma 598-48083] [3 1138-88367]`); Linear IDs never need one.

## A task without a thread

80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Admin rebuild                                               ◐ 2  ◇ 1  ○ 1  ✉ 0 │
│────────────────────────────────────────────────────────────────────────────────│
│    WORK                         THREAD  STATUS              PR      LINKS  DEV │
│  In progress                                                                   │
│  ◐ Users page                   t-0002  Building overview           L4         │
│  ◐ Templates page               t-0003  Writing tests               L F    ●●● │
│  ◇ Document select for summary  t-0004  review required     #2320   L F3   ●○  │
│                                                                                │
│  On hold until Monday 2026-10-05                                               │
│  ○ Subscriptions list           t-0001  idle                        L N    ~○  │
│                                                                                │
│  Backlog                                                                       │
│  · Settings page                                                               │
│  · Home page                                                                   │
│ ▸· Snippets page                                                    L          │
│                                                                                │
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ Snippets page                                                      ○ no thread │
│ Backlog · TASKS.md                                                             │
│━┫ Overview ┣━── Files ──── Log ────────────────────────────────────────────────│
│ ── Links ──                                                                    │
│ [1 ABC-1251 backlog]                                                           │
│                                                                                │
│                                                                                │
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ 1-9 link  l first  space fold  [ ] tab  z drawer  ? help                v9.9.9 │
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
│  ◇ Document select for summa…  #2320 review     L F3   ●○  │
│                                                            │
│  On hold until Monday 2026-10-05                           │
│  ○ Subscriptions list          idle             L N    ~○  │
│                                                            │
│  Backlog                                                   │
│  · Settings page                                           │
│  · Home page                                               │
│ ▸· Snippets page                                L          │
│                                                            │
│                                                            │
│────────────────────────────────────────────────────────────│
│ Snippets page                                  ○ no thread │
│ Backlog · TASKS.md                                         │
│━┫ Overview ┣━── Files ──── Log ────────────────────────────│
│ ── Links ──                                                │
│ [1 ABC-1251 backlog]                                       │
│                                                            │
│                                                            │
│                                                            │
│────────────────────────────────────────────────────────────│
│ 1-9 link  l first  [ ] tab  z drawer  ? help        v9.9.9 │
└────────────────────────────────────────────────────────────┘
```

- The pill is dim. A done task says `✓ done` (dim) instead; a task with an
  owner adds it to the second line (`Backlog · TASKS.md · owner sam`).
- **Files** and **Log** stay in the bar, dim and without counts, so the bar
  never jumps; `[`/`]` skip them and a click on one does nothing.
- With notes (the *Settings page* task) a **Note** section comes before
  *Links*.

## An inbox item

80 columns:

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
│━┫ Overview ┣━── Files 11 ──── Log 7 ───────────────────────────────────────────│
│ ── Subject ──                                                                  │
│ t-0002 Members /admin/users · ● needs you · ~95%                               │
│ ── Links ──                                                                    │
│ [1 ABC-1246 done] [2 ABC-1256 in progress] [3 ABC-1257 todo] [4 ABC-1250]      │
│ ↵ goes to t-0002's pane · the coordinator marks the item handled               │
│────────────────────────────────────────────────────────────────────────────────│
│ ↵ go to pane  1-9 link  [ ] tab  z drawer  ? help                       v9.9.9 │
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
│━┫ Overview ┣━── Files 11 ──── Log 7 ──────────── ↓ 1 more ─│
│ ── Subject ──                                              │
│ t-0002 Members /admin/users · ● needs you                  │
│ ── Links ──                                                │
│ [1 ABC-1246 done] [2 ABC-1256 in progress]                 │
│ [3 ABC-1257 todo] [4 ABC-1250]                             │
│────────────────────────────────────────────────────────────│
│ ↵ go to pane  1-9 link  [ ] tab  z drawer  ? help   v9.9.9 │
└────────────────────────────────────────────────────────────┘
```

- The card's title is the item's summary; the pill `✉ inbox` and its age are
  yellow. The second line is dim: kind, time, file.
- **Subject** is the subject thread in one line, its status in its colour.
  **Files** and **Log** are that thread's tabs: Log opens with this item
  marked (see [log.md](log.md#an-inbox-item-in-its-threads-log)), which
  shows what happened around it.
- A `pr` item has a *PR* section like the review row's; a `routine` or
  `space` item, which has no thread, has only *Summary* and dim Files/Log
  tabs.
