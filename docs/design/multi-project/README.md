# Multi-project view: design directions

Static mockups for milestone 8's multi-project view: one deck that shows all
of the user's herdr-projects projects at once. Nothing here is code. Today's
deck (design D, [../d-list-detail.md](../d-list-detail.md)) shows one
project beside its coordinator; these four directions are the options for
going across projects. The letters continue after the single-project
directions A–D, so "D" always means today's deck.

| | Direction | In one line |
|---|---|---|
| E | [Projects as groups in D](e-project-groups.md) | Today's list with projects as foldable groups, and one *Needs you* group across all of them. |
| F | [Project switcher](f-switcher.md) | Today's deck for one project, with a strip of project tabs (and their counts) on top. |
| G | [Overview, drill into the deck](g-overview.md) | A home page with one row per project; `↵` opens that project's deck as it is today. |
| H | [Needs-you inbox across everything](h-inbox-first.md) | A triage feed of what wants you in any project, sorted by urgency; quiet projects are one line. |

Each direction file shows four situations at 80 and 60 columns: normal use
with three active projects, something needing you in a project you are not
looking at, an empty and an archived project, and missing data sources.
E, G and H also have a 120-column variant for a herdr tab of their own. Each
file ends with what the direction is good and bad at. The
[comparison](#comparison) and [recommendation](#recommendation) are at the
end of this file.

## The three questions, per direction

| | Where it opens | Per-coordinator deck | Needs you and inbox across projects | Links |
|---|---|---|---|---|
| E | Next to coordinators as today (that project open, others folded), plus its own herdr tab from a key | Replaces it | One pinned *Needs you* group above all projects, rows tagged with the project | Drawer, as in D |
| F | Next to coordinators as today; optionally a popup from a key | Replaces it (same deck plus a strip) | Per project, as today; other projects only as red/yellow tab badges and a one-row band | Drawer, as in D, for the shown project |
| G | Its own herdr tab (or popup) from a key; `backspace` from any deck goes up to it | Complements it; its drill-down *is* today's deck | Pinned above the project rows, with D's drawer for the thread | Overview: PRs and localhost; everything else one level down |
| H | A herdr popup (or tab) from a key; never auto-opens | Complements it; nothing shared on screen | The whole view: sorted feed of needs, inbox, review, landing | Drawer, as in D, for the row |

## Shared conventions

Everything in the single-project [conventions](../README.md#shared-conventions)
holds: colours, glyphs, keys, the `▸` and grey background for the selected
row, D's drawer and its numbered links. Additions:

- **Frames** are 28 rows tall, as in the single-project mockups (E's
  120-column frame is 32). The box around a frame is not part of the pane.
- **A blank line separates list groups**, as in today's deck; in E it also
  separates projects.
- **Project names** come from PROJECT.md's `name`. Where space is short a
  project is shown by the first word of its name (`Billing`), longer when two
  projects would share it.
- **Project headings and rows** use the project's counts in the usual
  colours: red `● N` needs you, cyan `◐`, magenta `◇`, green `↻`, dim `○`,
  yellow `✉`, yellow `!` for a missing source.
- **Paused** projects are listed but marked `paused` (the herdr-projects
  ticker skips them, so their state can be old). **Archived** projects are
  hidden behind a folded *Archived* heading or an `A` key, as `herdr-projects
  list` hides them without `--all`, and never count in totals.
- **`↵` on a thread** focuses its pane even when it lives in another
  workspace. `pane.focus` takes any pane id; whether herdr 0.9.3 also
  switches the visible workspace has to be checked before building any of
  these.

### Sample data

Five made-up projects under one projects root, as on 2026-10-03:

| Slug | Name | Status | Threads | Inbox |
|---|---|---|---|---|
| `admin-rebuild` | Admin rebuild | active | The single-project sample: t-0002 and t-0003 working, t-0004 ready for review (#2320, checks ✓), t-0001 idle. | 0 |
| `billing-export` | Billing export | active | t-0003 *CSV export job* working, later waiting on you; t-0002 *Invoice PDF fonts* ready for review (#418, one failing check); t-0004 *Ledger totals fix* landing (#415, checks pending). Backlog of 3. | 0, later 1 |
| `docs-site` | Docs site | active | t-0001 *Search page* idle; one resolved. | 1 |
| `search-spike` | Search spike | paused | none; TASKS.md has no tasks. | 0 |
| `mobile-onboarding` | Mobile onboarding | archived | six resolved. | 0 |

All of it is invented: names, slugs, repos (`acme/webshop`, `acme/ledger`,
`acme/docs`), ticket IDs (`ABC-`, `BIL-`), PR numbers, `next[]` lines,
check names and ports.

The "missing" state is the same in every direction: the herdr socket is
gone, *Billing export* has no `.state/ticker.json`, and `herdr-projects
thread list docs-site --json` fails because that project's PROJECT.md does
not parse.

## What herdr-projects gives across projects

Only what PLAN.md's [data sources](../../PLAN.md#data-sources) list, read
for each folder under the projects root, plus a few things that only matter
across projects:

| What | Where | Notes |
|---|---|---|
| The projects | `herdr-projects --root <root> list [--all]` | One line per project: slug, status (`active` / `paused` / `archived`), and thread counts per group (`Waiting on you: 1, Ready for review: 2`). Text with tabs, not JSON. Archived only with `--all`. |
| Project status | `<slug>/.state/project.json` → `status` | The same status, per project, without running anything. |
| The coordinator | herdr pane token `hp_group = <slug>!0!<pane id>`; `<slug>/.state/coordinator.json` → `pane_id`, `workspace_id` | Lets `↵`/`c` focus a project's coordinator; its agent state comes from the herdr snapshot. |
| Needs you, total | `herdr-projects needs-you` | `projects: N need you`, and nothing when the ticker is not running. Useful as a cross-check, not as the source. |
| Last activity | thread `last_state_change` | Newest across the project's threads. |
| Per project | thread list, ticker.json, inbox, TASKS.md, briefs and reports | As today, once per project. |

Two sources are shared by all projects: the herdr socket (one herdr) and the
herdr-projects ticker (one process writes every project's status and
`ticker.json`). When either is gone, every project is affected at once, and
the mockups say so once rather than per project.

Cost: today's deck runs one `thread list --json` per refresh. Across five
projects that is five per refresh, plus reading five TASKS.md files. E reads
everything for every project; F and G only need full data for one project
and counts for the rest (`list`, inbox folder listings, ticker.json); H needs
every thread list but no TASKS.md parsing. fsnotify on the projects root
covers all projects' files with one watcher per folder.

## Open questions the choice brings

These hold for every direction; the user decides them along with the
direction.

1. **Linear workspace per project.** `linear_workspace` is one setting
   today. Projects for different organisations would need a per-project
   value, for example `[projects.<slug>] linear_workspace = "…"` in the
   config file.
2. **Starting things.** With no coordinator running, should `↵` on a project
   run `herdr-projects open <slug>`? The deck would write no files itself,
   but herdr-projects would, and it starts an agent. The mockups only say
   "no coordinator running". Resuming a paused project is a
   write (`herdr-projects resume`) and stays out.
3. **Order of projects.** The mockups keep `herdr-projects list` order (by
   slug) and pin needs on top instead of re-sorting rows, so rows do not jump.
   Sorting by last activity is the alternative.
4. **Auto-open.** Nothing new auto-opens in any direction; the
   multi-project view is opened by a key. A herdr notification for a new need
   elsewhere is possible in all of them.

## Comparison

| | E Groups in D | F Switcher | G Overview | H Inbox first |
|---|---|---|---|---|
| Glance: does anything need me, anywhere? | Very good (pinned group) | Fair (red tab, band) | Very good (pinned rows) | Best (it is the view) |
| Glance: what is each project doing? | Good, costs rows | Poor (one at a time) | Very good (one row each) | Poor (one *Quiet* line) |
| Next to a coordinator | Mixed: other projects take rows, cursor may jump away | Very good | Unchanged from today | Unchanged from today |
| 10+ projects | Fair | Fair (picker) | Very good | Very good |
| At 60 columns | As D | Good | Fair (PRs move to drawer) | Good |
| Best as | Split or tab | Split | Tab | Popup |
| Keys to learn | `[` `]` `A` | `[` `]` `p` `h` | `↵` down, `esc` up, `c` | `↵`, `q` |
| Data per refresh | Everything, every project | One project + counts | Counts + one project | Every thread list |
| Build cost | Low–medium | Lowest | Medium | Low–medium |
| Changes today's deck | Yes (it becomes this) | Yes (a strip) | No (adds `backspace`) | No |

## Recommendation

**G, overview rows that drill into today's deck**, opened from a key as a
herdr tab of its own, with two things taken from the others.

- It answers both cross-project questions on one page: the pinned rows say
  what needs you anywhere (with D's drawer, so the question is readable
  without leaving), and the table says what each project is doing, one row
  each, which scales to ten projects.
- It leaves the deck beside each coordinator alone. That deck is read in
  glances all day; E and F would make it show other projects' rows or tabs
  and let its cursor wander off to another project.
- Its drill-down is today's deck, so there is still one deck design to
  maintain, and the overview reads little: `herdr-projects list`, inbox
  folders, ticker.json and one herdr snapshot.

Take from the others:

- **From F, the band.** In the per-coordinator deck, a one-row red band
  when something needs you in *another* project (`● Billing export · CSV
  export job needs you · backspace`), so you learn about it without the
  overview open. It costs a row only while it is up.
- **From H, the order of the pinned rows**: needs you, then inbox, then
  failing checks, then the rest, and the yellow "may be incomplete" band when
  a global source is missing, since an empty pinned group otherwise reads as
  "all clear".

**Choose H instead** if what you want is triage only: a popup you open,
clear, and close, never a picture of each project. It is the cheapest way to
stop missing questions across projects. **Choose E** if you want one deck
everywhere and do not mind other projects taking rows beside each
coordinator. **F** is the least work and the least change, but it shows one
project at a time, so it adds the least.

Whatever is picked, PLAN.md's milestone 8 entry and its UI section should be
updated to match before building it.
