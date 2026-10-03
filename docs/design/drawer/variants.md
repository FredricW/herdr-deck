# Drawer · variants

Small alternatives where the choice matters. Each frame here is only the
drawer at its normal height (the list's bottom rule plus eight lines), not
the whole pane; the list above it is as in the other files. The
[README](README.md#recommendation) says which one is recommended.

## Tab bar

**T1 · Tabs on a rule (used in every mockup).** The bar doubles as the
line between card and content. The active tab sits in a heavy bracket
`━┫ … ┣━` in the status colour, bold; the others are dim on a dim rule.
Readable without colour, which matters for a glance.

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│────────────────────────────────────────────────────────────────────────────────│
│ Users page                                                   ● needs you  ~95% │
│ t-0002 · Members /admin/users · pane w1Z:p1                         ▰▰▰▰▰▰▰▰▰▱ │
│━┫ Overview ┣━── Files 11 ──── Log 7 ─────────────────── ↓ Note · Dev · Thread ─│
│ ── Next ──                                                                     │
│ → Approve phase 1 (ABC-1256 overview)                                          │
│ → Say whether to start ABC-1257 and ABC-1250                                   │
│ ── Links ──                                                                    │
│ [1 ABC-1246 done] [2 ABC-1256 in progress] [3 ABC-1257 todo] [4 ABC-1250]      │
└────────────────────────────────────────────────────────────────────────────────┘
```

**T2 · Pill tabs.** Plain words; the active one on the selection's grey
background, bold, with half-block ends (`▐ ▌`) drawn in the same grey so
it looks rounded. Lighter, but the bar no longer separates card and
content, and without colour the active tab is only the one with ends.

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│────────────────────────────────────────────────────────────────────────────────│
│ Users page                                                   ● needs you  ~95% │
│ t-0002 · Members /admin/users · pane w1Z:p1                         ▰▰▰▰▰▰▰▰▰▱ │
│▐ Overview ▌ Files 11   Log 7                             ↓ Note · Dev · Thread │
│ ── Next ──                                                                     │
│ → Approve phase 1 (ABC-1256 overview)                                          │
│ → Say whether to start ABC-1257 and ABC-1250                                   │
│ ── Links ──                                                                    │
│ [1 ABC-1246 done] [2 ABC-1256 in progress] [3 ABC-1257 todo] [4 ABC-1250]      │
└────────────────────────────────────────────────────────────────────────────────┘
```

**T3 · Card in the rule.** The title and pill move onto the list's bottom
rule, as today's drawer title does. One more line for content (six instead
of five), at the cost of a card that reads less like a card.

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│─ Users page ─────────────────────────────────────────────── ● needs you  ~95% ─│
│ t-0002 · Members /admin/users · pane w1Z:p1                         ▰▰▰▰▰▰▰▰▰▱ │
│━┫ Overview ┣━── Files 11 ──── Log 7 ────────────────────────── ↓ Dev · Thread ─│
│ ── Next ──                                                                     │
│ → Approve phase 1 (ABC-1256 overview)                                          │
│ → Say whether to start ABC-1257 and ABC-1250                                   │
│ ── Note ──                                                                     │
│ Phase 1 = layout + overview (ABC-1256); report on 1257/1250 before starting    │
│ them.                                                                          │
└────────────────────────────────────────────────────────────────────────────────┘
```

With T3 the *Links* section still sits below the fold here, because the
extra line went to *Note*: a 28-row pane is tight either way.

## Status pill

**P1 · Glyph and word (used).** `● needs you`, `◐ working`, `◇ review`,
`○ idle`, in the status colour, the same glyphs as the list.

**P2 · Filled pill.** The word on a background of the status colour, black
text: ` needs you ` on red, ` working ` on cyan. Louder; on a light
terminal theme the cyan and yellow backgrounds are hard to read, which is
why the deck has so far used coloured text only.

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

**L1 · Chips (used).** Each link one bracketed chip: digit bold, label in
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

**L2 · A line per kind.** Today's layout inside the section: dim kind
label, numbered links after it. One line per kind present, so it costs a
line more when there are several kinds, but the kind is always named and
the numbers line up the same way every time.

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

**L3 · A line per link.** Easiest to scan and to click, but four links take
four of the five content lines at the normal height.

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ ── Links ──                                                                    │
│ 1  ABC-1191   Linear · in review                                               │
│ 2  598-48083  Figma · Document-select                                          │
│ 3  1138-88367 Figma · Document-select                                          │
│ 4  635-76529  Figma · Document-select                                          │
└────────────────────────────────────────────────────────────────────────────────┘
```
