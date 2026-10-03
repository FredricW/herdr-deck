# Deck pane: design directions

Static mockups for the herdr-deck pane, the narrow split (60–80 columns, full
height) next to a herdr-projects coordinator. Nothing here is code. PLAN.md's
first draft was one possible direction; these four were the options to choose
from.

**Chosen (2026-10-02): D, compact list + detail drawer, with B's folding for
long lists such as Backlog.** PLAN.md's [UI section](../PLAN.md#ui) describes it.

| | Direction | In one line |
|---|---|---|
| A | [Stacked sections](a-sections.md) | The PLAN draft worked out: Tasks, Threads, Inbox stacked on one page, a red band for what needs you. |
| B | [Task tree](b-task-tree.md) | Tasks are the spine; threads, PRs and servers fold under the task they belong to. |
| C | [Tabs, with a "Now" feed](c-tabs.md) | One page at a time; the default page sorts threads by how much they want you. |
| D | [Compact list + detail drawer](d-list-detail.md) | One dense row per piece of work, and a drawer with everything about the selected row. |

The multi-project view (milestone 8) has its own directions, E–H, in
[multi-project/](multi-project/README.md). The drawer's redesign, a header card
with Overview, Files and Log tabs (chosen 2026-10-03), is in
[drawer/](drawer/README.md).

Each direction file shows six states, each at 80 and at 60 columns: normal
use, a thread that needs you, unhandled inbox items, the chooser for a row
with several links of one kind, an empty project, and missing data sources.
Each ends with what it is good and bad at. The [comparison and
recommendation](#comparison) are at the end of this file.

## Shared conventions

These hold for all four directions unless a direction says otherwise.

### Reading the mockups

- Each mockup is the pane at exactly 60 or 80 columns. The box drawn around
  it is not part of the pane; it adds one column on each side.
- Mockups are 28 rows tall; a real pane is usually taller, and the extra rows
  stay empty as shown.
- Plain text cannot show colour, weight or backgrounds, so each mockup is
  followed by notes on those. The selected row is always marked with `▸`
  as well as a grey background (in B with `›`, since `▸`/`▾` mean folded there).
- All glyphs used are one column wide in a Western terminal (Lip Gloss and
  go-runewidth count East Asian "ambiguous" characters as one column by
  default). No emoji.

### Colours

Named ANSI colours, so the user's terminal theme decides the actual shades.

| Meaning | Style |
|---|---|
| Needs you (thread group *Waiting on you*) | red, bold; bands and bars red background or red `┃` |
| Unhandled inbox item | yellow |
| Working | cyan |
| Ready for review, PR numbers | magenta |
| Landing; checks pass; port listening | green |
| Idle, resolved, metadata, empty states | dim (faint) |
| Missing or stale source | yellow `!` |
| Selected row | grey background (256-colour 237; 254 on a light terminal), text keeps its colours; also the link chooser's line |
| Link kinds (badges, chooser borders) | Linear blue, Figma magenta, Notion default, GitHub default |

### Glyphs

| Glyph | Meaning | Source |
|---|---|---|
| `●` red | thread needs you | herdr-projects group `Waiting on you` (`group` / `last_group`), or live agent state from herdr |
| `◐` cyan | working | group `Working`; text from `activity`, `percent` |
| `◇` magenta | ready for review | group `Ready for review`; `pr`, `pr_state`, `pr_review` |
| `↻` green | landing | group `Landing` |
| `○` dim | idle | group `Idle` |
| `✓` / `✕` / `⋯` | checks pass / fail / pending | ticker `prs[thread id].failing_checks[]` |
| `✉` yellow | unhandled inbox item | `inbox/*.md` front matter |
| `●` green / `○` dim after a port | port listening / not | TCP connect to 127.0.0.1, 400 ms |
| `[L]` `[F]` `[N]` `[G]` | Linear, Figma, Notion, GitHub PR links; `[L4]` = four Linear links | links scraped from TASKS.md, briefs, reports, `pr` |
| `!` yellow | a source is missing or stale | — |

### Keys

As in PLAN.md: `j`/`k` move, `↵` focuses the thread's herdr pane, `l` `f` `n`
`g` `o` open Linear / Figma / Notion / PR / localhost links, `e` opens the
worktree in the editor, `?` help, mouse click selects and wheel scrolls. Each
direction adds a few keys of its own and says so.

### Sample data

All mockups use one made-up project, `admin-rebuild`, shaped like a
herdr-projects folder on 2026-10-02:

- **Lists.** *In progress*: Users page (ABC-1246; notes name ABC-1256,
  ABC-1257, ABC-1250) → t-0002; Templates page (ABC-1051, a Figma link in
  t-0003's brief) → t-0003; Document select for summary (ABC-1191; three
  Figma frames in t-0004's report) → t-0004. *On hold until Monday
  2026-10-05*: Subscriptions list (ABC-1472; a Notion link in t-0001's
  report) → t-0001.
- **Threads.** t-0001 *Subscriptions /admin/plans* idle; t-0002
  *Members /admin/users* working, then needs you; t-0003 *Templates
  /templates* working with dev servers on :5181/:8011/:5441; t-0004
  *Summary select documents* ready for review with PR #2320 and servers
  on :5174 (up) and :8002 (down).
- **Inbox.** `thread-state` items, as herdr-projects writes them.

All of it is invented: titles, IDs, lists, notes, states, the PR number, the
`next[]` lines and the ports.

### What the deck shows, and what it does not

Only data that PLAN.md lists as available: thread list fields (`group`,
`rank`, `state_line`, `activity`, `percent`, `pr*`, `next[]`, `pane_id`,
`branch`, `worktree_path`), ticker PR data (`review_decision`,
`failing_checks`, `comment_count`, `commenters`), TASKS.md lists, tasks and
notes, inbox front matter, PROJECT.md `name`/`goal`/`repos`, scraped links,
dev.json ports and links, the herdr `port` token, and herdr's live agent
state.

Not shown, because v1 has no source for it: Linear issue status or titles
(milestone 8), PR titles, CI job names, token or cost figures. Figma links
show as file name plus `node-id`, since that is all the URL carries. The deck
never marks inbox items handled; that would be a write to herdr-projects.

## Comparison

| | A Sections | B Task tree | C Tabs | D List + drawer |
|---|---|---|---|---|
| Glance: does anything need me? | Very good (band) | Good (red bar, edge markers) | Good on *Now*, fair elsewhere (strip) | Very good (pinned group) |
| Glance: whole project | Good | Good when folded | Poor (one page) | Very good |
| Rows for this sample project | ~19 | ~15 unfolded, ~8 folded | ~15 on *Now* alone | 7 + drawer |
| Duplication | Each thread twice | None | Across pages | None |
| At 60 columns | Fair: IDs, percents go | Good | Best | Fair: columns fold |
| Keys to learn | Fewest | `space` `!` `[` `]` extra | `1`–`5` page switches | Digits; no chooser mode |
| Chooser | Popup over the list | Inline row | Bottom sheet | The drawer itself |
| Mouse | Good | Good | Good (big tabs) | Good |
| Scales to 10+ threads / long backlog | Poorly (sections scroll) | Well (folding) | Well (per page) | Well (one row each) |
| Build cost after M1 | Lowest | Medium | Medium | Medium |

## Recommendation

**D, compact list + detail drawer**, for v1.

- The pane sits beside a coordinator all day, so it is read in glances. D
  gives one row per piece of work, with the needs-you rows pinned on top,
  and keeps the depth (notes, `next[]`, every link, servers) in the drawer
  for the row you are on.
- It removes the chooser as a separate mode: the drawer numbers every link,
  so a row with four Linear issues costs one digit, and a mouse user clicks
  the one they want.
- It scales: the sample TASKS.md already has six more backlog tasks,
  which are six more rows in D and a scrolling section in A.

Take two things from the others: B's folding for lists like *Backlog* (start
folded, one row), and C's sources page, which D already uses as the drawer's
*Sources* view.

**Choose A instead** if v1 should ship with the least work: it is closest to
the M1 skeleton, and D can follow later because both read the same data.
**Choose B** if you think of the project task by task and the coordinator
keeps TASKS.md naming threads reliably. **C** is the weakest fit for an
always-visible side pane, because a glance shows one page out of five.

Whatever is picked, PLAN.md's UI section should be updated to match before
milestone 4 (live UI).
