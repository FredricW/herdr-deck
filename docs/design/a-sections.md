# A · Stacked sections

The draft in `docs/PLAN.md`, worked out: one screen, four stacked sections
(Tasks, Threads, Inbox, and dev servers folded under their threads), a summary
in the header and a "needs you" band pinned under it. Everything is visible at
once; `tab` moves between sections and `j`/`k` within one.

Shared colours, glyphs and sample data are in the [README](README.md#shared-conventions).
Each frame below is the pane itself: the box adds one column on each side.

## Normal use

Nothing needs you; two threads work, one PR waits for review, one thread is
on hold.

80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Admin rebuild                           ◐ 2 working  ◇ 1 review  ○ 1 idle  ✉ 0 │
│────────────────────────────────────────────────────────────────────────────────│
│ TASKS                                                                          │
│ In progress                                                                    │
│ ▸ Users page                t-0002 ◐  ABC-1246                            [L4] │
│   Templates page            t-0003 ◐  ABC-1051                         [L] [F] │
│   Document select           t-0004 ◇  ABC-1191                    [L] [F3] [G] │
│ On hold until Monday 2026-10-05                                                │
│   Subscriptions list        t-0001 ○  ABC-1472                         [L] [N] │
│                                                                                │
│ THREADS                                                                        │
│   ◐ t-0002 Members /admin/users                       Building overview · ~60% │
│   ◐ t-0003 Templates /templates                           Writing tests · ~70% │
│     ↳ :5181 frontend ●  :8011 api ●  :5441 pg ●                                │
│   ◇ t-0004 Summary select documents           PR #2320 · review · 2 comments ✓ │
│     ↳ :5174 frontend ●  :8002 api ○                                            │
│   ○ t-0001 Subscriptions /admin/plans                                     idle │
│                                                                                │
│ INBOX                                                        nothing unhandled │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ ↵ focus  l linear  f figma  n notion  g PR  o localhost  e editor  ? help      │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns:

```text
┌────────────────────────────────────────────────────────────┐
│ Admin rebuild                           ◐ 2  ◇ 1  ○ 1  ✉ 0 │
│────────────────────────────────────────────────────────────│
│ TASKS                                                      │
│ In progress                                                │
│ ▸ Users page             t-0002 ◐                     [L4] │
│   Templates page         t-0003 ◐                  [L] [F] │
│   Document select        t-0004 ◇             [L] [F3] [G] │
│ On hold until Monday 2026-10-05                            │
│   Subscriptions list     t-0001 ○                  [L] [N] │
│                                                            │
│ THREADS                                                    │
│   ◐ t-0002 Members /admin/users          Building overview │
│   ◐ t-0003 Templates /templates              Writing tests │
│     ↳ :5181 frontend ●  :8011 api ●  :5441 pg ●            │
│   ◇ t-0004 Summary select documen…         PR #2320 · 2c ✓ │
│     ↳ :5174 frontend ●  :8002 api ○                        │
│   ○ t-0001 Subscriptions /admin/…                     idle │
│                                                            │
│ INBOX                                    nothing unhandled │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│────────────────────────────────────────────────────────────│
│ ↵ focus  l f n g links  o local  e edit  ⇥ section  ? help │
└────────────────────────────────────────────────────────────┘
```

- **Colours.** Header counts take their state colour (◐ cyan, ◇ magenta,
  ○ dim, ✉ yellow when non-zero, dim at 0). Section titles bold; list headings
  (`In progress`, `On hold …`) bold, not upper-case. Thread titles default
  colour, the right-hand status in the state colour. `✓` after the PR is green
  (all checks pass). Port dots: `●` green = TCP connect succeeded, `○` dim =
  nothing listening.
- **Selection.** The selected row is reverse video across the full width; `▸`
  marks it as well, for terminals and screenshots without colour.
- **Badges.** `[L]` Linear, `[F]` Figma, `[N]` Notion, `[G]` GitHub PR; a digit
  (`[L4]`, `[F3]`) means several links of that kind, so the key opens a
  chooser. Badges are dim with a bold letter and are clickable.
- **At 60 columns** the Linear ID column goes (the badge still opens it), the
  thread status drops the percent, the PR line abbreviates comments to `2c`
  and titles truncate with `…`.

## A thread needs you

t-0002 finished phase 1 and waits for an answer. herdr-projects also dropped
an inbox item for it.

80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Admin rebuild                                ● 1 needs you  ◐ 1  ◇ 1  ○ 1  ✉ 1 │
│────────────────────────────────────────────────────────────────────────────────│
│ ● t-0002 Members /admin/users                            needs you · ~95% · 3m │
│────────────────────────────────────────────────────────────────────────────────│
│ TASKS                                                                          │
│ In progress                                                                    │
│   Users page                t-0002 ●  ABC-1246                            [L4] │
│   Templates page            t-0003 ◐  ABC-1051                         [L] [F] │
│   Document select           t-0004 ◇  ABC-1191                    [L] [F3] [G] │
│ On hold until Monday 2026-10-05                                                │
│   Subscriptions list        t-0001 ○  ABC-1472                         [L] [N] │
│                                                                                │
│ THREADS                                                                        │
│ ▸ ● t-0002 Members /admin/users                               needs you · ~95% │
│     next: Approve phase 1, then say whether to start ABC-1257                  │
│   ◇ t-0004 Summary select documents           PR #2320 · review · 2 comments ✓ │
│     ↳ :5174 frontend ●  :8002 api ○                                            │
│   ◐ t-0003 Templates /templates                           Writing tests · ~70% │
│     ↳ :5181 frontend ●  :8011 api ●  :5441 pg ●                                │
│   ○ t-0001 Subscriptions /admin/plans                                     idle │
│                                                                                │
│ INBOX                                                              1 unhandled │
│   ✉ t-0002 is now Waiting on you                                            3m │
│                                                                                │
│                                                                                │
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ ↵ go to pane  l linear  f figma  n notion  g PR  o localhost  e editor  ? help │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns:

```text
┌────────────────────────────────────────────────────────────┐
│ Admin rebuild            ● 1 needs you  ◐ 1  ◇ 1  ○ 1  ✉ 1 │
│────────────────────────────────────────────────────────────│
│ ● t-0002 Members /admin/users               needs you · 3m │
│────────────────────────────────────────────────────────────│
│ TASKS                                                      │
│ In progress                                                │
│   Users page             t-0002 ●                     [L4] │
│   Templates page         t-0003 ◐                  [L] [F] │
│   Document select        t-0004 ◇             [L] [F3] [G] │
│ On hold until Monday 2026-10-05                            │
│   Subscriptions list     t-0001 ○                  [L] [N] │
│                                                            │
│ THREADS                                                    │
│ ▸ ● t-0002 Members /admin/users           needs you · ~95% │
│     next: Approve phase 1, then say whether to start…      │
│   ◇ t-0004 Summary select documen…         PR #2320 · 2c ✓ │
│     ↳ :5174 frontend ●  :8002 api ○                        │
│   ◐ t-0003 Templates /templates              Writing tests │
│     ↳ :5181 frontend ●  :8011 api ●  :5441 pg ●            │
│   ○ t-0001 Subscriptions /admin/…                     idle │
│                                                            │
│ INBOX                                          1 unhandled │
│   ✉ t-0002 is now Waiting on you                        3m │
│                                                            │
│                                                            │
│                                                            │
│────────────────────────────────────────────────────────────│
│ ↵ go to pane  l f n g links  o local  e edit  ? help       │
└────────────────────────────────────────────────────────────┘
```

- **The band.** One row per thread in the `Waiting on you` group, between two
  rules, at most three rows (`+2 more` after that). Bold white on a red
  background; the band is clickable and `!` jumps to the first entry.
- **Ordering.** Threads sort by herdr-projects' `rank` (Waiting on you, Ready
  for review, Landing, Working, Idle), so the thread that needs you is first.
- **`next:`** The first line of the thread's `next[]` (from its report),
  shown only for threads that need you, dim italic.
- **Header.** `● 1 needs you` is bold red and comes first; the other counts
  shrink to glyph plus number.
- **Time.** `3m` is the age of `last_state_change`.

## Unhandled inbox items

Two items arrived; the coordinator has not handled them yet. The deck only
lists them: handling happens in the coordinator, because the deck never writes
to herdr-projects.

80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Admin rebuild                                ● 1 needs you  ◐ 1  ◇ 1  ○ 1  ✉ 2 │
│────────────────────────────────────────────────────────────────────────────────│
│ ● t-0002 Members /admin/users                            needs you · ~95% · 9m │
│────────────────────────────────────────────────────────────────────────────────│
│ TASKS                                                                          │
│ In progress                                                                    │
│   Users page                t-0002 ●  ABC-1246                            [L4] │
│   Templates page            t-0003 ◐  ABC-1051                         [L] [F] │
│   Document select           t-0004 ◇  ABC-1191                    [L] [F3] [G] │
│ On hold until Monday 2026-10-05                                                │
│   Subscriptions list        t-0001 ○  ABC-1472                         [L] [N] │
│                                                                                │
│ THREADS                                                                        │
│   ● t-0002 Members /admin/users                               needs you · ~95% │
│   ◇ t-0004 Summary select documents           PR #2320 · review · 2 comments ✓ │
│   ◐ t-0003 Templates /templates                           Writing tests · ~70% │
│   ○ t-0001 Subscriptions /admin/plans                                     idle │
│                                                                                │
│ INBOX                                                              2 unhandled │
│ ▸ ✉ t-0002 is now Waiting on you                                            9m │
│     thread-state · its pane w1Z:p1 shows the report                            │
│   ✉ t-0004 is now Ready for review                                          2m │
│     thread-state · PR #2320 opened                                             │
│                                                                                │
│                                                                                │
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ ↵ go to t-0004  g PR  ⇥ section  ? help                                        │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns:

```text
┌────────────────────────────────────────────────────────────┐
│ Admin rebuild                      ● 1  ◐ 1  ◇ 1  ○ 1  ✉ 2 │
│────────────────────────────────────────────────────────────│
│ ● t-0002 Members /admin/users               needs you · 9m │
│────────────────────────────────────────────────────────────│
│ TASKS                                                      │
│ In progress                                                │
│   Users page             t-0002 ●                     [L4] │
│   Templates page         t-0003 ◐                  [L] [F] │
│   Document select        t-0004 ◇             [L] [F3] [G] │
│ On hold until Monday 2026-10-05                            │
│   Subscriptions list     t-0001 ○                  [L] [N] │
│                                                            │
│ THREADS                                                    │
│   ● t-0002 Members /admin/users           needs you · ~95% │
│   ◇ t-0004 Summary select documen…         PR #2320 · 2c ✓ │
│   ◐ t-0003 Templates /templates              Writing tests │
│   ○ t-0001 Subscriptions /admin/…                     idle │
│                                                            │
│ INBOX                                          2 unhandled │
│   ✉ t-0002 is now Waiting on you                        9m │
│ ▸ ✉ t-0004 is now Ready for review                      2m │
│     thread-state · PR #2320 opened                         │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│────────────────────────────────────────────────────────────│
│ ↵ go to t-0004  g PR  ⇥ section  ? help                    │
└────────────────────────────────────────────────────────────┘
```

- **Inbox rows** are yellow: `✉`, `subject`/`summary` on the first line, age
  of `created` on the right. The second line (`kind` · rest of `summary`)
  shows for every item at 80 columns and only for the selected one at 60.
- **Threads compress.** With the inbox open the dev-server lines fold away
  (they come back when the Threads section has focus) to keep the screen to
  one page.
- **`↵`** on an inbox item whose `subject` is a thread id goes to that
  thread's pane; otherwise it goes to the coordinator's pane.

## Several links of one kind: the chooser

`l` on *Users page*, whose line and notes mention four Linear issues. A small
popup opens under the row and covers the rows below it.

80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Admin rebuild                           ◐ 2 working  ◇ 1 review  ○ 1 idle  ✉ 0 │
│────────────────────────────────────────────────────────────────────────────────│
│ TASKS                                                                          │
│ In progress                                                                    │
│ ▸ Users page                t-0002 ◐  ABC-1246                            [L4] │
│   ╭─ Linear · Users page ────────────────────────────────────────────────────╮ │
│   │ 1  ABC-1246   task line                                                  │ │
│   │ 2  ABC-1256   task note "Phase 1 = layout + overview (ABC-1256)"         │ │
│   │ 3  ABC-1257   task note                                                  │ │
│   │ 4  ABC-1250   task note                                                  │ │
│   │    1-4 open · a open all · esc close                                     │ │
│   ╰──────────────────────────────────────────────────────────────────────────╯ │
│   ◐ t-0003 Templates /templates                           Writing tests · ~70% │
│     ↳ :5181 frontend ●  :8011 api ●  :5441 pg ●                                │
│   ◇ t-0004 Summary select documents           PR #2320 · review · 2 comments ✓ │
│     ↳ :5174 frontend ●  :8002 api ○                                            │
│   ○ t-0001 Subscriptions /admin/plans                                     idle │
│                                                                                │
│ INBOX                                                        nothing unhandled │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ 1-4 open  a open all  j/k move  ↵ open  esc close                              │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns, here for `f` on *Document select*, whose thread report links
three Figma frames:

```text
┌────────────────────────────────────────────────────────────┐
│ Admin rebuild                           ◐ 2  ◇ 1  ○ 1  ✉ 0 │
│────────────────────────────────────────────────────────────│
│ TASKS                                                      │
│ In progress                                                │
│   Users page             t-0002 ◐                     [L4] │
│   Templates page         t-0003 ◐                  [L] [F] │
│ ▸ Document select        t-0004 ◇             [L] [F3] [G] │
│   ╭─ Figma · Document select ────────────────────────────╮ │
│   │ 1  Design 598:48083    t-0004 report                 │ │
│   │ 2  Design 1138:88367   t-0004 report                 │ │
│   │ 3  Design 635:76529    t-0004 report                 │ │
│   │    1-3 open · d desktop app · esc close              │ │
│   ╰──────────────────────────────────────────────────────╯ │
│     ↳ :5181 frontend ●  :8011 api ●  :5441 pg ●            │
│   ◇ t-0004 Summary select documen…         PR #2320 · 2c ✓ │
│     ↳ :5174 frontend ●  :8002 api ○                        │
│   ○ t-0001 Subscriptions /admin/…                     idle │
│                                                            │
│ INBOX                                    nothing unhandled │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│────────────────────────────────────────────────────────────│
│ 1-3 open  d desktop  j/k move  ↵ open  esc close           │
└────────────────────────────────────────────────────────────┘
```

- **Popup.** It overlays the rows under the selected row, full width, so
  nothing peeks out beside it. Rounded border in the link kind's colour,
  title bold. Each entry is a digit, the link's short label and where it was found (task line, task
  note, brief, report), dim. Figma labels are the file name plus `node-id`,
  since that is all the URL carries.
- **Keys.** Digits open directly; `a` opens all; `d` (Figma only) opens the
  desktop rewrite. A click on an entry opens it; a click outside closes the
  popup. The footer swaps to the chooser's keys while it is open.
- **One link only:** the key opens it straight away, no popup.

## Empty project

A fresh project: no `TASKS.md`, no threads, no inbox. The goal comes from
`PROJECT.md`.

80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Herdr Deck                                                     nothing running │
│────────────────────────────────────────────────────────────────────────────────│
│ Build herdr-deck v1 (milestones 1-7 in docs/PLAN.md): a Go + Charm status      │
│ pane that runs next to each herdr-projects coordinator …                       │
│                                                                                │
│ TASKS                                                                          │
│   No TASKS.md yet. The coordinator writes one when you give it work.           │
│                                                                                │
│ THREADS                                                                        │
│   No threads yet.                                                              │
│                                                                                │
│ INBOX                                                        nothing unhandled │
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
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ r reload  ? help                                                               │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns:

```text
┌────────────────────────────────────────────────────────────┐
│ Herdr Deck                                 nothing running │
│────────────────────────────────────────────────────────────│
│ Build herdr-deck v1 (milestones 1-7 in docs/PLAN.md): a    │
│ Go + Charm status pane that runs next to each …            │
│                                                            │
│ TASKS                                                      │
│   No TASKS.md yet. The coordinator writes one when         │
│   you give it work.                                        │
│                                                            │
│ THREADS                                                    │
│   No threads yet.                                          │
│                                                            │
│ INBOX                                    nothing unhandled │
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
│ r reload  ? help                                           │
└────────────────────────────────────────────────────────────┘
```

- The goal is dim italic, at most two lines. Empty-state text is dim.
- Link keys are hidden from the footer while there is nothing to open.

## A data source is missing

No ticker (`ticker.json` last checked 2 h ago), no herdr socket, and no
`.herdr-deck/dev.json` in the repo. The deck shows what the files still say and
marks the rest.

80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Admin rebuild                                               ◐ 2  ◇ 1  ○ 1  ✉ 0 │
│────────────────────────────────────────────────────────────────────────────────│
│ TASKS                                                                          │
│ In progress                                                                    │
│ ▸ Users page                t-0002 ◐  ABC-1246                            [L4] │
│   Templates page            t-0003 ◐  ABC-1051                         [L] [F] │
│   Document select           t-0004 ◇  ABC-1191                    [L] [F3] [G] │
│ On hold until Monday 2026-10-05                                                │
│   Subscriptions list        t-0001 ○  ABC-1472                         [L] [N] │
│                                                                                │
│ THREADS                                                            as of 14:23 │
│   ◐ t-0002 Members /admin/users                       Building overview · ~60% │
│   ◐ t-0003 Templates /templates                           Writing tests · ~70% │
│     ↳ :5291 port token ●                                           no dev.json │
│   ◇ t-0004 Summary select documents                   PR #2320 · state unknown │
│     ↳ :5174 port token ○                                           no dev.json │
│   ○ t-0001 Subscriptions /admin/plans                                     idle │
│                                                                                │
│ INBOX                                                        nothing unhandled │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ ! ticker stale 2h · ! no herdr socket · ! no dev.json in webshop               │
│────────────────────────────────────────────────────────────────────────────────│
│ l linear  f figma  n notion  g PR  o localhost  e editor  ? help               │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns:

```text
┌────────────────────────────────────────────────────────────┐
│ Admin rebuild                           ◐ 2  ◇ 1  ○ 1  ✉ 0 │
│────────────────────────────────────────────────────────────│
│ TASKS                                                      │
│ In progress                                                │
│ ▸ Users page             t-0002 ◐                     [L4] │
│   Templates page         t-0003 ◐                  [L] [F] │
│   Document select        t-0004 ◇             [L] [F3] [G] │
│ On hold until Monday 2026-10-05                            │
│   Subscriptions list     t-0001 ○                  [L] [N] │
│                                                            │
│ THREADS                                        as of 14:23 │
│   ◐ t-0002 Members /admin/users          Building overview │
│   ◐ t-0003 Templates /templates              Writing tests │
│     ↳ :5291 port token ●                                   │
│   ◇ t-0004 Summary select documen…            PR #2320 · ? │
│     ↳ :5174 port token ○                                   │
│   ○ t-0001 Subscriptions /admin/…                     idle │
│                                                            │
│ INBOX                                    nothing unhandled │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│────────────────────────────────────────────────────────────│
│ ! ticker stale 2h · ! no herdr · ! no dev.json             │
│────────────────────────────────────────────────────────────│
│ l f n g links  o local  e edit  ? help                     │
└────────────────────────────────────────────────────────────┘
```

- **Source line.** A yellow line above the footer, one `!` chip per missing
  source; hidden when all sources are healthy. Clicking a chip (or `?`) shows
  what is missing and what it costs.
- **No ticker.** Thread statuses come from the TOML/JSON as last written and
  are dim, with `as of 14:23` (the newest `updated`) on the section title. PR
  rows lose review, comments and checks; only the thread's own `pr_state`
  remains, or `state unknown`.
- **No herdr socket.** `↵` (go to pane) leaves the footer; "needs you" relies
  on `last_group` from the files.
- **No manifest.** Ports fall back to the workspace's `port` token, labelled
  `port token`, still probed for the dot. At 80 columns the reason is on the
  right; at 60 it is in the source line only.

## Good and bad

| | |
|---|---|
| Scanning | Good. Everything is on one page; the band and header answer "does anything need me" without moving. |
| Density | Medium. Each thread shows twice (as a task's `t-NNNN` and as a thread row), which costs about a third of the height. With 8+ threads or a long backlog the sections start to scroll separately. |
| Narrow width | Fair. At 60 columns the Linear IDs and percents go, thread titles truncate early because the right column is wide. |
| Keys | Few: `tab` + `j`/`k` + one letter per link kind, digits in the chooser. |
| Mouse | Good. Every row, badge, band entry and chooser entry is a click target; wheel scrolls the section under the pointer. |
| Build cost | Lowest. Closest to the M1 skeleton (sections + fake data). |
