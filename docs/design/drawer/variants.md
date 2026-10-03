# Drawer · variants

The alternatives offered for the tab bar, the status pill and the links,
and which one the user chose on 2026-10-03 (see the
[README](README.md#decisions)). The frames here are excerpts of the drawer,
not the whole pane; the list above it is as in the other files.

## Tab bar

**Chosen: plain labels on background colours.** Each tab is its label with
one space of padding on each side (` Overview `, ` Files 11 `, ` Log 7 `),
no rule and no brackets; counts stay in the labels. The colours:

| Tab | Background | Text |
|---|---|---|
| active | blue (named ANSI blue, the terminal theme's shade) | bold bright white |
| inactive | dark grey, 256-colour 237 (254 on a light terminal), the selection's grey | plain |
| without data (no thread, a resolved thread's files) | the same grey | dim, no count |

Blue is the accent because it is the one palette colour no status uses:
red, cyan, magenta, green, yellow and dim all mean something in the list,
while blue only marks Linear IDs, as text. So the active tab looks the same
whichever row is selected, and never reads as a status. Bright white on
blue reads on both dark and light themes.

The label's text starts in column 2, where the rest of the drawer's text
starts, because the tab's own padding fills column 1.

**One space between tabs, or touching.** Plain text cannot show the
backgrounds, so each is drawn twice: as the pane shows its text, and as a
map of its backgrounds (`█` blue, `░` grey, blank the drawer's own
background).

Separated by one space (chosen):

```text
┌────────────────────────────────────────────────────────────┐
│ Users page                               ● needs you  ~95% │
│ t-0002 · Members /admin/users                   ▰▰▰▰▰▰▰▰▰▱ │
│ Overview   Files 11   Log 7                       ↓ 7 more │
│ ── Next ──                                                 │
│ → Approve phase 1 (ABC-1256 overview)                      │
└────────────────────────────────────────────────────────────┘
```

```text
 Overview   Files 11   Log 7
██████████ ░░░░░░░░░░ ░░░░░░░
```

Touching:

```text
┌────────────────────────────────────────────────────────────┐
│ Users page                               ● needs you  ~95% │
│ t-0002 · Members /admin/users                   ▰▰▰▰▰▰▰▰▰▱ │
│ Overview  Files 11  Log 7                         ↓ 7 more │
│ ── Next ──                                                 │
│ → Approve phase 1 (ABC-1256 overview)                      │
└────────────────────────────────────────────────────────────┘
```

```text
 Overview  Files 11  Log 7
██████████░░░░░░░░░░░░░░░░░
```

**Pick: one space between.** Touching tabs make the inactive ones one grey
strip (` Files 11  Log 7 ` above), so a glance cannot tell where *Files*
ends, and a click near the join is a guess. The one-column gap shows the
drawer's background between every pair, costs two columns, and the bar
still fits at 60 columns with the `↓` hint beside it.

*Earlier options, not taken:* T1, tabs on a rule (`━┫ Overview ┣━── Files
11 ──`); T2, pill tabs with half-block ends; T3, the card's title on the
list's bottom rule to win a line. The 50 % drawer height gives the room T3
was after.

## Status pill

**P1 · Glyph and word (chosen).** `● needs you`, `◐ working`, `◇ review`,
`○ idle`, in the status colour, the same glyphs as the list.

**P2 · Filled pill (not taken).** The word on a background of the status
colour, black text: ` needs you ` on red, ` working ` on cyan. Louder; on a
light terminal theme the cyan and yellow backgrounds are hard to read, and
next to background-coloured tabs it would be one filled shape too many.

```text
┌────────────────────────────────────────────────────────────┐
│ Users page                                NEEDS YOU   ~95% │
│ t-0002 · Members /admin/users                   ▰▰▰▰▰▰▰▰▰▱ │
└────────────────────────────────────────────────────────────┘
```

(Plain text cannot show the red background; P2 is drawn in capitals here
only to set it apart.)

## Links: chips or lines

The review row's links (one Linear issue, three Figma frames; the PR's own
chip is in the *PR* section).

**L1 · Chips (chosen).** Each link one bracketed chip: digit bold, label in
the kind's colour, state after it. A run of one kind drops the kind word
after its first chip. Densest; four links in one line at 80 columns.

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ ── Links ──                                                                    │
│ [1 ABC-1191 in review] [2 Figma 598-48083] [3 1138-88367] [4 635-76529]        │
└────────────────────────────────────────────────────────────────────────────────┘
```

```text
┌────────────────────────────────────────────────────────────┐
│ ── Links ──                                                │
│ [1 ABC-1191 in review] [2 Figma 598-48083]                 │
│ [3 1138-88367] [4 635-76529]                               │
└────────────────────────────────────────────────────────────┘
```

**L2 · A line per kind (not taken).** Today's layout inside the section:
dim kind label, numbered links after it. One line per kind present, so it
costs a line more when there are several kinds, but the kind is always
named. The fallback if chips turn out noisy in use.

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ ── Links ──                                                                    │
│ Linear  1 ABC-1191 in review                                                   │
│ Figma   2 598-48083   3 1138-88367   4 635-76529                               │
└────────────────────────────────────────────────────────────────────────────────┘
```

```text
┌────────────────────────────────────────────────────────────┐
│ ── Links ──                                                │
│ Linear  1 ABC-1191 in review                               │
│ Figma   2 598-48083  3 1138-88367  4 635-76529             │
└────────────────────────────────────────────────────────────┘
```

**L3 · A line per link (not taken).** Easiest to scan and to click, but
four links take half of the tab's eight lines.

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ ── Links ──                                                                    │
│ 1  ABC-1191   Linear · in review                                               │
│ 2  598-48083  Figma · Document-select                                          │
│ 3  1138-88367 Figma · Document-select                                          │
│ 4  635-76529  Figma · Document-select                                          │
└────────────────────────────────────────────────────────────────────────────────┘
```
