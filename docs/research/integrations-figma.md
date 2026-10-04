# Figma integrations — ideas

Written 2026-10-03 against herdr-deck v0.1.3, herdr 0.9.3 and Figma's REST
API, MCP server and help centre as documented that day. This is
exploration only: no code was changed, and no Figma API call was made for
this document. Limits, scopes and field names were checked that day, so
verify them before you rely on them. The examples use the made-up
`admin-rebuild` project of the [design README](../design/README.md), with
made-up file keys such as `aBcDeFgHiJkLmNoPqRsTuV`.

The structure follows the
[Linear and GitHub ideas](integrations-linear-github.md).

**Recommendation in one paragraph.** Build a small read-only Figma reader
(`internal/source/figma`) shaped like the Linear one. It uses a personal
access token from `$FIGMA_ACCESS_TOKEN` or a `figma_token_command`, keeps
the token in memory only, and calls only the cheap endpoints by default.
Use it first to give Figma chips **the file's real name, when it was last
edited and by whom**, and **"changed since the thread started"** (F1, F2).
Both come from one Tier 3 call per file. Then add the **open comment count**
(F3), from one Tier 2 call per file. Rendering a frame (F6) and reading
node names (F4) use Figma's Tier 1 endpoints. A View or Collab seat may
make as few as 20 Tier 1 calls a **month**, so those features run only on
demand, behind a setting, with a disk cache. When an image does render,
herdr 0.9.3 can show it inline through the Kitty graphics protocol, but
only on Ghostty or Kitty as the outer terminal. Everywhere else, use a
half-block preview or open the PNG. Only one idea writes to Figma: linking
the thread's PR to the frame as a **Dev Mode resource** (F8). It is useful,
and it belongs behind a confirm and a setting that is off by default.
Design specs for agents (F9) should come from Figma's own MCP server in the
thread's agent, not through the deck. The ranked shortlist is in
[§5](#5-shortlist), and the decisions for the user are in
[§6](#6-decisions-for-the-user).

Contents:

1. [What the deck has today](#1-what-the-deck-has-today)
2. [Auth, limits, images and how to fetch](#2-auth-limits-images-and-how-to-fetch)
3. [The catalogue](#3-the-catalogue)
4. [Ideas not to build](#4-ideas-not-to-build)
5. [Shortlist](#5-shortlist)
6. [Decisions for the user](#6-decisions-for-the-user)
7. [Sources](#7-sources)

---

## 1. What the deck has today

The deck has no Figma API client. Everything below works from the URL
alone.

- **Scraping** (`internal/source/tasks/links.go`). Figma links are found
  in TASKS.md, thread files and reports when they match
  `https://(www.)figma.com/(design|file|proto|board)/…`.
  - `figmaLabel` names a link by the name slug in its path, with dashes
    turned into spaces, plus its `node-id`: `Templates · node 598-48083`.
  - The drawer shows a link as a chip, `[2 Figma 598-48083]`. The kind word
    appears only on the first chip of a run of Figma links.
  - Links from `/slides/`, `/deck/`, `/make/`, `/site/` and `/buzz/` are
    not recognised as Figma today.
- **Desktop rewrite** (`internal/deck/figma.go`). `Link.DesktopURL`
  replaces `https://www.figma.com/` with `figma://` and keeps the path and
  query.
  - With `figma_desktop = true` (config, or `$HERDR_DECK_FIGMA_DESKTOP`),
    every Figma link the deck opens goes to the desktop app.
  - In the `f` chooser, `d` sends one link to the desktop app.
- **Keys.** `f` opens the first Figma link. With several, it highlights
  their chips and waits for one of these:
  - a digit, to open that link;
  - `a`, to open all of them;
  - `d`, to open one in the desktop app;
  - `f` again, to open the first;
  - `esc`, to cancel.

  `1`–`9` open numbered chips. Chips are numbered across kinds in this
  order: Linear, Figma, Notion, GitHub, localhost.
- **Ctrl+click in herdr.** The plugin's `[[link_handlers]] id="figma"`
  sends Figma URLs to `herdr-deck plugin open-link`. That command applies
  the same desktop rewrite (`internal/plugin/openlink.go`).
- **Browser tab reuse** (`internal/launch/browser.go`). `pageKey` treats
  two Figma URLs as the same tab when they have the same file key, or the
  same branch key when there is one, whichever frame they select.
  - `/file/` and `/design/` count as the same view; `/proto/` does not.
  - `figma://` links never go through tab reuse, because `isWeb` says no.

What the deck cannot tell today:

- the file's real name (it only has the URL's slug, which goes stale on a
  rename);
- the frame's name;
- whether the design moved on since the work started;
- whether anyone left comments on the design.

## 2. Auth, limits, images and how to fetch

### Auth: a personal access token, never stored

- **Personal access token (PAT).** You create one under Settings →
  Security → Personal access tokens and choose its scopes there. It is
  sent as `X-Figma-Token: <token>`.
  - Since 2025-04-28, PATs expire after at most 90 days. Non-expiring
    tokens can no longer be created.
  - So the deck should treat a 403 as "the token has probably expired" and
    say so in the Sources view.
- **Source of the key.** Mirror the Linear key:
  - Read `$FIGMA_ACCESS_TOKEN`, the variable name Figma's Code Connect CLI
    reads.
  - Otherwise read `figma_token_command` from the config file, for example
    `op read op://Private/Figma/token`. Run it the same way as
    `linear_api_key_command`: no shell, a 30 s timeout, at most once per
    deck start, and its output never echoed.
  - Keep the token in memory only, and redact it from every error. The
    config file holds the command, never the token.
- **Scopes per idea.** Ask for the fewest:

  | Scope | Ideas |
  |---|---|
  | `file_metadata:read` | F1, F2, F7 |
  | `file_comments:read` | F3 |
  | `file_versions:read` | F2 (version list) |
  | `file_content:read` | F4, F5, F6 |
  | `file_dev_resources:read` | F8 (read part) |
  | `file_dev_resources:write` | F8 (write part) |

  A token with only the first three covers the shortlist's cheap reads.
- **OAuth 2** (PKCE with S256, 90-day access tokens with a refresh token,
  `Authorization: Bearer`). A *private* OAuth app needs no review by Figma,
  but it does need a registered redirect URL and a client secret. The docs
  do not say whether a `localhost` redirect URL is allowed. That is too much
  for a single-user tool. Revisit it only if the deck gets other users.
- **Plan access tokens** are made by admins, belong to no user and exist
  only on Organization and Enterprise plans. They are not a fit.

### Rate limits shape everything

Since 2025-11-17, limits depend on **the requester's seat** and on **the
plan of the file's owner**. Usage is tracked per user and plan for a PAT.
Calls per minute unless stated:

| Tier | Endpoints the ideas use | View/Collab seat | Dev/Full: Starter / Pro / Org |
|---|---|---|---|
| 1 | `GET /v1/files/:key`, `/nodes`, `GET /v1/images/:key` | **up to 20 per month** | 10 / 15 / 20 |
| 2 | comments, versions, dev resources, image fills, webhooks | up to 5 | 25 / 50 / 100 |
| 3 | `GET /v1/files/:key/meta`, `/v1/me`, components and styles | up to 10 | 50 / 100 / 150 |

Caveats from the page itself:

- It says the actual View/Collab limit "may be lower", possibly two Tier 1
  requests a month.
- Its prose and its table disagree on the Starter Tier 1 number (6 versus
  20 a month).
- It gives no Enterprise figures I could confirm.
- A 429 carries `Retry-After` (seconds), `X-Figma-Plan-Tier` and
  `X-Figma-Rate-Limit-Type` (`low` for View/Collab, `high` for Dev/Full).

What this means for a pane that refreshes every few seconds:

- **Tier 3 `/meta` is the polling endpoint.** It returns `name`,
  `folder_name`, `last_touched_at`, `last_touched_by`, `thumbnail_url`,
  `version`, `editorType`, `role` and `url`, without the document tree.
  - Poll it at most every 5 minutes per file key, and only for files linked
    from threads that are not resolved.
  - A project with six Figma files makes about 1.2 calls a minute. That is
    well under even the View/Collab limit of 10.
- **Tier 2** (comments, versions) is polled only when `/meta`'s `version`
  changed, or once on first sight. Comments do not bump the file version,
  so either poll them separately every 10 minutes or fetch them when the
  drawer opens on the thread.
- **Tier 1** (node names, renders, dev status) is **never polled**. It
  runs on a key press, or once per (file, node, version) when the user has
  turned it on. Results are cached on disk under
  `$XDG_STATE_HOME/herdr-deck/figma/`. A 429 on Tier 1 stops Tier 1 calls
  until `Retry-After`. If the type was `low`, it also says once that this
  seat cannot afford renders.
- **Failures** follow the Linear reader:
  - go to `Snapshot.Missing` as `Figma: …`;
  - wait a minute after an error;
  - wait 5 minutes after a refused token;
  - wait until `Retry-After` after a 429.

  A reload never waits on the network.
- **Node IDs.** URLs use `node-id=598-48083`; the API uses `598:48083`.
  Convert the hyphen to a colon.
- **Branch links.** For a branch link (`/design/<key>/branch/<branchKey>/…`),
  use the branch key: REST endpoints accept it. The exceptions are dev
  resources and published variables, which need the main file key.
- **Webhooks are out.** Webhooks v2 (`FILE_UPDATE`, `FILE_COMMENT`,
  `DEV_MODE_STATUS_UPDATE`, …) need an endpoint reachable from Figma.
  `FILE_UPDATE` fires only after 30 minutes without edits anyway. A local
  TUI has no public URL, so it polls.

### What a terminal can show

The deck runs inside herdr, which is itself a terminal. It parses each
pane's output with an embedded libghostty-vt and redraws the pane into the
outer terminal. So the question is what **herdr** passes on, not what the
user's terminal supports.

| Way to show an image | Inside herdr 0.9.3 | Outside herdr |
|---|---|---|
| Kitty graphics protocol (APC `ESC _ G`), incl. Unicode placeholders (U+10EEEE) | **Yes**, when the outer terminal is Ghostty, Kitty or WezTerm (`[terminal] kitty_graphics`, on by default) | Kitty, Ghostty; WezTerm without placeholders |
| iTerm2 inline images (OSC 1337) | No, not handled (inferred: herdr redraws panes and documents only Kitty) | iTerm2, WezTerm |
| Sixel (DCS) | No: closed as a feature request ("Kitty graphics only") | iTerm2, WezTerm, others; not Ghostty/Kitty |
| Half-block `▀` with true-colour fg/bg | Yes, it is plain text | Everywhere with true colour |
| Open the PNG in Preview or a browser | Yes | Yes |

Details that matter for a build:

- **Which outer terminals count.** herdr decides from its own environment:
  `TERM_PROGRAM` ghostty or wezterm, `TERM` `xterm-ghostty`, `xterm-kitty`
  or `xterm-wezterm`, or `KITTY_WINDOW_ID` (`src/client/handshake.rs`).
  - With **iTerm2 as the outer terminal, images do not appear** (herdr
    issue #3941, open).
  - Inside a herdr pane the deck sees `TERM_PROGRAM=herdr` and
    `HERDR_ENV=1`, not the outer terminal. So it cannot tell whether an
    image will show.
  - A Kitty `a=q` query might answer that, but that was not tested. The
    safe design is a setting, `figma_previews = "kitty" | "blocks" | "off"`,
    with `blocks` as the default.
- **Placeholders are the right Kitty mode in a TUI.** Send the image once
  with `U=1` (a virtual placement) through `tea.Raw`. Then draw U+10EEEE
  cells in `View()`.
  - The cells move with the text, so scrolling and Lip Gloss layout keep
    working. `ansi.StringWidth` counts each cell as one column.
  - herdr bug #4858: a placeholder image is stretched to the grid, so pick
    the grid from the image's aspect ratio.
  - Bubble Tea's cell renderer keeping the combining diacritics was not
    tested end to end.
- **Libraries.** These are already in the module graph, maintained by
  Charm:
  - `github.com/charmbracelet/x/ansi/kitty` has the encoder,
    `VirtualPlacement`, `Placeholder` and `Diacritic`.
  - `github.com/charmbracelet/x/mosaic` renders an `image.Image` to half or
    quarter blocks.
- **Resolution.**
  - A 40 × 12 cell half-block preview is 40 × 24 real pixels: enough to
    recognise a layout, not to read text.
  - The same 40 × 12 cells through Kitty graphics on a Retina screen show
    roughly 650 × 430 device pixels.

### Fetching: one reader, shaped like Linear's

`internal/source/figma` follows `internal/source/linear`:

- `live.Source.Read` calls `Apply`, which lays the cache over every Figma
  link.
- `Apply` starts one background fetch at a time for keys that are missing
  or stale.
- When the fetch ends, `OnUpdate` sends a reload.
- Tests use a fake `http.RoundTripper` and command runner; they never call
  Figma.

Data goes on the link the way `Link.Issue` does for Linear, as a new
`Link.Figma *FigmaFile` with these fields:

- name, `last_touched_at`, last editor, version;
- the open comment count (all, and on this node);
- an optional node name;
- an optional dev status.

---

## 3. The catalogue

Every idea gives:

- what it does and a sketch;
- data and API calls, auth, read or write;
- limits and caching;
- effort: S is about a day, M a few days, L a week or more;
- value and risks.

Sketches are drawer fragments at 80 columns, in the current style:
`── Name ──` sections, `[N label]` chips, dim metadata. Ideas are in rank
order, and the overall rank is in [§5](#5-shortlist). **Only F8 writes to
Figma.**

#### F1. Real file names and "edited 2h ago" on Figma chips — **read**, S

Chips take the file's current name from Figma instead of the URL slug. A
dim suffix says when the file was last touched and by whom. The list's
LINKS column is unchanged.

```text
 ── Links ──
 [1 ABC-1051 in progress] [2 Figma Templates v2 · edited 2h ago (ana)]
 [3 Admin kit · 3d]
```

At 60 columns the editor drops out: `[2 Figma Templates v2 · 2h]`.

- **Data:** `GET /v1/files/:key/meta`, giving `name`, `last_touched_at`
  and `last_touched_by.handle`.
- **Auth:** PAT, `file_metadata:read`. **Read.**
- **Limits:** Tier 3. One call per file key every 5 minutes, only for
  links on threads that are not resolved. Links on resolved threads keep
  their last answer.
- **Effort:** S. Most of it is the reader (token, cache, back-off), which
  every later idea reuses.
- **Value:** medium. It fixes stale slugs (`Untitled`, renamed files) and
  shows at a glance whether a design is alive.
- **Risks:**
  - The token expires after at most 90 days, so the "token refused" note
    must be clear.
  - A file the token cannot see returns 403 or 404. Show the slug as today,
    with a dim `?`.

#### F2. "Design changed since the thread started" — **read**, S

The deck compares the file's `last_touched_at` with the thread's `created`
time (and with `launched_at`, when the thread was briefed). When the design
moved after the thread started, the chip gets a yellow `◆` and the
*Thread* section says what changed. Named versions come from the version
history.

```text
 ── Links ──
 [1 ABC-1051 in progress] [2 Figma Templates v2 ◆ changed 2h ago]
 ...
 ── Thread ──
 design   Templates v2 changed after the thread started (ana, 2h ago)
          versions since: "Handoff 2" (yesterday)
```

- **Data:**
  - `/meta` (as F1) for `last_touched_at` and `version`.
  - On a change, `GET /v1/files/:key/versions?page_size=10` for named
    versions newer than the thread's `created`: `label`, `description`,
    `user`, `created_at`.
- **Auth:** `file_metadata:read`, plus `file_versions:read` for the
  labels. **Read.**
- **Limits:** Tier 3, plus Tier 2 only when `version` changed.
- **Effort:** S once F1 exists.
- **Value:** high for design-driven work. "Did the design move under me?"
  is the question an agent's brief cannot answer.
- **Risks:**
  - `last_touched_at` is file-wide. An edit to another page also counts,
    so the wording must say "file changed", not "frame changed".
  - Knowing whether the linked frame changed needs F4's Tier 1 data.
  - Showing the versions as events in the Log tab would go against the
    decision that Log keeps to what herdr-projects records. See decision 5.

#### F3. Open comments on the design, with a count badge — **read**, M

Overview's *Links* chip gets `💬 3` (or `3 open` without emoji). A Figma
section, or a full view on a key, lists the open comment threads: who,
what, and how long ago. Comments pinned to the linked node come first.

```text
 ── Figma ──
 [2 Templates v2] 3 open comments · 1 on this frame
 ▸ ana   2h  "Spacing under the header should be 24, not 16"   frame
   lee   1d  "Do we need the empty state here?"
   ana   3d  "Copy TBD"
```

- **Data:**
  - `GET /v1/files/:key/comments?as_md=true`. Open threads are the root
    comments (no `parent_id`) with `resolved_at` null. Reply counts come
    from `parent_id`.
  - A comment is tied to a node only through `client_meta.node_id`
    (FrameOffset forms). The API has no filter by node, so filter on the
    client.
- **Auth:** `file_comments:read`. **Read.**
- **Limits:** Tier 2. Every 10 minutes per file, or when the drawer opens
  on the thread and the cache is over 2 minutes old.
- **Effort:** M. The fetch is small; the view and the node matching take
  the time.
- **Value:** high when designers review in Figma. Unanswered design
  feedback is otherwise invisible to the person running the agents.
- **Risks:**
  - "On this frame" matches only comments pinned directly to the linked
    node. A comment on a child layer has the child's `node_id`. Knowing it
    sits inside the frame needs the node tree (Tier 1), so the first
    version says "on this frame" only for exact matches.
  - Comment text is user content. Strip control characters, as the diff
    view does.
  - Should open comments tint the row the way inbox items do? See
    decision 4.

#### F4. Frame name and Dev Mode status on the chip — **read**, M (Tier 1)

The chip shows the frame's name instead of its number. A dim marker shows
its Dev Mode status (`ready for dev` or `completed`).

```text
 ── Links ──
 [2 Figma Templates v2 › Template list] [3 › Empty state ready for dev]
```

- **Data:** `GET /v1/files/:key/nodes?ids=598:48083&depth=1`, giving
  `nodes[id].document.name` and `devStatus.type` (`NONE`, `READY_FOR_DEV`
  or `COMPLETED`). Batch every node of one file into one call.
- **Auth:** `file_content:read`. **Read.**
- **Limits:** **Tier 1.** View/Collab seats get about 20 calls a month, so
  this is opt-in (`figma_node_details = true`).
  - Cache on disk by (file key, node id, file `version`). Re-fetch only
    when F1 sees a new `version`, and then at most once an hour.
  - Without the setting, the chip keeps the URL's node number.
- **Effort:** M, because of the opt-in, the disk cache and the 429
  handling.
- **Value:** medium. `598-48083` means nothing; "Template list" does.
  "Ready for dev" tells the user the design is handed off.
- **Risks:**
  - It burns Tier 1 quota that F6 also wants.
  - `depth=1` still returns the node's whole subtree for a big frame. Use
    `depth=1` and ignore the children; there is no names-only call.

#### F5. Open the exact frame, in the browser, the app or Dev Mode — no API, S

These are improvements to what exists, with no API:

- **Dev Mode.** A setting `figma_dev_mode = true`, or a `D` choice in the
  `f` chooser, adds `m=dev`. Figma documents that a Dev Mode link drops
  you into Dev Mode.
- **The other Figma kinds.** The scraper also recognises `/slides/`,
  `/deck/`, `/make/`, `/site/` and `/buzz/`. Launch already knows
  slides/deck. Chips name the kind when it is not a design file: `board`,
  `proto`, `slides`.
- **Branches.** Chips mark branch links (`Templates v2 ⑂ branch`).

```text
 f  pick a Figma link: 1-9 open · a all · d desktop app · D Dev Mode · esc
```

- **Data:** none.
- **Auth:** none.
- **Effort:** S.
- **Value:** medium, and cheap. Developers want Dev Mode, not the design
  canvas.
- **Risks:**
  - `figma://` is not documented by Figma. A forum report says node focus
    may not work when a link opens through it. Figma's documented route is
    the "Open links in desktop app" preference, which takes over `https`
    links through FigmaAgent.
  - Recommendation: in the README, point users who want the desktop app at
    that preference, and keep `figma_desktop` as the fallback.
  - Check by hand whether `figma://…?node-id=` lands on the frame.

#### F6. Frame preview: inline image, half-block, or "open preview" — **read**, M–L

`P` (or a chip's preview action) renders the linked frame. The image goes
in the list's place, like the diff preview, or in a full drawer view.

- With `figma_previews = "kitty"` on Ghostty or Kitty inside herdr, it is a
  real image.
- With `blocks` (the default), it is a half-block mosaic.
- With `off`, or on any failure, `o` opens the cached PNG in Preview.

```text
 Templates v2 › Template list                 rendered 2h ago · o open PNG
 ▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀
 ▀▀▀▀▀▀▀▀▀▀▀▀  (true-colour half-blocks: 56×20 cells = 56×40 px)
 ▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀
```

A cheaper first step needs no Tier 1 call. `/meta`'s `thumbnail_url` is
the **file's** thumbnail (its cover), which F1 already fetches. It is a
plain image download.

- **Data:**
  - `GET /v1/images/:key?ids=598:48083&format=png&scale=0.5` returns a
    temporary URL (valid up to 30 days). Download it once.
  - Cache the PNG under `…/figma/<key>/<node>@<version>.png`.
  - Or use `thumbnail_url` for the file-level cover.
- **Auth:** `file_content:read` for renders. **Read.**
- **Limits:** Tier 1. Render only on a key press, never ahead of time.
  Reuse the cache while the file's `version` is the same.
- **Effort:**
  - M for half-blocks and "open PNG", using `x/mosaic` and the existing
    full-view plumbing.
  - L with Kitty placeholders, which needs `tea.Raw`, image IDs, cleanup
    on resize and testing across terminals.
- **Value:** medium. Seeing the frame next to the running dev server is the
  heart of design QA. But a 56 × 40 pixel mosaic only shows the layout,
  and the real check happens in Figma or the browser anyway.
- **Risks:**
  - Tier 1 quota.
  - herdr's open Kitty bugs: stretching (#4858), flicker on redraw
    (#3676), large frames dropped (#3946).
  - The deck cannot detect the outer terminal from inside herdr.
  - Images make golden tests harder; keep the image out of the goldens.

#### F7. A Figma section in Overview — **read**, S once F1–F3 exist

Instead of only chips, Overview gets a *Figma* section after *Links*. It
collects what F1–F4 know, one line per file, and the Figma chips move into
it, the way the PR's chip lives only in *PR*.

```text
 ── Figma ──
 [2 Templates v2] edited 2h ago (ana) ◆ after thread start · 3 open
   › Template list ready for dev · › Empty state
 [3 Admin kit] edited 3d ago
```

At 60 columns, the second line keeps only the frame names.

- **Data:** whatever F1–F4 fetched. No calls of its own.
- **Effort:** S. Section ordering, chips and numbering already exist. The
  digits stay in the Linear, Figma, Notion, GitHub order.
- **Value:** medium. It gathers the design context in one place without
  crowding *Links*.
- **Risks:**
  - It adds a seventh section to an eight-line Overview.
  - Show it only when a Figma link has API data; otherwise the chips stay
    in *Links* as today.

#### F8. Link the PR to the frame in Dev Mode — **WRITES TO FIGMA**, M

When a thread's PR opens and the thread links a Figma frame, the deck
offers to add the PR as a **dev resource** on that frame. Designers and
developers in Dev Mode then see "PR #2320" on the frame. The read half
shows a frame's existing dev resources (other PRs, Storybook, docs) as
chips.

```text
 ── Figma ──
 [2 Templates v2 › Template list] dev links: Storybook, PR #2290
 + link PR #2320 to this frame in Dev Mode?  y yes · n no · never for this
```

- **Data:**
  - Read with `GET /v1/files/:key/dev_resources?node_ids=598:48083`.
  - Write with `POST /v1/dev_resources {dev_resources:[{name:"PR #2320",
    url, file_key, node_id}]}`.
  - Errors come back in `errors[]` even with a 200.
  - The write needs the **main** file key, not a branch key.
  - At most 10 dev resources per node, and no duplicate URL on one node,
    so check the read first.
- **Auth:** `file_dev_resources:read`, plus `file_dev_resources:write` for
  the write. **Writes to Figma.**
  - Dev resources are a Dev Mode feature, and Dev Mode needs a paid plan
    and a Full or Dev seat.
  - The API docs do not say whether the endpoint enforces that, so test it
    with the user's real seat before building.
- **Limits:** Tier 2. One read when the drawer opens on the thread, one
  write per PR.
- **Effort:** M. The write needs a confirm, a "never for this file"
  memory, and the undo path (`DELETE /v1/files/:key/dev_resources/:id`).
- **Value:** medium-high for teams that work in Dev Mode. It closes the
  loop from design to code without anyone pasting a link.
- **Risks:**
  - It is a write, visible to the whole team in Figma.
  - Default off (`figma_writes = false`). Always confirm, naming the frame
    and the PR. Never trigger it from a key that also opens or navigates.
  - Alternative: the coordinator or thread agent does it through the Figma
    MCP server, or the user asks for it (see decision 2).

#### F9. Design specs for the thread's agent through Figma's MCP server — outside the deck, S

The deck does not fetch specs itself. Figma's MCP server already gives
agents what they need from a frame link:

- `get_design_context`, whose default output is React + Tailwind;
- `get_variable_defs`, `get_screenshot` and `get_metadata`;
- `get_code_connect_map`, when Code Connect is set up.

The deck's part is small:

- **A Sources-view check.** Is a Figma MCP server configured for the
  agents? Look in the user's and the worktree's `.mcp.json` or Claude
  settings, read-only. Show a dim note when threads link Figma frames but
  no Figma MCP server is configured.
- **A copy action.** `y` on a Figma chip copies a ready line such as `Use
  the Figma MCP server's get_design_context on <url>`. The deck never
  types into an agent's pane on its own.

```text
 ── Figma ──
 [2 Templates v2 › Template list]   y copy for agent · P preview
 ! no Figma MCP server configured for agents (see README)
```

- **Data:** none from Figma. The deck reads local config only.
- **Auth:** none in the deck. The remote server (`https://mcp.figma.com/mcp`)
  logs in with OAuth in the agent's client.
  - The remote server works on every seat and plan, but read tools on
    View/Collab seats are capped at up to 6 calls a month on paid plans.
  - The desktop server (`http://127.0.0.1:3845/mcp`) needs a Dev or Full
    seat and the desktop app open.
- **Effort:** S. Most of the value is in the coordinator's brief and the
  README, not in code.
- **Value:** high, but it lands in the agent, not the deck. This is where
  frame specs pay off: the agent builds from them.
- **Risks:**
  - The MCP server's read quota is separate from, and smaller than, the
    REST quota on cheap seats.
  - The deck must not try to proxy the MCP server or hold its OAuth token.

#### F10. Other projects' design activity in the `p` picker — **read**, S–M

The project picker shows, per project, a dim `◆ design changed` when F2
fires for any of its unresolved threads.

```text
 admin-rebuild   here   ◐ 2  ◇ 1   ◆ design changed
 webshop                ◐ 1        ✉ 2 updates
```

- **Data:** F1/F2's cache, kept per project. It needs other projects'
  links scraped, which the picker does not do today.
- **Limits:** this multiplies `/meta` calls by the number of projects. Poll
  other projects every 15 minutes, or only show what is already cached.
- **Effort:** S–M.
- **Value:** low to medium. Design changes are news, not needs, so this
  must stay as quiet as the inbox `✉`.
- **Risks:**
  - Polling cost.
  - The rule is that only waiting threads count in the picker. This would
    be a dim note, never a need.

---

## 4. Ideas not to build

- **Design tokens and variables drift against code.**
  - The Variables REST API (`/v1/files/:key/variables/local` and
    `/published`) is **Enterprise only**: "you must have a Full seat in an
    Enterprise org".
  - Comparing variables with a codebase's tokens is a lint job, not a
    status-pane job.
  - A thread agent can do it on demand with the MCP server's
    `get_variable_defs`.
- **Code Connect coverage.** Code Connect needs a Dev or Full seat on an
  Organization or Enterprise plan, and lives in the repo
  (`figma.config.json`, `*.figma.ts`). Whether a component is mapped is a
  repo concern for the agent and CI, not the deck.
- **Replying to or resolving Figma comments from the deck.** The API can
  post and delete comments, but has no documented resolve call. Writing
  prose belongs in Figma or the agent. The deck stays a reader of comments
  (F3).
- **Webhooks.** They need an endpoint Figma can reach, and a passcode; no
  HMAC is documented. `FILE_UPDATE` lags 30 minutes. Polling `/meta` is
  simpler and fresher for a local pane.
- **Writing designs** (`use_figma`, `generate_figma_design` on the remote
  MCP server). That is agent work, and a beta Figma says will become a
  paid feature.
- **OAuth app for the deck.** It needs a client secret and a registered
  redirect for one user. Revisit only if the deck gets other users.
- **Sixel or iTerm2 image paths.** herdr shows neither. Kitty
  placeholders or half-blocks cover herdr; outside herdr the deck rarely
  runs.

## 5. Shortlist

Overall rank by value for effort, reads first:

| # | Idea | R/W | Effort | Why first |
|---|---|---|---|---|
| 1 | [F1](#f1-real-file-names-and-edited-2h-ago-on-figma-chips--read-s) Real file names, last edited | read | S | One Tier 3 call per file. It brings in the reader, token handling and back-off that every later idea needs. |
| 2 | [F2](#f2-design-changed-since-the-thread-started--read-s) Design changed since the thread started | read | S | The question nothing else answers. Nearly free once F1 exists. |
| 3 | [F5](#f5-open-the-exact-frame-in-the-browser-the-app-or-dev-mode--no-api-s) Dev Mode links, more Figma kinds, branches | none | S | No token needed. Benefits every user today. |
| 4 | [F3](#f3-open-comments-on-the-design-with-a-count-badge--read-m) Open comments with a count | read | M | Design feedback becomes visible next to the work. Tier 2 is affordable. |
| 5 | [F9](#f9-design-specs-for-the-threads-agent-through-figmas-mcp-server--outside-the-deck-s) Specs through the MCP server, plus a Sources check | none | S | The real spec work belongs in the agent. The deck only nudges. |

Next in line:

- F7, the Figma section, once F1–F3 give it content;
- F8, linking the PR as a dev resource, the only write, which needs
  decision 2;
- F4, frame names, which costs Tier 1 calls;
- F6, the preview, which costs Tier 1 and terminal work: half-blocks first,
  Kitty later.

Suggested order of PRs:

1. F5;
2. F1 with the Figma reader;
3. F2;
4. F3;
5. F9.

F5 needs no token, so it can go first or in parallel. Each is one PR with
a CHANGELOG line, as usual.

## 6. Decisions for the user

1. **A Figma token at all?** Recommendation: yes, a PAT with
   `file_metadata:read`, `file_comments:read` and `file_versions:read`,
   from `$FIGMA_ACCESS_TOKEN` or `figma_token_command` (e.g. `op read …`).
   It expires within 90 days, so plan to rotate it. Without a token, F5
   and F9 still work.
2. **Writes to Figma (F8)?** Recommendation: not in the first round. If
   yes, it goes behind `figma_writes = false` by default and a `y/n`
   confirm naming the frame and PR. Choose who performs it: the deck, or
   the coordinator or thread agent through the Figma MCP server.
3. **Tier 1 features (F4 frame names, F6 previews).** What seat does the
   user have on the plan that owns the design files?
   - A **View or Collab** seat gets up to about 20 Tier 1 calls a month:
     leave F4 and F6 out, or keep F6 as "open the file thumbnail".
   - A **Dev or Full** seat allows 10–20 a minute: F4 and F6 can be opt-in
     settings.
4. **Do open Figma comments count as attention?** Recommendation: no. They
   are news like inbox items: a count on the chip, never a *Needs you*
   row, and no row tint. Possibly a dim marker in the picker.
5. **Figma events in the Log tab?** Named versions and new comments would
   fit Log's timeline. But the drawer decision says Log keeps to what
   herdr-projects records. Recommendation: keep Log as decided and show
   them in *Thread* (F2) and *Figma* (F3/F7) instead.
6. **Previews: which way to draw?** If F6 is built, start with half-blocks
   and "open PNG". Add Kitty placeholders behind
   `figma_previews = "kitty"`, since the deck cannot detect from inside
   herdr whether the outer terminal shows images. Which outer terminal
   does the user run herdr in? Ghostty or Kitty would make Kitty
   worthwhile; iTerm2 would not.
7. **The desktop app.** Recommendation: document Figma's own "Open links
   in desktop app" preference as the main route, and keep `figma_desktop`
   (the undocumented `figma://` rewrite) as a fallback. Verify once by
   hand whether `figma://…?node-id=` focuses the frame.

## 7. Sources

Figma REST API:

- File endpoints (GET file, nodes, images, image fills, `/meta`):
  https://developers.figma.com/docs/rest-api/file-endpoints/
- OpenAPI spec (fields such as `last_touched_at`, `devStatus`,
  `client_meta`): https://github.com/figma/rest-api-spec
  (`openapi/openapi.yaml`)
- Rate limits: https://developers.figma.com/docs/rest-api/rate-limits/
- Authentication, PATs, OAuth apps, scopes:
  https://developers.figma.com/docs/rest-api/authentication/,
  https://developers.figma.com/docs/rest-api/personal-access-tokens/,
  https://developers.figma.com/docs/rest-api/oauth-apps/,
  https://developers.figma.com/docs/rest-api/scopes/
- Changelog (`/meta` added 2025-04-29, PAT expiry 2025-04-28, rate limits):
  https://developers.figma.com/docs/rest-api/changelog/
- Comments: https://developers.figma.com/docs/rest-api/comments-endpoints/,
  https://developers.figma.com/docs/rest-api/comments-types/
- Versions: https://developers.figma.com/docs/rest-api/version-history-endpoints/
- Dev resources: https://developers.figma.com/docs/rest-api/dev-resources-endpoints/
- Variables (Enterprise): https://developers.figma.com/docs/rest-api/variables/,
  https://developers.figma.com/docs/rest-api/variables-endpoints/
- Webhooks v2: https://developers.figma.com/docs/rest-api/webhooks/,
  https://developers.figma.com/docs/rest-api/webhooks-events/

Figma MCP server and Code Connect:

- https://developers.figma.com/docs/figma-mcp-server/
- https://developers.figma.com/docs/figma-mcp-server/tools-and-prompts/
- https://developers.figma.com/docs/figma-mcp-server/rate-limits-access/
- https://developers.figma.com/docs/figma-mcp-server/remote-server-installation/
- https://developers.figma.com/docs/figma-mcp-server/local-server-installation/
- https://help.figma.com/hc/en-us/articles/32132100833559-Guide-to-the-Figma-MCP-server
- https://developers.figma.com/docs/code-connect/ and the quickstart
  (`FIGMA_ACCESS_TOKEN`): https://developers.figma.com/docs/code-connect/quickstart-guide/

Desktop app, Dev Mode and URLs:

- Open links in the desktop app:
  https://help.figma.com/hc/en-us/articles/360039824334-Open-links-in-the-desktop-app
- Troubleshooting desktop links:
  https://help.figma.com/hc/en-us/articles/22850791655831-Troubleshoot-opening-links-in-the-Desktop-app
- Dev Mode: https://help.figma.com/hc/en-us/articles/15023124644247-Guide-to-Dev-Mode
- URL formats and node IDs: https://developers.figma.com/docs/embeds/resources/,
  https://help.figma.com/hc/en-us/articles/1500005554982-Guide-to-files-and-folders,
  https://help.figma.com/hc/en-us/articles/5665697002263-Share-a-branch
- `figma://` (not official, forum):
  https://forum.figma.com/t/link-to-node-does-not-work-for-desktop-app/13733

Terminal images and herdr:

- Kitty graphics protocol, Unicode placeholders:
  https://sw.kovidgoyal.net/kitty/graphics-protocol/
- iTerm2 inline images: https://iterm2.com/documentation-images.html
- Sixel support by terminal: https://www.arewesixelyet.com/
- herdr: https://github.com/herdrdev/herdr (CHANGELOG, the configuration
  docs' "Kitty graphics" section, `src/client/handshake.rs`), issues #3941
  (iTerm2 outer terminal), #4858 (placeholder stretching), #3676, #3946,
  and the closed Sixel requests #3030 and #4330; `herdr --default-config`
  (`[terminal] kitty_graphics`)
- WezTerm placeholders: https://github.com/wezterm/wezterm/issues/986
- Go: https://github.com/charmbracelet/x (`ansi/kitty`, `mosaic`)
