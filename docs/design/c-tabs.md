# C · Tabs, with a "Now" feed

One page at a time, chosen from a tab bar: **Now**, **Tasks**, **Threads**,
**Inbox**, **Servers**. Each page has the full height and width for one kind
of thing, so rows can take two lines and nothing needs to truncate hard. The
default page, *Now*, is a feed of threads sorted by how much they want you,
so the deck opens on the answer to "what should I look at".

Shared colours, glyphs and sample data are in the [README](README.md#shared-conventions).
Each frame below is the pane itself: the box adds one column on each side.

**Keys.** `1`–`5` or `tab`/`shift+tab` switch pages, `j`/`k` move, link
letters act on the selected row. A click on a tab switches to it.

## Normal use

80 columns, *Now*:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Admin rebuild                                               ◐ 2  ◇ 1  ○ 1  ✉ 0 │
│ Now   Tasks 4   Threads 4   Inbox   Servers 5/6                                │
│ ━━━                                                                            │
│ IN REVIEW                                                                      │
│ ▸ ◇ t-0004 Summary select documents                               PR #2320 [G] │
│     review required · 2 comments (@reviewer) · checks ✓                        │
│     Document select for summary · ABC-1191 [L] [F3]                            │
│                                                                                │
│ WORKING                                                                        │
│   ◐ t-0002 Members /admin/users                                           ~60% │
│     Building overview                                                          │
│     Users page · ABC-1246 [L4]                                                 │
│   ◐ t-0003 Templates /templates                                           ~70% │
│     Writing tests · :5181 ● :8011 ● :5441 ●                                    │
│     Templates page · ABC-1051 [L] [F]                                          │
│                                                                                │
│ IDLE                                                                           │
│   ○ t-0001 Subscriptions /admin/plans                                          │
│     Subscriptions list (on hold until Monday) · ABC-1472 [L] [N]               │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ 1-5 page  ↵ pane  l f n g link  o local  e edit  ? help                        │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns, *Now*:

```text
┌────────────────────────────────────────────────────────────┐
│ Admin rebuild                           ◐ 2  ◇ 1  ○ 1  ✉ 0 │
│ Now  Tasks 4  Threads 4  Inbox  Servers 5/6                │
│ ━━━                                                        │
│ IN REVIEW                                                  │
│ ▸ ◇ t-0004 Summary select documents              #2320 [G] │
│     review required · 2 comments · checks ✓                │
│     Document select for summary [L] [F3]                   │
│                                                            │
│ WORKING                                                    │
│   ◐ t-0002 Members /admin/users                       ~60% │
│     Building overview                                      │
│     Users page [L4]                                        │
│   ◐ t-0003 Templates /templates                       ~70% │
│     Writing tests · :5181 ● :8011 ● :5441 ●                │
│     Templates page [L] [F]                                 │
│                                                            │
│ IDLE                                                       │
│   ○ t-0001 Subscriptions /admin/plans                      │
│     Subscriptions list (on hold) [L] [N]                   │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│────────────────────────────────────────────────────────────│
│ 1-5 page  ↵ pane  l f n g o links  e edit  ? help          │
└────────────────────────────────────────────────────────────┘
```

The other pages, one width each (both widths follow the same rules as *Now*).

*Tasks*, 60 columns: TASKS.md as written, list notes included.

```text
┌────────────────────────────────────────────────────────────┐
│ Admin rebuild                           ◐ 2  ◇ 1  ○ 1  ✉ 0 │
│ Now  Tasks 4  Threads 4  Inbox  Servers 5/6                │
│      ━━━━━━━                                               │
│ IN PROGRESS                                                │
│ ▸ Users page                                 ◐ t-0002 [L4] │
│     Phase 1 = layout + overview (ABC-1256); report on      │
│     1257/1250 before starting them.                        │
│   Templates page                          ◐ t-0003 [L] [F] │
│   Document select for summary            ◇ t-0004 [L] [F3] │
│   No PR until the user gives a go-ahead.                   │
│                                                            │
│ ON HOLD UNTIL MONDAY 2026-10-05                            │
│   Subscriptions list                      ○ t-0001 [L] [N] │
│     Was to replace /admin/subscriptions.                   │
│     Writing a findings note …                              │
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
│ 1-5 page  ↵ thread  l f n g o links  ? help                │
└────────────────────────────────────────────────────────────┘
```

*Servers*, 80 columns: every worktree's dev servers in one place.

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Admin rebuild                                               ◐ 2  ◇ 1  ○ 1  ✉ 0 │
│ Now   Tasks 4   Threads 4   Inbox   Servers 5/6                                │
│                                     ━━━━━━━━━━━                                │
│ t-0003 Templates /templates                                 webshop · dev.json │
│ ▸ ● :5181  frontend                                      http://localhost:5181 │
│   ● :8011  api                                      http://localhost:8011/docs │
│   ● :5441  pg                                                                  │
│                                                                                │
│ t-0004 Summary select documents                             webshop · dev.json │
│   ● :5174  frontend                                      http://localhost:5174 │
│   ○ :8002  api                                                   not listening │
│                                                                                │
│ t-0002, t-0001                                       no state.json in worktree │
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
│ 1-5 page  ↵ open  c copy URL  e edit  ? help                                   │
└────────────────────────────────────────────────────────────────────────────────┘
```

- **Tab bar.** The active tab is bold with a heavy underline (`━`) on the
  next row; the others dim. A tab's count takes the colour of what it counts
  when it matters (Inbox yellow when non-zero; Servers green, or red when a
  port that dev.json lists is down, as in `5/6`).
- **Now** groups by herdr-projects group in `rank` order with bold section
  titles. Each thread is three lines: id + title + the main fact on the
  right; activity or PR detail; the task it belongs to with its link badges.
  Commenter names come from the ticker's `commenters[]`.
- **Tasks** puts list headings upper-case; list notes (paragraphs under a
  heading) dim italic; task notes dim under the task. Only the selected
  task's notes are unfolded at 60 columns.
- **Servers** shows the URLs the manifest's `links[]` build; `↵` opens one.

## A thread needs you

80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Admin rebuild                                ● 1 needs you  ◐ 1  ◇ 1  ○ 1  ✉ 1 │
│ Now ●   Tasks 4   Threads 4   Inbox 1   Servers 5/6                            │
│ ━━━━━                                                                          │
│ NEEDS YOU                                                                      │
│ ▸ ● t-0002 Members /admin/users                                             3m │
│     needs you · ~95%                                                           │
│     → Approve phase 1 (ABC-1256 overview)                                      │
│     → Say whether to start ABC-1257 and ABC-1250                               │
│     Users page · ABC-1246 [L4]                                                 │
│                                                                                │
│ IN REVIEW                                                                      │
│   ◇ t-0004 Summary select documents                               PR #2320 [G] │
│     review required · 2 comments (@reviewer) · checks ✓                        │
│                                                                                │
│ WORKING                                                                        │
│   ◐ t-0003 Templates /templates                                           ~70% │
│     Writing tests · :5181 ● :8011 ● :5441 ●                                    │
│                                                                                │
│ IDLE                                                                           │
│   ○ t-0001 Subscriptions /admin/plans                                          │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ ↵ go to pane  1-5 page  l f n g link  o local  ? help                          │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns, while on another page (*Tasks*):

```text
┌────────────────────────────────────────────────────────────┐
│ Admin rebuild            ● 1 needs you  ◐ 1  ◇ 1  ○ 1  ✉ 1 │
│ Now ●  Tasks 4  Threads 4  Inbox 1  Servers 5/6            │
│      ━━━━━━━                                               │
│ IN PROGRESS                                                │
│   Users page                                 ● t-0002 [L4] │
│     Phase 1 = layout + overview (ABC-1256); report on      │
│     1257/1250 before starting them.                        │
│ ▸ Templates page                          ◐ t-0003 [L] [F] │
│   Document select for summary            ◇ t-0004 [L] [F3] │
│   No PR until the user gives a go-ahead.                   │
│                                                            │
│ ON HOLD UNTIL MONDAY 2026-10-05                            │
│   Subscriptions list                      ○ t-0001 [L] [N] │
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
│ ● t-0002 needs you · 3m                           1 to see │
│────────────────────────────────────────────────────────────│
│ 1-5 page  ↵ thread  l f n g o links  ? help                │
└────────────────────────────────────────────────────────────┘
```

- **On *Now*** a *Needs you* group appears first, red title, rows bold red,
  with the thread's `next[]` lines as `→` items.
- **On any other page** the *Now* tab shows a red `●`, and a red strip above
  the footer names the thread; a click on it, or `1`, goes to *Now*. This is
  the cost of tabs: without the strip a need would be invisible from four of
  the five pages.
- The deck never switches page by itself, even when something needs you.

## Unhandled inbox items

80 columns, *Inbox*:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Admin rebuild                                ● 1 needs you  ◐ 1  ◇ 1  ○ 1  ✉ 2 │
│ Now ●   Tasks 4   Threads 4   Inbox 2   Servers 5/6                            │
│                               ━━━━━━━                                          │
│ UNHANDLED                                                                      │
│   ✉ t-0002 is now Waiting on you                                    14:23 · 9m │
│     thread-state · its pane w1Z:p1 shows the report: `thread read              │
│     admin-rebuild t-0002` shows it                                             │
│ ▸ ✉ t-0004 is now Ready for review                                  14:30 · 2m │
│     thread-state · PR #2320 opened                                             │
│                                                                                │
│ HANDLED TODAY                                                                6 │
│   t-0002 is now Waiting on you                                           14:23 │
│   t-0004 is now Waiting on you                                           14:13 │
│   t-0004 is now Waiting on you                                           14:11 │
│   …                                                                            │
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
│ ↵ go to t-0004  g PR  1-5 page  ? help                                         │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns, *Inbox*:

```text
┌────────────────────────────────────────────────────────────┐
│ Admin rebuild                      ● 1  ◐ 1  ◇ 1  ○ 1  ✉ 2 │
│ Now ●  Tasks 4  Threads 4  Inbox 2  Servers 5/6            │
│                            ━━━━━━━                         │
│ UNHANDLED                                                  │
│   ✉ t-0002 is now Waiting on you                        9m │
│     thread-state · its pane w1Z:p1 shows the               │
│     report                                                 │
│ ▸ ✉ t-0004 is now Ready for review                      2m │
│     thread-state · PR #2320 opened                         │
│                                                            │
│ HANDLED TODAY                                            6 │
│   t-0002 is now Waiting on you                       14:23 │
│   t-0004 is now Waiting on you                       14:13 │
│   …                                                        │
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
│ ↵ go to t-0004  g PR  1-5 page  ? help                     │
└────────────────────────────────────────────────────────────┘
```

- With its own page the inbox has room for the whole `summary` (wrapped) and
  for today's handled items from `inbox/done/` (dim), which the other
  directions leave out. Reading `done/` is still read-only.
- *Now* also lists unhandled items at the top, under the needs-you group,
  one line each; the Inbox tab count is yellow.

## Several links of one kind: the chooser

A bottom sheet slides up over the lower part of the page.

80 columns, `l` on *t-0002* in *Now*:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Admin rebuild                                               ◐ 2  ◇ 1  ○ 1  ✉ 0 │
│ Now   Tasks 4   Threads 4   Inbox   Servers 5/6                                │
│ ━━━                                                                            │
│ IN REVIEW                                                                      │
│   ◇ t-0004 Summary select documents                               PR #2320 [G] │
│     review required · 2 comments (@reviewer) · checks ✓                        │
│     Document select for summary · ABC-1191 [L] [F3]                            │
│                                                                                │
│ WORKING                                                                        │
│ ▸ ◐ t-0002 Members /admin/users                                           ~60% │
│     Building overview                                                          │
│     Users page · ABC-1246 [L4]                                                 │
│   ◐ t-0003 Templates /templates                                           ~70% │
│─ Linear · Users page / t-0002 ─────────────────────────────────────────────────│
│   1  ABC-1246                                                        task line │
│   2  ABC-1256                                                        task note │
│   3  ABC-1257                                                        task note │
│   4  ABC-1250                                                        task note │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ 1-4 open  a all  j/k ↵ open  esc close                                         │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns, `f` on *t-0004*:

```text
┌────────────────────────────────────────────────────────────┐
│ Admin rebuild                           ◐ 2  ◇ 1  ○ 1  ✉ 0 │
│ Now  Tasks 4  Threads 4  Inbox  Servers 5/6                │
│ ━━━                                                        │
│ IN REVIEW                                                  │
│ ▸ ◇ t-0004 Summary select documents              #2320 [G] │
│     review required · 2 comments · checks ✓                │
│     Document select for summary [L] [F3]                   │
│                                                            │
│ WORKING                                                    │
│   ◐ t-0002 Members /admin/users                       ~60% │
│     Building overview                                      │
│─ Figma · t-0004 ───────────────────────────────────────────│
│   1  Design 598:48083                        t-0004 report │
│   2  Design 1138:88367                       t-0004 report │
│   3  Design 635:76529                        t-0004 report │
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
│ 1-3 open  d desktop  a all  esc close                      │
└────────────────────────────────────────────────────────────┘
```

- The sheet's top rule carries the link kind and the row's name in the kind's
  colour; entries as in A (digit, label, where found). It always sits at the
  same place, so a mouse user learns where to look.

## Empty project

80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Herdr Deck                                                     nothing running │
│ Now   Tasks   Threads   Inbox   Servers                                        │
│ ━━━                                                                            │
│ Nothing needs you, and no thread is running.                                   │
│                                                                                │
│ Build herdr-deck v1 (milestones 1-7 in docs/PLAN.md): a Go + Charm status      │
│ pane that runs next to each herdr-projects coordinator …                       │
│                                                                                │
│ Tasks appear when the coordinator writes TASKS.md; threads when it starts      │
│ them.                                                                          │
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
│                                                                                │
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ 1-5 page  r reload  ? help                                                     │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns:

```text
┌────────────────────────────────────────────────────────────┐
│ Herdr Deck                                 nothing running │
│ Now  Tasks  Threads  Inbox  Servers                        │
│ ━━━                                                        │
│ Nothing needs you, and no thread is running.               │
│                                                            │
│ Build herdr-deck v1 (milestones 1-7 in docs/PLAN.md): a    │
│ Go + Charm status pane that runs next to each …            │
│                                                            │
│ Tasks appear when the coordinator writes TASKS.md;         │
│ threads when it starts them.                               │
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
│                                                            │
│                                                            │
│                                                            │
│────────────────────────────────────────────────────────────│
│ 1-5 page  r reload  ? help                                 │
└────────────────────────────────────────────────────────────┘
```

- Tabs without counts are dim; they stay clickable and show their own empty
  text.

## A data source is missing

80 columns, *Now*:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Admin rebuild                                               ◐ 2  ◇ 1  ○ 1  ✉ 0 │
│ Now   Tasks 4   Threads 4   Inbox   Servers ?   ! 3                            │
│ ━━━                                                                            │
│ IN REVIEW                                                          as of 14:23 │
│ ▸ ◇ t-0004 Summary select documents                               PR #2320 [G] │
│     review state unknown (ticker stale 2h)                                     │
│     Document select for summary · ABC-1191 [L] [F3]                            │
│                                                                                │
│ WORKING                                                                        │
│   ◐ t-0002 Members /admin/users                                           ~60% │
│     Building overview                                                          │
│     Users page · ABC-1246 [L4]                                                 │
│   ◐ t-0003 Templates /templates                                           ~70% │
│     Writing tests · :5291 port token ●                                         │
│     Templates page · ABC-1051 [L] [F]                                          │
│                                                                                │
│ IDLE                                                                           │
│   ○ t-0001 Subscriptions /admin/plans                                          │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ 1-5 page  l f n g link  o local  e edit  ? help                                │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns, the `!` tab (sources):

```text
┌────────────────────────────────────────────────────────────┐
│ Admin rebuild                           ◐ 2  ◇ 1  ○ 1  ✉ 0 │
│ Now  Tasks 4  Threads 4  Inbox  Servers ?  ! 3             │
│                                            ━━━             │
│ SOURCES                                                    │
│   ✓ PROJECT.md, TASKS.md, inbox/                read 14:41 │
│   ✓ thread list --json                          read 14:41 │
│   ! ticker.json                      last check 12:38 (2h) │
│     PR review, comments and checks are not shown.          │
│   ! herdr socket                                 not found │
│     ↵ cannot focus panes; status from files only.          │
│   ! .herdr-deck/dev.json                    not in webshop │
│     Ports from the workspace port token instead.           │
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
│                                                            │
│────────────────────────────────────────────────────────────│
│ 1-5 page  r retry  ? help                                  │
└────────────────────────────────────────────────────────────┘
```

- A sixth tab, `! 3` in yellow, appears only when something is missing and
  explains each source. Elsewhere stale rows are dim with `as of`, as in A.

## Good and bad

| | |
|---|---|
| Scanning | Good on *Now*, poor elsewhere: from *Tasks* you see a need only through the red strip, and a glance at the pane shows one page, not the project. |
| Density | Low per page, but the most room per item: three-line thread rows, wrapped inbox summaries, full URLs on *Servers*. |
| Narrow width | Best of the four: every page is one column of one kind, so 60 columns loses little. |
| Keys | Most: a page switch before most actions (`1`–`5`), then `j`/`k`. |
| Mouse | Good: tabs are big targets; the bottom sheet is always in the same place. |
| Extra | The only direction with room for handled inbox history, a full servers view and a sources page. |
| Build cost | Medium: five small views instead of one big one; each is simple. |
