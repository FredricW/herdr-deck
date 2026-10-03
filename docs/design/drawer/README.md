# Drawer redesign: header card and tabs

Static mockups for the next drawer, the detail view under the list in
[design D](../d-list-detail.md). Nothing here is code. The user chose the
direction on 2026-10-03: a **header card** on top, then **tabs**:
**Overview · Files · Log**. The same day they settled the variants and the
open questions; the [decisions](#decisions) are below, and
[PLAN.md's UI section](../../PLAN.md#ui) describes the result for the build.

| File | What it shows |
|---|---|
| [overview.md](overview.md) | The Overview tab: a thread that needs you, a working thread with dev servers, a thread in review with failing checks, a task without a thread, an inbox item; normal and full height |
| [files.md](files.md) | The Files tab as a diffstat: list and tree views, a thread with many files, the drawer cursor |
| [log.md](log.md) | The Log tab: the thread's timeline, what herdr-projects records for it, and what it does not |
| [variants.md](variants.md) | The tab bar (chosen style, and separated versus touching tabs), status pill styles, links as chips or lines |

Every state is drawn at 80 and 60 columns, in the 28-row frame the earlier
design docs use, where the drawer has its normal height (half of the pane:
the list's bottom rule plus eleven lines), and again at full height (`z`).
A real pane is usually taller; the extra rows are split between the list
and the drawer. The sample data is the made-up `admin-rebuild` project of
the [design README](../README.md#sample-data), plus a few invented events,
files and check names. The box around a frame is not part of the pane.
Plain text cannot show the tab bar's backgrounds, so the note beside each
frame names the active tab.

## Decisions

Made by the user on 2026-10-03.

- **Tab bar:** plain text labels on background colours, one space of
  padding on each side (` Overview `, ` Files 11 `), no rule and no
  brackets, counts in the labels, and one space between tabs (touching
  tabs merge the inactive ones into one grey strip). The active tab is bold
  near-black text (256-colour 234; 16 on a light terminal) on **blue**,
  the inactive tabs plain text on dark grey (256-colour
  237, 254 on a light terminal), a tab without data dim on that grey. Blue
  is the one palette colour no status uses, so the active tab never reads
  as a status. Comparison in [variants.md](variants.md#tab-bar). This
  replaces the earlier proposal of tabs on a rule (T1). The text was
  bright white at first; on 2026-10-03 the user switched it to dark text,
  since white is unreadable on the pastel blue of themes such as
  Catppuccin Mocha.
- **Status pill:** P1, the status glyph plus a word (`● needs you`).
- **Links:** L1, chips (`[1 ABC-1246 done]`), with the kind word only on
  the first chip of a run of one kind.
- **The PR's chip** lives in the *PR* section only, never also in *Links*.
- The six open questions, answered as proposed:
  1. **Drawer height:** the drawer takes **50 %** of the pane (was 40 %).
     In a 28-row pane that leaves eight lines for the tab's content.
  2. **Section order:** *Next*, *PR*, *Note*, *Links*, *Dev*, *Thread*.
  3. **Tab memory:** the tab stays as you move through the list; the
     needs-you jump shows Overview, and a row without a thread shows
     Overview until the cursor is back on a thread.
  4. **Focus:** `tab` moves the focus into the drawer, and inside it `tab` /
     `shift+tab` switch tabs; `[`/`]` switch tabs from anywhere; a click in
     the drawer focuses it; `esc` returns to the list.
  5. **Log** keeps to what herdr-projects records: no git commits.
  6. **Files** stay in path order, as git prints them.

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
 Overview   Files 11   Log 7           ↓ 7 more    tabs on blue / grey · what is below
 ── Next ──                                          the tab's content: eight lines
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
- **Tab bar.** ` Overview `, ` Files N `, ` Log N ` on their backgrounds,
  one space apart. `Files N` counts changed files (`Files …` until git has
  answered once), `Log N` counts events. A tab with nothing behind it (no
  thread, a resolved thread's files) is dim, has no count and is skipped.
  At the right, dim, what is below the drawer's end: section names on
  Overview at 80 columns, else `↓ N more` lines.
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
| Active tab | blue background, bold near-black text (256-colour 234; 16 on a light terminal) |
| Inactive tabs | dark grey background (256-colour 237; 254 on a light terminal), plain text; a tab without data dim text |
| The `↓` hint | dim |
| Section rules | dim rule, bold name; *Next* red while the thread needs you |
| Link chips | brackets dim, digit bold, Linear ID blue, Figma magenta, Notion and GitHub default; Linear state in its state colour (as today); a closed issue's chip dim |
| PR section | `#2320` magenta; *review required* magenta, *approved* green, *changes requested* red; `✕ N failing` red, `✓ no failing checks` green |
| Dev | `●` green listening, `○` dim, `~` fallback port dim, `starting…` yellow |
| Files: letters | `A` green, `M` yellow, `D` red, `R` cyan, `?` faint green, a binary file's letter faint (PR #23) |
| Files: counts and bar | `+N` green, `-M` red; bar cells green for added, red for deleted, `▁` dim; folder rows dim |
| Log: glyphs | `●` red (waiting on you, blocked), `◇` magenta (PR opened or updated), `✕` red (checks failing), `✓` green (merged), `✓` dim (resolved), `≡` plain (report), `▶` `+` `»` dim; `✉` yellow on an unhandled item |
| Log: age, clock, day rules, rail `│` | dim |
| Drawer cursor | `▸` and the selection's grey background (237, or 254 on a light terminal), as in the list |

New glyphs, all one column wide: `▰` `▱` (progress), `▇` `▁` (diffstat),
`≡` `▶` `»` (log), `↓` (more below).

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
| `d` | focus Files (`d 3` still opens file 3; `d d` the whole diff) | `d` again or `a`: the whole diff | as on Overview |
| `v` | — | the diff preview on or off (built after this design; see PLAN.md) | — |
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
