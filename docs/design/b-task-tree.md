# B · Task tree

Tasks are the spine. Each task is a tree node; its threads, their PRs and
their dev servers hang under it, so every piece of work appears once. Threads
that no task mentions collect in a last group, *Not in TASKS.md*. Nodes fold
with `space` or a click on the `▾`/`▸`.

Shared colours, glyphs and sample data are in the [README](README.md#shared-conventions).
Each frame below is the pane itself: the box adds one column on each side.

**Keys.** `j`/`k` move over every visible row, `space` folds or unfolds the
node, `[`/`]` jump between lists, `!` jumps to the next row that needs you.
The link letters (`l f n g o`) act on the selected node and everything under
it, so `g` on a task opens its thread's PR. Because `l` opens Linear, folding
uses `space`, not `h`/`l`.

## Normal use

80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Admin rebuild                           ◐ 2 working  ◇ 1 review  ○ 1 idle  ✉ 0 │
│────────────────────────────────────────────────────────────────────────────────│
│ In progress                                                                  3 │
│ ▾ Users page                                                     ABC-1246 [L4] │
│   └ ◐ t-0002 Members /admin/users                     Building overview · ~60% │
│ ▾ Templates page                                              ABC-1051 [L] [F] │
│   └ ◐ t-0003 Templates /templates                         Writing tests · ~70% │
│       :5181 frontend ●   :8011 api ●   :5441 pg ●                              │
│ ▾ Document select for summary                                ABC-1191 [L] [F3] │
│   └ ◇ t-0004 Summary select documents                             PR #2320 [G] │
│       review required · 2 comments · checks ✓                                  │
│       :5174 frontend ●   :8002 api ○                                           │
│                                                                                │
│ On hold until Monday 2026-10-05                                              1 │
│›▸ Subscriptions list                                ○ t-0001  ABC-1472 [L] [N] │
│                                                                                │
│ Not in TASKS.md                                                              0 │
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
│ space fold  ↵ pane  l f n g link  o local  e edit  ! next need  ? help         │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns:

```text
┌────────────────────────────────────────────────────────────┐
│ Admin rebuild                           ◐ 2  ◇ 1  ○ 1  ✉ 0 │
│────────────────────────────────────────────────────────────│
│ In progress                                              3 │
│ ▾ Users page                                          [L4] │
│   └ ◐ t-0002 Members /admin/use…         Building overview │
│ ▾ Templates page                                   [L] [F] │
│   └ ◐ t-0003 Templates /templates            Writing tests │
│       :5181 frontend ●  :8011 api ●  :5441 pg ●            │
│ ▾ Document select for summary                     [L] [F3] │
│   └ ◇ t-0004 Summary select doc…              PR #2320 [G] │
│       review required · 2c · checks ✓                      │
│       :5174 frontend ●  :8002 api ○                        │
│                                                            │
│ On hold until Monday 2026-10-05                          1 │
│›▸ Subscriptions list                      ○ t-0001 [L] [N] │
│                                                            │
│ Not in TASKS.md                                          0 │
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
│ space fold  ↵ pane  l f n g o links  ! need  ? help        │
└────────────────────────────────────────────────────────────┘
```

- **Task rows** are bold; the list headings are dim upper-and-lower case with
  a dim count on the right. Thread rows are normal weight, their status in
  the state colour. Tree lines (`└`, `├`) are dim.
- **Folded node** (`▸`): the task's row carries its threads' glyphs and ids,
  so a folded tree still shows state. Lists whose heading says *hold* or
  *backlog* start folded; *In progress* starts open. The fold state is kept
  per project for the session.
- **Badges** on the task row count links from the task line, its notes and
  its threads' brief and report together. The PR badge `[G]` sits on the
  thread row, where the PR belongs.
- **Selection.** Reverse video row; in the mockups a `›` in the first column
  marks it, because `▸`/`▾` here mean folded/unfolded. In this mock the
  selected row is *Subscriptions list*.

## A thread needs you

80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Admin rebuild                                ● 1 needs you  ◐ 1  ◇ 1  ○ 1  ✉ 1 │
│────────────────────────────────────────────────────────────────────────────────│
│ In progress                                                                  3 │
│┃▾ Users page                                                     ABC-1246 [L4] │
│┃  └ ● t-0002 Members /admin/users                        needs you · ~95% · 3m │
│┃      ├ Approve phase 1 (ABC-1256 overview)                                    │
│┃      └ Say whether to start ABC-1257 and ABC-1250                             │
│ ▾ Templates page                                              ABC-1051 [L] [F] │
│   └ ◐ t-0003 Templates /templates                         Writing tests · ~70% │
│       :5181 frontend ●   :8011 api ●   :5441 pg ●                              │
│ ▾ Document select for summary                                ABC-1191 [L] [F3] │
│   └ ◇ t-0004 Summary select documents                             PR #2320 [G] │
│       review required · 2 comments · checks ✓                                  │
│       :5174 frontend ●   :8002 api ○                                           │
│                                                                                │
│ On hold until Monday 2026-10-05                                              1 │
│ ▸ Subscriptions list                                ○ t-0001  ABC-1472 [L] [N] │
│                                                                                │
│ Not in TASKS.md                                                              0 │
│                                                                                │
│ Inbox                                                                        1 │
│   ✉ t-0002 is now Waiting on you                                            3m │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ ↵ go to pane  ! next need  space fold  l f n g o links  ? help                 │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns:

```text
┌────────────────────────────────────────────────────────────┐
│ Admin rebuild            ● 1 needs you  ◐ 1  ◇ 1  ○ 1  ✉ 1 │
│────────────────────────────────────────────────────────────│
│ In progress                                              3 │
│┃▾ Users page                                          [L4] │
│┃  └ ● t-0002 Members /admin/use…            needs you · 3m │
│┃      ├ Approve phase 1 (ABC-1256 overview)                │
│┃      └ Say whether to start ABC-1257 and ABC-…            │
│ ▾ Templates page                                   [L] [F] │
│   └ ◐ t-0003 Templates /templates            Writing tests │
│       :5181 frontend ●  :8011 api ●  :5441 pg ●            │
│ ▾ Document select for summary                     [L] [F3] │
│   └ ◇ t-0004 Summary select doc…              PR #2320 [G] │
│       review required · 2c · checks ✓                      │
│       :5174 frontend ●  :8002 api ○                        │
│                                                            │
│ On hold until Monday 2026-10-05                          1 │
│ ▸ Subscriptions list                      ○ t-0001 [L] [N] │
│                                                            │
│ Not in TASKS.md                                          0 │
│                                                            │
│ Inbox                                                    1 │
│   ✉ t-0002 is now Waiting on you                        3m │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│────────────────────────────────────────────────────────────│
│ ↵ pane  ! next need  space fold  l f n g o  ? help         │
└────────────────────────────────────────────────────────────┘
```

- **No separate band.** The task stays where TASKS.md puts it; instead a red
  bar `┃` runs down the left edge of the whole node, the thread row is bold
  red, and the node unfolds itself (even inside a folded list) to show the
  thread's `next[]` lines from its report, dim. When a needs-you node is
  scrolled out of view, a one-line red `▲ 1 needs you above` or
  `▼ … below` marker pins to the top or bottom edge; clicking it or `!`
  scrolls there.
- **Header.** `● 1 needs you` first, bold red.
- **Selection** is on the t-0002 thread row (reverse video; the red bar
  takes the first column, so no `›` here).
- **Inbox** appears as the last group only when it has unhandled items.

## Unhandled inbox items

The inbox is the one part not tied to a task, so it shows as the last group,
yellow, and the header's `✉ 2` jumps there on click (or `i`).

80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Admin rebuild                                ● 1 needs you  ◐ 1  ◇ 1  ○ 1  ✉ 2 │
│────────────────────────────────────────────────────────────────────────────────│
│ In progress                                                                  3 │
│┃▸ Users page                                           ● t-0002  ABC-1246 [L4] │
│ ▸ Templates page                                    ◐ t-0003  ABC-1051 [L] [F] │
│ ▾ Document select for summary                                ABC-1191 [L] [F3] │
│   └ ◇ t-0004 Summary select documents                             PR #2320 [G] │
│       review required · 2 comments · checks ✓                                  │
│       :5174 frontend ●   :8002 api ○                                           │
│                                                                                │
│ On hold until Monday 2026-10-05                                              1 │
│ ▸ Subscriptions list                                ○ t-0001  ABC-1472 [L] [N] │
│                                                                                │
│ Not in TASKS.md                                                              0 │
│                                                                                │
│ Inbox                                                                        2 │
│   ✉ t-0002 is now Waiting on you                                            9m │
│     thread-state · pane w1Z:p1 shows the report                                │
│› ✉ t-0004 is now Ready for review                                           2m │
│     thread-state · PR #2320 opened                                             │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ ↵ go to t-0004  g PR  i inbox  ! next need  ? help                             │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns:

```text
┌────────────────────────────────────────────────────────────┐
│ Admin rebuild                      ● 1  ◐ 1  ◇ 1  ○ 1  ✉ 2 │
│────────────────────────────────────────────────────────────│
│ In progress                                              3 │
│┃▸ Users page                                 ● t-0002 [L4] │
│ ▸ Templates page                          ◐ t-0003 [L] [F] │
│ ▾ Document select for summary                     [L] [F3] │
│   └ ◇ t-0004 Summary select doc…              PR #2320 [G] │
│       review required · 2c · checks ✓                      │
│       :5174 frontend ●  :8002 api ○                        │
│                                                            │
│ On hold until Monday 2026-10-05                          1 │
│ ▸ Subscriptions list                      ○ t-0001 [L] [N] │
│                                                            │
│ Not in TASKS.md                                          0 │
│                                                            │
│ Inbox                                                    2 │
│   ✉ t-0002 is now Waiting on you                        9m │
│› ✉ t-0004 is now Ready for review                       2m │
│     thread-state · PR #2320 opened                         │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│────────────────────────────────────────────────────────────│
│ ↵ go to t-0004  g PR  i inbox  ! need  ? help              │
└────────────────────────────────────────────────────────────┘
```

- Here the user has folded *Users page* and *Templates page*; the folded
  *Users page* keeps its red bar and `●`, so folding never hides a need.
- Inbox rows behave as in A: `↵` goes to the subject thread's pane, or to the
  coordinator's when the subject is not a thread. The deck does not mark
  items handled.

## Several links of one kind: the chooser

The chooser unfolds inline as one row under the node, so nothing is covered.
Digits open; `esc` folds it away.

80 columns, `l` on *Users page*:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Admin rebuild                           ◐ 2 working  ◇ 1 review  ○ 1 idle  ✉ 0 │
│────────────────────────────────────────────────────────────────────────────────│
│ In progress                                                                  3 │
│›▾ Users page                                                     ABC-1246 [L4] │
│   ├ Linear  1 ABC-1246  2 ABC-1256  3 ABC-1257  4 ABC-1250  a all              │
│   └ ◐ t-0002 Members /admin/users                     Building overview · ~60% │
│ ▾ Templates page                                              ABC-1051 [L] [F] │
│   └ ◐ t-0003 Templates /templates                         Writing tests · ~70% │
│       :5181 frontend ●   :8011 api ●   :5441 pg ●                              │
│ ▾ Document select for summary                                ABC-1191 [L] [F3] │
│   └ ◇ t-0004 Summary select documents                             PR #2320 [G] │
│       review required · 2 comments · checks ✓                                  │
│       :5174 frontend ●   :8002 api ○                                           │
│                                                                                │
│ On hold until Monday 2026-10-05                                              1 │
│ ▸ Subscriptions list                                ○ t-0001  ABC-1472 [L] [N] │
│                                                                                │
│ Not in TASKS.md                                                              0 │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ 1-4 open  a all  esc close                                                     │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns, `f` on *Document select for summary* (three Figma frames; the
row wraps):

```text
┌────────────────────────────────────────────────────────────┐
│ Admin rebuild                           ◐ 2  ◇ 1  ○ 1  ✉ 0 │
│────────────────────────────────────────────────────────────│
│ In progress                                              3 │
│ ▾ Users page                                          [L4] │
│   └ ◐ t-0002 Members /admin/use…         Building overview │
│ ▾ Templates page                                   [L] [F] │
│   └ ◐ t-0003 Templates /templates            Writing tests │
│       :5181 frontend ●  :8011 api ●  :5441 pg ●            │
│›▾ Document select for summary                     [L] [F3] │
│   ├ Figma  1 598:48083  2 1138:88367  3 635:76529          │
│   │        d desktop app  a all                            │
│   └ ◇ t-0004 Summary select doc…              PR #2320 [G] │
│       review required · 2c · checks ✓                      │
│       :5174 frontend ●  :8002 api ○                        │
│                                                            │
│ On hold until Monday 2026-10-05                          1 │
│ ▸ Subscriptions list                      ○ t-0001 [L] [N] │
│                                                            │
│ Not in TASKS.md                                          0 │
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

- The chooser row has the link kind's colour as background tint, digits bold.
  Each `N label` pair is a click target. Labels are the bare ID or Figma
  `node-id`; where a link was found is not shown (no room), which is the
  trade against A's popup.
- When there are more links than fit on two lines, the row becomes a short
  indented list, one link per line.

## Empty project

80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Herdr Deck                                                     nothing running │
│────────────────────────────────────────────────────────────────────────────────│
│ Build herdr-deck v1 (milestones 1-7 in docs/PLAN.md): a Go + Charm status      │
│ pane that runs next to each herdr-projects coordinator …                       │
│                                                                                │
│ No tasks yet.                                                                  │
│   The tree grows from TASKS.md, which the coordinator writes when you give     │
│   it work. Threads it starts appear under their task, or under                 │
│   "Not in TASKS.md" until it lists them.                                       │
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
│ No tasks yet.                                              │
│   The tree grows from TASKS.md, which the coordinator      │
│   writes when you give it work. Threads it starts          │
│   appear under their task, or under "Not in TASKS.md"      │
│   until it lists them.                                     │
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
│ r reload  ? help                                           │
└────────────────────────────────────────────────────────────┘
```

- An empty tree has nothing to explain itself with, so the empty state says
  where the tree comes from. Dim text; goal dim italic.

## A data source is missing

80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Admin rebuild                                               ◐ 2  ◇ 1  ○ 1  ✉ 0 │
│ ! ticker stale 2h · ! no herdr socket · ! no dev.json in webshop               │
│────────────────────────────────────────────────────────────────────────────────│
│ In progress                                                        as of 14:23 │
│ ▾ Users page                                                     ABC-1246 [L4] │
│   └ ◐ t-0002 Members /admin/users                     Building overview · ~60% │
│ ▾ Templates page                                              ABC-1051 [L] [F] │
│   └ ◐ t-0003 Templates /templates                         Writing tests · ~70% │
│       :5291 port token ●                                                       │
│ ▾ Document select for summary                                ABC-1191 [L] [F3] │
│   └ ◇ t-0004 Summary select documents                             PR #2320 [G] │
│       review state unknown (no ticker)                                         │
│       :5174 port token ○                                                       │
│                                                                                │
│ On hold until Monday 2026-10-05                                              1 │
│›▸ Subscriptions list                                ○ t-0001  ABC-1472 [L] [N] │
│                                                                                │
│ Not in TASKS.md                                                              0 │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ space fold  l f n g link  o local  e edit  ? help                              │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns:

```text
┌────────────────────────────────────────────────────────────┐
│ Admin rebuild                           ◐ 2  ◇ 1  ○ 1  ✉ 0 │
│ ! ticker 2h · ! no herdr · ! no dev.json                   │
│────────────────────────────────────────────────────────────│
│ In progress                                    as of 14:23 │
│ ▾ Users page                                          [L4] │
│   └ ◐ t-0002 Members /admin/use…         Building overview │
│ ▾ Templates page                                   [L] [F] │
│   └ ◐ t-0003 Templates /templates            Writing tests │
│       :5291 port token ●                                   │
│ ▾ Document select for summary                     [L] [F3] │
│   └ ◇ t-0004 Summary select doc…              PR #2320 [G] │
│       review state unknown                                 │
│       :5174 port token ○                                   │
│                                                            │
│ On hold until Monday 2026-10-05                          1 │
│›▸ Subscriptions list                      ○ t-0001 [L] [N] │
│                                                            │
│ Not in TASKS.md                                          0 │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│────────────────────────────────────────────────────────────│
│ space fold  l f n g o links  e edit  ? help                │
└────────────────────────────────────────────────────────────┘
```

- The source line sits under the header (yellow) rather than above the
  footer: in a tree the bottom is often scrolled. Same rules as A: stale
  statuses dim with `as of`, `↵` gone without a socket, `port token`
  fallback ports.

## Good and bad

| | |
|---|---|
| Scanning | Good for "how is task X going"; fair for "what needs me", which relies on the red bar and the edge markers instead of a fixed band. |
| Density | Good: no duplication. Folding lets 10+ tasks fit; unfolded, a task with a PR and servers takes four rows. |
| Narrow width | Good. The tree indent costs 4–7 columns, but the right column is short (badges only on task rows), so titles keep more room than in A. |
| Keys | More than A: `space` to fold, `!` to jump, `[`/`]` for lists. Link keys reach down the node, so the task row is enough for most links. |
| Mouse | Good: fold arrows, badges and inline chooser entries are click targets. |
| Fit with the data | Depends on TASKS.md naming threads. In the sample TASKS.md every task names its thread, so it works; a sloppy coordinator moves threads to *Not in TASKS.md*. A thread named by two tasks shows under both. |
| Build cost | Medium: needs a tree model, fold state and the thread ↔ task join. |
