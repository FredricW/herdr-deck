# Drawer redesign: header card and tabs

Static mockups for the next drawer, the detail view under the list in
[design D](../d-list-detail.md). Nothing here is code. The user chose the
direction (2026-10-03): a **header card** on top, then **tabs**:
**Overview · Files · Log**. These files work it out in detail so the user
can pick the variants before anything is built.

| File | What it shows |
|---|---|
| [overview.md](overview.md) | The Overview tab: a thread that needs you, a working thread with dev servers, a thread in review with failing checks, a task without a thread, an inbox item; normal and full height |
| [files.md](files.md) | The Files tab as a diffstat: list and tree views, a thread with many files, the drawer cursor |
| [log.md](log.md) | The Log tab: the thread's timeline, what herdr-projects records for it, and what it does not |
| [variants.md](variants.md) | Tab bar styles, status pill styles, links as chips or lines |

Every state is drawn at 80 and 60 columns, in the 28-row frame the earlier
design docs use, where the drawer has its normal height (the list's bottom
rule plus eight lines), and again at full height (`z`). A real pane is
usually taller; the extra rows go to the list and the drawer as today. The
sample data is the made-up `admin-rebuild` project of the
[design README](../README.md#sample-data), plus a few invented events,
files and check names. The box around a frame is not part of the pane.

## What moves where

Every line the drawer shows today, and where it goes:

| Today | New place |
|---|---|
| title rule (`─ Users page ─`) | card, line 1 |
| `Thread` id, title, pane, agent state | card, line 2 (id, thread title, pane); the agent's state in *Thread* |
| `Status` state line and activity | card: status pill and percent with a bar; the activity on line 2 |
| `Owner` | card, line 2 |
| `Next` | Overview, *Next* |
| `Note` | Overview, *Note* |
| `Linear` (with states), `Figma`, `Notion` | Overview, *Links*, as chips |
| `GitHub` (PR link, review, comments, `✕ N failing`) | Overview, *PR*, with check names and commenters; the card's right edge when there is no percent |
| `Open` (localhost links with dots) | Overview, *Dev*, as chips under the servers |
| `Dev` (servers with dots, `starting…`) | Overview, *Dev* |
| `Report` | Overview, *Thread*; report events in Log; `r` as today |
| `Branch` | Overview, *Thread* |
| `Log` (the dev `up` command's log file) | Overview, *Thread* (`log`) |
| `Files` | the Files tab |
| inbox item: `Kind`, `Subject`, `Summary`, `File`, links | card (summary, kind, time, file), Overview *Subject* and *Links*; Files and Log are the subject thread's |
| list heading / project: `Note`, `Folded`, `Goal`, `Repos`, `Folder` | unchanged: a card with the list's or project's name and no tabs, then the same lines |
| `r` report, `?` help, `!` sources, `s` settings, `w` What's new | unchanged full views with today's title rule and no card or tabs; `esc` returns to the tab you were on. The report view keeps the thread's card and shows `Report · esc returns` where the tab bar was |

## Anatomy

```text
──────────────────────────────────────────────────  the list's bottom rule
 Users page                     ● needs you  ~95%    card, line 1: title · pill · percent
 t-0002 · Members /admin/users       ▰▰▰▰▰▰▰▰▰▱    card, line 2 (dim) · progress bar
━┫ Overview ┣━── Files 11 ──── Log 7 ── ↓ 4 more ─  tab bar · counts · what is below
 ── Next ──                                          the tab's content: five lines
 → Approve phase 1 (ABC-1256 overview)               at the normal height in a
 …                                                   28-row pane, 21 at full height
```

- **Card.** Line 1: the row's title, bold; at the right the status pill
  (`● needs you`, `◐ working`, `◇ review`, `↻ landing`, `○ idle`, `○ no
  thread`, `✓ done`, `✉ inbox`) and the percent, both in the status colour.
  Line 2, dim: thread id, the thread's own title when the row is a task
  (dropped at 60 columns), the activity while working, the pane (dropped at
  60); at the right a ten-cell bar `▰▱` in the status colour, or, with no
  percent, the PR (`#2320 ✕ 2 failing`). Several threads on one task: the
  card shows the most pressing one, as the list does, and *Thread* lists
  them all.
- **Tab bar.** `Files N` counts changed files (`Files …` until git has
  answered once), `Log N` counts events. A tab with nothing behind it (no
  thread, a resolved thread's files) is dim, has no count and is skipped.
  At the right, dim, what is below the drawer's end: section names on
  Overview at 80 columns, else `↓ N more`.
- **Overview's sections** are `── Name ──` rules, dim with the name bold,
  in a fixed order: *Next*, *PR*, *Note*, *Links*, *Dev*, *Thread*. Empty
  ones are left out; blank lines go between them only when all of them fit.
- **Which tab shows.** The tab stays when you move through the list, so
  reading Files and pressing `j` shows the next thread's files. Two
  exceptions: when the cursor jumps to a needs-you row by itself, the drawer
  shows Overview, since *Next* is why it jumped; and a row without a thread
  shows Overview until you move back to one.

## Colours

The deck's palette (named ANSI colours, [design README](../README.md#colours)),
applied to the new parts:

| Part | Style |
|---|---|
| Card title | bold |
| Card second line | dim |
| Pill, percent and bar | the status colour: needs you red bold, working cyan, review magenta, landing green, idle / no thread / done dim, inbox yellow; empty bar cells `▱` dim |
| Active tab | bold, its `━┫ ┣━` bracket in the status colour |
| Other tabs, the rule, the `↓` hint | dim |
| Section rules | dim rule, bold name; *Next* red while the thread needs you |
| Link chips | brackets dim, digit bold, Linear ID blue, Figma magenta, Notion and GitHub default; Linear state in its state colour (as today); a closed issue's chip dim |
| PR section | `#2320` magenta; *review required* magenta, *approved* green, *changes requested* red; `✕ N failing` red, `✓ no failing checks` green |
| Dev | `●` green listening, `○` dim, `~` fallback port dim, `starting…` yellow |
| Files: letters | `A` green, `M` yellow, `D` red, `R` cyan, `?` faint green, a binary file's letter faint (PR #23) |
| Files: counts and bar | `+N` green, `-M` red; bar cells green for added, red for deleted, `▁` dim; folder rows dim |
| Log: glyphs | `●` red (waiting on you, blocked), `◇` magenta (PR opened or updated), `✕` red (checks failing), `✓` green (merged), `✓` dim (resolved), `≡` plain (report), `▶` `+` `»` dim; `✉` yellow on an unhandled item |
| Log: age, clock, day rules, rail `│` | dim |
| Drawer cursor | `▸` and the selection's grey background (237, or 254 on a light terminal), as in the list |

New glyphs, all one column wide: `▰` `▱` (progress), `━` `┫` `┣` (active
tab), `▇` `▁` (diffstat), `≡` `▶` `»` (log), `↓` (more below).

## Keys, focus and mouse

The focus is on the list, as today. `j`/`k` move the list, and the drawer
follows. `tab` moves the focus into the drawer: the list's `▸` goes dim and
a cursor appears in the drawer. Inside the drawer, `tab` and `shift+tab`
switch tabs, `j`/`k` move the drawer cursor, `↵` acts on the item under it,
and `esc` gives the focus back to the list. A click in the drawer gives it
the focus too.

| Key | Overview | Files | Log |
|---|---|---|---|
| `[` `]` | previous / next tab, from the list or the drawer | same | same |
| `1`–`9` | open the numbered link | open the numbered file's diff | open the numbered link (PR events carry the PR's chip) |
| `l` `f` `n` `g` `o` | first link of that kind, as today, on every tab (the chooser for several highlights the chips) | same | same |
| `d` | switch to Files (`d 3` still opens file 3; `d d` the whole diff) | `d` again or `a`: the whole diff | as on Overview |
| `t` | — | list ↔ tree | — |
| `↵` (list focus) | the thread's pane | same | same |
| `↵` (drawer focus) | open the link under the cursor | open the file under the cursor, numbered or not | the event: a report shows it, a PR event opens the PR, the rest focus the pane |
| `r` `e` `u` `z` `pgup` `pgdn` | as today | same | same |

`d` used to open a chooser over the Files section; with Files a tab of its
own the chooser is the tab, so the old key sequences keep working.

**Mouse.** A click on a tab switches to it; a click on a chip, file or
event opens it, as a click on a drawer link does today; a click on the
total line opens the whole diff; a click on the card does nothing (the row
is already selected). The wheel scrolls the tab under the pointer.

## Data the deck does not read yet

Most of the design uses today's snapshot. These need new reads, all
read-only:

- **Percent** as a number (`percent` in `thread list --json`, may be
  `null`); today only the state line's `~95%` text is kept.
- **Thread timestamps** for Log: `created`, `launched_at`,
  `brief_seen_at`, `last_state_change`, `last_report_change`, and
  `resolved_reason`.
- **Inbox history**: items in `inbox/done/` as well as `inbox/`, matched by
  `subject`, with their `event` field. The folder grows (about 200 items in
  a busy project after two days), so read only files whose name holds the
  thread id and cache them by name.
- **ticker.json**: `commenters[]` and `last_pr_check`, next to the fields
  the deck reads now.

## Recommendation

- **T1, tabs on a rule.** It separates card and content, and the active tab
  is visible without colour. (T2 and T3 in [variants.md](variants.md).)
- **P1, glyph-and-word pills.** Same glyphs and colours as the list; P2's
  filled pills fight light themes.
- **L1, chips**, with the kind word only on a run's first chip. Four links
  on one line at 80 columns leaves room for *Next* in a short drawer. If the
  chips feel noisy in use, L2 (a line per kind) is the fallback and costs a
  line per extra kind.
- **The PR's chip in the *PR* section only**, so no link shows twice.
- **Keep the 28-row budget honest:** at the normal height the tab content is
  five lines. Either accept that (the `↓` hint names what is below, and `z`
  is one key), or give the drawer half of the pane instead of 40 % once it
  has tabs (two more lines at 28 rows). The second is a one-line change.

## Open questions

1. **Drawer height.** Keep 40 % (five content lines at 28 rows) or raise it
   to 50 % now that the drawer has a card and a tab bar?
2. **Section order.** Proposed: *Next*, *PR*, *Note*, *Links*, *Dev*,
   *Thread*, so what asks something of you comes first. The task named
   *PR* last; which do you prefer?
3. **Tab memory.** Proposed: the tab stays as you move through the list
   (except the needs-you jump). Or should every row open on Overview?
4. **`tab` for focus.** `tab` both enters the drawer and, inside it,
   switches tabs, while `[`/`]` switch tabs from anywhere. Is that one key
   too many meanings, or should `tab` only switch tabs and a click or `↵`
   on the drawer give it the focus?
5. **Commits in Log.** The branch's commits (`git log <merge-base>..`) are
   real, dated events, but they come from git, not herdr-projects. Add them
   to Log (dim, `·` glyph), or keep Log to what herdr-projects records?
6. **Files order.** Path order, as git prints it (shown), or largest change
   first, which the bars make easy to read?
