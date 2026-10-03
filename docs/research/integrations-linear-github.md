# Linear and GitHub integrations — ideas

Written 2026-10-03 against herdr-deck v0.1.3, herdr-projects 0.2.34, gh
2.x, Linear's GraphQL API and GitHub's REST and GraphQL APIs as documented
that day. Exploration only: no code was changed for this document. Limits
and field names below were checked that day, so verify them before you
rely on them. Examples use the made-up `admin-rebuild` project (`ABC-`
tickets, `acme/webshop`) of the [design README](../design/README.md).

**Recommendation in one paragraph.** Build a small read-only GitHub reader
(`internal/source/github`) that shells out to `gh` the way herdr-projects
already does, so no token is ever handled by the deck. Use it first to make
the drawer's *PR* section say **why a PR is not merging**: behind main,
checks still running, a conflict, or auto-merge on. Next, show the **tail
of a failing check's log**. Widen the existing Linear query so chips carry
the **issue's title and assignee**, with a URL that works without
`linear_workspace`. Then list **unresolved review threads**. All four are
reads. Writes come after that, one at a time, each behind a confirm and a
setting that is off by default: re-run failed jobs first, merging second.
Leave Linear state changes to Linear's own GitHub integration, which
already moves issues when PRs open and merge. The ranked shortlist is in
[§5](#5-shortlist), and the decisions for the user are in
[§6](#6-decisions-for-the-user).

Contents:

1. [What the deck has today](#1-what-the-deck-has-today)
2. [Auth, limits and how to fetch](#2-auth-limits-and-how-to-fetch)
3. [The catalogue](#3-the-catalogue)
4. [Ideas not to build](#4-ideas-not-to-build)
5. [Shortlist](#5-shortlist)
6. [Decisions for the user](#6-decisions-for-the-user)
7. [Sources](#7-sources)

---

## 1. What the deck has today

- **Linear** (`internal/source/linear`). It gets the key from
  `$LINEAR_API_KEY`, else from `linear_api_key_command`'s output, and keeps
  it in memory only. It sends one GraphQL request per 50 IDs, with one
  aliased `issues(filter: {team.key, number in})` per team, and asks only
  `identifier state { name type }`. Answers are cached for 3 minutes and
  fetched in the background. After an error it backs off: a minute after a
  failure, 5 minutes after a refused key, and until the reset time after a
  rate limit. URLs are built from `linear_workspace`. The chip shows
  `ABC-123 in progress`.
- **GitHub.** The deck calls nothing itself. Every 2 minutes the
  herdr-projects ticker runs `gh pr view --json
  state,reviewDecision,statusCheckRollup,comments,reviews,…` per thread PR
  (herdr-projects `src/pr.rs`) and writes a summary to `ticker.json`
  `prs[thread id]`. The summary holds state, review decision, failing check
  names, comment count, commenters and `last_pr_check`. The *PR* section
  renders it:

  ```text
   ── PR ──
   [5 #2320] open · review required · 2 comments (sam, alex)
   ✕ 2 failing: lint, test (ubuntu-latest)
  ```

  Several things are not there today:
  - pending or running checks;
  - why a PR is blocked (behind, conflict, required check missing);
  - auto-merge;
  - drafts;
  - review threads;
  - logs;
  - the PR body;
  - anything outside the thread's own PR.
- **Principles that still hold.** The deck is read-only towards
  herdr-projects, so it never writes TASKS.md, thread files or the inbox.
  Writes to Linear or GitHub are a new kind of write. Each one is marked
  **write** below, and none should ship without the user's go-ahead (see
  [§6](#6-decisions-for-the-user)).

## 2. Auth, limits and how to fetch

### GitHub: reuse `gh`

The user's `gh` is logged in with the scopes `gist, read:org, repo,
workflow` (checked with `gh auth status`; gh asks for `repo`, `read:org`
and `gist` at least). `repo` covers checks, logs, re-runs, merges and
notifications, and `/notifications` answered 200 to this OAuth-app token.

There are two ways to call GitHub. Neither stores a token.

| | A. Shell out to `gh` (recommended) | B. Go HTTP with `gh auth token` |
|---|---|---|
| Token | never seen by the deck | read once per start, kept in memory, redacted from errors (like the Linear key) |
| Hosts, enterprise, keyring | gh handles them | the deck must resolve host and account |
| Conditional requests (ETag → 304, free) | `gh api --cache <ttl>` caches locally, but sends no `If-None-Match` | yes, the deck keeps ETags |
| Cost | one process per call, about 100–300 ms | one HTTP request |
| Precedent | herdr-projects does exactly this | the deck's Linear reader |

Recommendation: **A**. Use `gh pr view --json`, `gh api graphql` and
`gh api <rest path>` from one reader, run off the UI goroutine with a 10 s
timeout. Use the same backoff as the Linear reader: wait after an error,
and wait until `x-ratelimit-reset` after a 403/429. A missing or logged-out
`gh` is a Sources note, not an error. Switch a single hot path to B only if
the process cost ever shows.

**Budget.** REST allows 5,000 requests per hour. GraphQL allows 5,000
points per hour, and a query costs 1 point per 100 nodes requested,
rounded. A probe of one search plus `rateLimit` cost 1 point. Search has
its own limit of 30 requests per minute. Conditional REST requests that
return 304 are free.

A project with 5 open PRs polled once a minute with one GraphQL query each
uses 300 points an hour, about 6 % of the budget. herdr-projects'
`gh pr view` calls draw on the same budget, since it is the same user.
Rules that keep it small:

- Poll only PRs of threads that are not resolved, and only while the deck
  runs.
- Fetch heavy data (logs, review threads, bodies) only when the drawer
  shows it, and cache it by head SHA. A log never changes once its job has
  finished.
- Ask about `main` once per repo, not once per thread.

### Linear: keep the personal key

- **Personal key:** sent as `Authorization: <key>`.
- **OAuth:** `Bearer`, with tokens that live 24 hours plus refresh tokens,
  PKCE, a registered app, and scopes `read`, `write`, `issues:create`,
  `comments:create` and `admin`.
- **Limits:** a key gets 2,500 requests and 3,000,000 complexity points
  per hour; an OAuth app gets 5,000 requests and 2,000,000 points. One
  query may cost at most 10,000 points. A field costs 0.1 point, an object
  1, and a connection multiplies its children by `first`, which defaults to
  50. The `X-Complexity` header gives a query's cost.

For a local tool the personal key is simpler, and its points budget is the
larger of the two. OAuth only pays off if the deck ever acts as an app
(`actor=app`) or is shared with people who should not paste keys.

Two limits shape the designs below:

- **Webhooks need a public HTTPS URL**, so a local deck keeps polling.
- **Complexity:** keep `first` small on nested connections. Comments
  `first: 50` inside issues `first: 50` costs 2,500 points or more.

## 3. The catalogue

Every idea lists what it does, a sketch, data and API calls, auth, whether
it reads or writes, limits and caching, effort (S about a day, M a few
days, L a week or more), value and risks. Sketches are drawer fragments at
the 80-column width, in the current style: `── Name ──` sections,
`[N label]` chips and status glyphs.

Ideas are in rank order within each group. The overall rank is in
[§5](#5-shortlist).

### GitHub

#### G1. Why the PR is not merging — **read**, S

The *PR* section explains what stands between the PR and a merge: running
checks, whether the branch is behind its base, a conflict, a draft,
auto-merge, and who still has to review.

```text
 ── PR ──
 [5 #2320] open · approved · behind main · auto-merge on
 ◌ 1 running: test (macos-latest) 4m   ✓ 3 passed
 ⚠ behind main by 2 commits: update the branch before it can merge
```

```text
 ── PR ──
 [5 #2320] draft · review required (sam) · conflicts with main
 ✕ 1 failing: lint   ◌ 1 queued
```

- **Data:** one `gh pr view <url> --json
  mergeStateStatus,mergeable,isDraft,autoMergeRequest,reviewRequests,statusCheckRollup,headRefOid`.
  - `mergeStateStatus` is one of `BEHIND`, `BLOCKED`, `CLEAN`, `DIRTY`,
    `HAS_HOOKS`, `UNKNOWN` or `UNSTABLE`.
  - `statusCheckRollup` items carry `status`/`conclusion`, so the section
    can show running and queued checks next to failing ones.
  - "Behind by N" comes from `gh api
    repos/{o}/{r}/compare/{base}...{head}` (`behind_by`) when asked.
- **Limits:** one call per open PR per minute. A closed or merged PR is
  never asked again.
- **Value:** high. main is protected (up to date, four required checks), so
  "behind" and "running" are why PRs wait. The ticker only reports failing
  checks.
- **Risks:**
  - `mergeable` is `null` while GitHub is still computing it. Show nothing
    and ask again.
  - The section would have two sources (the ticker and the deck), so it
    needs a rule: the newer `checked` time wins.

#### G2. Failing check: log tail in the drawer — **read**, M

A failing check gets a chip. Opening it shows the failing step's last lines
in the drawer (a full-height view like `r`), not in the browser.

```text
 ── PR ──
 [5 #2320] open · review required
 ✕ 2 failing: [6 lint] [7 test (ubuntu-latest)]
```

```text
 test (ubuntu-latest) · ✕ failed 3m ago · step "go test"  esc returns
 ─────────────────────────────────────────────────────────────────────
 --- FAIL: TestUsersPage (0.02s)
     users_test.go:41: got 3 rows, want 4
 FAIL
 FAIL    acme/webshop/internal/users  0.214s
 ok      acme/webshop/internal/admin  0.101s
 Error: Process completed with exit code 1.
 ↵ open in browser  R re-run failed (off)
```

- **Data:**
  1. The rollup's `detailsUrl` holds the run and job IDs
     (`/actions/runs/<run>/job/<job>`).
  2. `gh api repos/{o}/{r}/actions/jobs/{job}/logs` follows the 302 to a
     plain-text log. The redirect expires after 1 minute, so fetch it right
     away. This was checked against FredricW/herdr-deck: the response is
     plain text with timestamps.
  3. To find the failing step, `gh api .../actions/jobs/{job}` lists its
     steps. Cut the log at that step's `##[group]` marker, drop the
     timestamps and keep the last 40 lines.
  4. `gh run view --job <id> --log-failed` does the same in one command,
     but prints every failed step with prefixes.
- **Limits:** fetch only when opened. Cache by job ID, since a finished
  job's log never changes. Logs can be megabytes, so stream them and keep
  the tail.
- **Value:** high. It saves the open-browser, find-job, scroll loop for
  the most common question: why is it red?
- **Risks:**
  - Third-party checks (not Actions) have no log. Fall back to opening
    `detailsUrl`.
  - Logs may hold secrets that GitHub failed to mask. Show them, but never
    write them to disk.
  - ANSI in logs: strip it, or pass the colour codes through after
    sanitising them.

#### G3. Review threads in the drawer — **read**, M (L with diff marks)

The *PR* section lists unresolved review threads with their file, line,
author and first line. Later, the Files tab and diff preview mark lines
that have comments.

```text
 ── Review ──  3 unresolved · 1 outdated
 [8] src/pages/users/index.ts:41   sam   "Should this be paginated?"
 [9] src/pages/users/table.tsx:12  alex  "Rename to UserRow" (+2 replies)
     src/api/users.ts:7 (outdated) sam   "Drop the any here"
```

```text
 Files 4                                  4 files  +210 -12  vs origin/main
 1 M src/pages/users/index.ts    +88 -4  ▰▰▰▰▰▱▱▱  ◆1
 2 A src/pages/users/table.tsx   +96     ▰▰▰▰▰▱▱▱  ◆1
```

- **Data:** `gh api graphql` with `pullRequest(number) { reviewThreads(first:
  50) { nodes { isResolved isOutdated path line startLine diffSide comments(first:
  3) { nodes { author { login } body createdAt } } } } }`. `gh pr view
  --json` has no review threads, so this needs GraphQL. Cost: 1–2 points.
- **Diff marks:** a thread's `line` on the `RIGHT` side is a line of the
  PR head's file. The diff preview shows the worktree. These agree only
  while the worktree is at the PR head with no local edits; otherwise mark
  the thread "moved?" or leave the mark out. That check is the L part.
- **Limits:** fetch when the drawer shows the thread, and again when the
  ticker's `comment_count` or the head SHA changes.
- **Value:** high for threads in review. Today the deck only says "2
  comments (sam, alex)".
- **Risks:**
  - Rendering long Markdown bodies: show the first line, and render the
    full body with glamour on `↵`.
  - Resolving a thread from the deck would be a write. Keep that out.

#### G4. Re-run failed jobs — **write**, S

`R` on a failing PR (or in the G2 log view) asks to confirm, then re-runs
the failed jobs of the latest run.

```text
 ✕ 2 failing: [6 lint] [7 test (ubuntu-latest)]
 Re-run 2 failed jobs of run 4242 on #2320?  y yes  n no
```

- **Data:** `gh run rerun <run> --failed` (failed jobs plus their
  dependents), or `POST
  /repos/{o}/{r}/actions/runs/{run}/rerun-failed-jobs` → 201. The `repo`
  scope is enough.
- **Value:** medium-high for flaky macOS runners. It is a small,
  idempotent and reversible write.
- **Risks:**
  - It re-runs the same commit, so it does not help a PR that is behind.
    Show G1's "update the branch" instead.
  - It costs Actions minutes on private repos.

#### G5. Merge, auto-merge and update branch — **write**, M

`M` on an approved PR offers the project's merge flow:

1. update the branch when it is behind;
2. enable auto-merge while checks run;
3. merge with a merge commit when it is clean.

```text
 ── PR ──
 [5 #2320] open · approved · behind main
 Merge #2320 into main?  u update branch first  a auto-merge  esc cancel
```

- **Data:**
  - `gh pr update-branch <url>` (add `--rebase` to rebase);
  - `gh pr merge <url> --merge --auto`, or `--merge
    --match-head-commit <sha>` so a push between view and merge fails
    safely. The REST call answers 409 on a SHA mismatch.
  - Auto-merge must be enabled in the repo's settings.
- **Value:**
  - medium-high: today the user tells the coordinator, or opens the
    browser, to merge;
  - it fits the project's rule that PRs are merged by the user, since the
    user presses the key.
- **Risks:**
  - It is the most consequential write here. It needs a confirm that names
    the PR, the base and the method, and it should never be bound to a
    single key without confirmation.
  - It bypasses the coordinator's "review, then merge in dependency order"
    flow. The coordinator learns of the merge only from the ticker.
  - The merge method should come from config (`merge_method = "merge"`),
    not be guessed.

#### G6. main's CI in the header — **read**, S

The header shows the state of the base branch of each repo the project
uses, so a red main is visible before any thread hits it.

```text
 Admin rebuild          main ✕ lint   ◐ 2  ◇ 1  ○ 1  ✉ 0
```

- **Data:** per repo in PROJECT.md, `gh api
  repos/{o}/{r}/commits/{branch}/check-runs?filter=latest` (REST, with an
  ETag so an unchanged answer is free in option B), or the rollup through
  GraphQL `repository { defaultBranchRef { target { ... on Commit {
  statusCheckRollup { state } } } } }`. Poll every 2–5 minutes.
- **Value:** medium. It explains failures that are not the thread's fault.
- **Risks:** header width at 60 columns. Show only `main ✕`, and only when
  the branch is red.

#### G7. Review requests for you — **read**, M

PRs that ask for the user's review, across repos, are shown in the `p`
project picker (not in Needs you, which stays for threads).

```text
 Projects                                         type to filter
 ▸ Admin rebuild        here   ◐ 2  ◇ 1
   Billing portal              ○ 1
 ── Reviews for you ──  3
   acme/webshop#2331  Add CSV export          sam    2d
   acme/infra#88      Bump node to 24         alex   5h
```

- **Data:** `gh search prs --review-requested=@me --state=open --json
  repository,number,title,author,updatedAt`. `review-requested:` includes
  team requests; `user-review-requested:@me` covers direct ones only.
  Search allows 30 requests a minute, so poll every 5 minutes, and only
  while the picker is open or once at start.
- **Value:** medium. It is useful, but it is not about this project.
  `gh status` already shows it in a shell.
- **Risks:** scope creep towards a GitHub inbox. Keep it inside the picker.

#### G8. PR description and conversation — **read**, M

`r` on a thread in review offers the PR as well as the report: the body,
the review summary and the last comments, rendered with glamour like
What's new.

```text
 #2320 Document select for summary · sam ✓ approved · alex ✎ 2   esc returns
 ─────────────────────────────────────────────────────────────────────
 ## Summary
 Adds the document picker to the summary page (ABC-1191).
 ## Comments
 alex · 2h  Can we keep the old picker behind a flag?
```

- **Data:** `gh pr view --json body,title,author,latestReviews,comments`.
  Fetch on open and cache by `updatedAt`.
- **Value:** medium. Mostly useful when reviewing other people's PRs in a
  project.
- **Risks:** large bodies and images (show alt text).

#### G9. Notifications count — **read**, M

The header shows `◆ 4` for unread GitHub notifications about the project's
repos. A view lists them by reason (`review_requested`, `ci_activity`,
`mention`).

- **Data:** `GET /notifications` with `If-Modified-Since` set to the last
  `Last-Modified`. A 304 does not count, so obey `X-Poll-Interval`, which
  was 60 s when checked. `repo` scope works.
- **Value:** low-medium. It overlaps with the inbox and with G7, and
  marking items read would be a write.
- **Risks:** noise. Notifications cover every subscribed repo, not just the
  project's.

#### G10. Draft and ready toggle — **write**, S

`gh pr ready <url>` / `gh pr ready --undo`.

- **Value:** low. Threads open PRs the way the coordinator says. Listed for
  completeness.

#### G11. Releases — **read**, S

The project card for a repo shows the latest release and how many merged
PRs it is behind.

```text
 Repos   acme/webshop  v2.4.0 (3d) · 7 merged since
```

- **Data:** `gh api repos/{o}/{r}/releases/latest` (the newest
  non-prerelease) plus `compare/{tag}...{default}`.
- **Value:** low-medium, for projects that ship releases. herdr-deck itself
  does.

### Linear

#### L1. Titles, assignees and real URLs on chips — **read**, S

The Linear chip keeps its short form. The *Links* section gains a
one-line-per-issue form when there is room, and the URL comes from Linear,
so `linear_workspace` stops being required once a key is set.

```text
 ── Links ──
 [1 ABC-1246 done]         Users list: columns and sorting      sam
 [2 ABC-1256 in progress]  Users overview layout            ▲  you
 [3 ABC-1257 todo]         Invite flow                          —
 [4 Figma 598-48083] [5 1138-88367]
```

- **Data:** the existing batched query, asking for more fields per node:
  `title url priority assignee { displayName isMe }` (`▲` = urgent or high
  priority). The cost rises by about 0.5 points per issue, which is
  negligible. The workspace's URL key comes from `viewer { organization {
  urlKey } }`, once per key.
- **Value:** high for a small change. "ABC-1257" alone means nothing at a
  glance.
- **Risks:**
  - Width at 60 columns: keep chips there.
  - Titles in a public screenshot or GIF: demos use `--fake` data, so this
    is fine.

#### L2. Drift between Linear and the work — **read** (both services), S–M

The deck flags mismatches it can see, as a dim line in *Links* or a `⚠`
on the chip, and changes nothing.

```text
 ── Links ──
 [1 ABC-1191 in progress] ⚠ PR #2320 merged 2h ago, issue still open
 [2 ABC-1199 canceled]    ⚠ canceled in Linear, thread still working
```

- **Rules:**
  - PR merged but issue not completed or canceled;
  - issue canceled or done while the thread works;
  - issue assigned to someone else;
  - thread resolved, issue still started.
- **Data:** L1's fields plus the PR state the deck already has.
- **Value:** medium. It catches the gaps Linear's GitHub automation leaves:
  issues mentioned only in prose, PRs with no ID in the title, branch or
  body.
- **Risks:** false alarms when a task names an issue only as context, such
  as "after ABC-1250". Flag only issues on the thread's branch or PR
  (`issueVcsBranchSearch(branchName)` or the PR's IDs), not every mention.

#### L3. Issue view: description and comments — **read**, M

A Linear chip opens in the drawer (`i`, or `↵` on the chip with the drawer
focused) rather than the browser: the description and the latest comments,
rendered with glamour.

```text
 ABC-1256 Users overview layout · in progress · you · ▲ high  esc returns
 ─────────────────────────────────────────────────────────────────────
 Project: Admin rebuild · Cycle 42 (ends Fri)
 Show members with role and last login. Sortable by name.
 - [ ] Empty state
 - [x] Pagination
 ── Comments 2 ──
 sam · 1d  Design is in Figma 598-48083.
```

- **Data:** `issue(id: "ABC-1256") { title description url state { name
  type } assignee { displayName } project { name } cycle { number endsAt }
  comments(first: 5, orderBy: createdAt) { nodes { body user { displayName
  } createdAt } } }`. `issue(id:)` takes the identifier. Fetching one issue
  on demand is fine; a missing ID errors, so handle it. Cost is about 10
  points. Cache by `updatedAt`.
- **Value:** medium. You read the ticket without leaving the terminal.
- **Risks:**
  - Linear Markdown has mentions and embedded images. Render them as plain
    text.
  - Long descriptions: the view scrolls.

#### L4. Project or cycle progress on the card — **read**, M

The project card (a list heading or the project row) shows the Linear
project or current cycle that the project's issues belong to.

```text
 Admin rebuild
 Linear  Admin rebuild ▰▰▰▰▰▰▱▱▱▱ 62% · Cycle 42 ▰▰▰▰▱▱▱▱▱▱ 41% ends Fri
```

- **Data:** for each linked issue, `project { id name progress }` and
  `cycle { number progress endsAt }`. Pick the most common one, or let
  the user name it in config (`linear_project`) or in PROJECT.md. The deck
  only reads PROJECT.md. `progress` is weighted by estimates and is 0
  without them.
- **Value:** medium for projects that map onto one Linear project; noise
  otherwise.
- **Risks:** guessing the mapping wrong. Prefer an explicit setting.

#### L5. My Linear issues — **read**, M

A picker (`L` or a section in `p`) lists the user's open issues. Issues no
task or thread mentions yet are marked, so you see what has no thread.

```text
 My issues                                     type to filter
 ▸ ABC-1257 Invite flow              todo         no thread
   ABC-1256 Users overview layout    in progress  t-0002
   ABC-1301 CSV export bug           triage ▲     no thread
```

- **Data:** `viewer { assignedIssues(filter: { state: { type: { nin:
  ["completed","canceled"] } } }, first: 50) { nodes { identifier title
  state { name type } priority } } }`.
- **Value:** medium. It answers "what should the coordinator pick up
  next?" Starting a thread is the coordinator's job, so `↵` opens the
  issue or copies `ABC-1257` to the clipboard. It does not start anything.
- **Risks:** it duplicates Linear's own "My issues" view.

#### L6. Change an issue's state from the deck — **write**, M

On a Linear chip, `s` opens a state chooser with the team's workflow
states.

```text
 ABC-1256 state:  1 Todo  2 In Progress  3 In Review  4 Done   esc cancel
```

- **Data:**
  - `team { states { nodes { id name type position } } }`, cached per
    team;
  - `issueUpdate(id, input: { stateId })`.
- **Value:** low-medium. It mostly fixes what L2 finds.
- **Risks:** a write with the user's key, done as the user. Linear's
  automation may move the issue back on the next PR event.

#### L7. Create a Linear issue from a task — **write**, M

On a task with no Linear link, `c` creates an issue (title from the task,
description from its notes, in a configured team) and shows the new ID.

- **Data:** `issueCreate(input: { teamId, title, description })`.
- **Value:** low. The deck cannot write the new ID back into TASKS.md
  (read-only towards herdr-projects), so the link would be lost on the
  next reload unless the user or coordinator adds it. The deck could copy
  the ID, or type a message to the coordinator (see decision 4).
- **Risks:** duplicate issues if pressed twice. Check for an open issue
  with the same title first.

### Cross-cutting

#### X1. Other projects' GitHub attention in the picker line — **read**, M

The list's last line, today `● 2 other projects need you`, gains GitHub
and Linear counts as separate, dimmer facts. Needs you stays reserved for
threads.

```text
 ● 2 other projects need you · ✕ 1 red PR · ◆ 3 reviews for you
```

- **Data:** G1/G2 for other projects' thread PRs (from their ticker.json,
  or the Roster's thread files), G7 for reviews, and L5's count for
  assigned issues.
- **Value:** medium. It is one glance for everything outside this project.
- **Risks:**
  - It must not blur the rule that Needs you means a waiting thread
    (decided 2026-10-03).
  - Polling every project's PRs multiplies GitHub calls. Reuse ticker.json
    where it exists; the Roster avoids it today, so this would be a change.

#### X2. A shared fetch layer — infrastructure, S–M

This is not a feature. G1–G9 and L1–L7 want the same machinery:

- background fetch;
- per-key backoff;
- caches keyed by SHA or `updatedAt`;
- `Snapshot.Missing` notes;
- redaction.

The Linear reader already has these. Extract them before the second
reader, or copy its shape into `internal/source/github` deliberately.

#### X3. Writes through the coordinator instead of the API — alternative

Instead of calling GitHub or Linear itself, a write key could type a
request into the coordinator's pane (`pane.send_input`), such as "re-run
the failed checks on #2320" or "merge #2320". The coordinator keeps
control of order and review, and the deck stays a viewer. This is cheap
(S), but slow, fuzzy (an agent interprets it) and interrupts the
coordinator. It suits merges (G5) better than re-runs (G4).

## 4. Ideas not to build

- **Moving Linear issues when a thread starts, opens a PR or merges.**
  Linear's GitHub integration already does this. It moves linked issues to
  *In Progress* when a PR opens and to *Done* when it merges. Per team it
  can also act on drafts, review requests and ready-for-merge, and it has
  per-branch rules. It links an issue by the ID in the branch name or PR
  title, or by magic words in the PR body (`Fixes ABC-123`, or `Part of
  ABC-123` for no state change). The deck duplicating this would race it.
  Better: make sure threads put the ID in the branch or PR title, which
  Linear's `Issue.branchName` gives, and let L2 flag misses.
- **Linking the PR to the issue as an attachment**
  (`attachmentLinkGitHubPR`). The integration creates the attachment when
  it links the PR. Build this only for a workspace without the
  integration.
- **Linear webhooks or GraphQL subscriptions.** Webhooks need a public
  HTTPS URL. The schema declares a `Subscription` type, but the docs do
  not document it, so treat it as unsupported. Poll instead.
- **Commenting on issues or PRs from the deck.** Writing prose is the
  agents' and the user's job in the real tools. The deck has no text
  editor worth the name.
- **Approving PRs from the deck.** Approving your own PR is not allowed,
  and approving others' without reading the diff is a bad habit for the
  deck to make easy.

## 5. Shortlist

Overall rank by value for effort, reads first:

| # | Idea | R/W | Effort | Why first |
|---|---|---|---|---|
| 1 | [G1](#g1-why-the-pr-is-not-merging--read-s) Why the PR is not merging | read | S | main is strict, so "behind" and "running" are what PRs wait on. It brings in the GitHub reader every later idea needs. |
| 2 | [L1](#l1-titles-assignees-and-real-urls-on-chips--read-s) Linear titles, assignees, real URLs | read | S | Three more fields on an existing query, and `linear_workspace` stops being required. |
| 3 | [G2](#g2-failing-check-log-tail-in-the-drawer--read-m) Failing check log tail | read | M | Answers "why is it red" without leaving the pane. |
| 4 | [G3](#g3-review-threads-in-the-drawer--read-m-l-with-diff-marks) Unresolved review threads (list first, diff marks later) | read | M | The deck says "2 comments" today and nothing more. |
| 5 | [G4](#g4-re-run-failed-jobs--write-s) Re-run failed jobs | **write** | S | The first write: small, idempotent, behind a confirm and a setting. |

Next in line:

- L2, drift (cheap once G1 and L1 exist);
- G6, main's CI in the header;
- G5, merge or auto-merge (needs decision 1);
- L3, the issue view.

Suggested order of PRs:

1. G1 (with the GitHub reader);
2. L1;
3. G2;
4. G3 (list);
5. G4.

Each is one PR with a CHANGELOG line, as usual.

## 6. Decisions for the user

1. **Writes at all?** Recommendation: yes, but only these:
   - G4 re-run and G5 merge on GitHub; L6 state change on Linear, if ever.
   - Each behind a `y/n` confirm that names the target.
   - Each behind a config switch, off by default (`github_writes = false`,
     or one key per action).
   - Never bound to a key that also opens or navigates.
   - No Linear writes until L2 shows drift is a real problem.
2. **How the deck reaches GitHub.** Recommendation: shell out to `gh`
   (option A in [§2](#github-reuse-gh)). The deck never sees a token,
   matches herdr-projects, and handles enterprise hosts. The alternative is
   Go HTTP with `gh auth token`, kept in memory.
3. **Linear auth.** Recommendation: keep the personal key (env or
   `linear_api_key_command`). OAuth with PKCE only if the deck is shared
   with others or needs to act as an app.
4. **Who performs writes.** Recommendation: the deck calls the API itself
   for re-runs. For merges, choose between the deck (G5) and typing the
   request to the coordinator (X3).
5. **Where new views live:**
   - G2's log and G3's threads in the *PR* section, with full views like
     `r`;
   - L3 as a full drawer view;
   - G7 and L5 inside the `p` picker rather than new top-level views;
   - G6 in the header.

   Is the `p` picker the right home for things outside the project?
6. **Two sources for PR state.** The deck's own G1 fetch is newer than
   ticker.json, which may say otherwise for up to 2 minutes.
   Recommendation: the newer timestamp wins per field, and `checked Ns ago`
   shows which.
7. **Polling budget.** Recommendation:
   - open PRs once a minute;
   - main's CI every 2–5 minutes;
   - search every 5 minutes;
   - Linear issues every 3 minutes, as now;
   - heavy data only on demand.

## 7. Sources

Linear:

- GraphQL basics, `issue(id:)` with identifiers, personal key header:
  <https://linear.app/developers/graphql>
- OAuth 2.0 (scopes, PKCE, 24 h tokens, refresh, `actor=app`):
  <https://linear.app/developers/oauth-2-0-authentication>
- Rate limits and complexity: <https://linear.app/developers/rate-limiting>
- Pagination (`first`/`after`, default 50):
  <https://linear.app/developers/pagination>
- Webhooks (public HTTPS URL, retries):
  <https://linear.app/developers/webhooks>
- GitHub integration (state automation, magic words):
  <https://linear.app/docs/github>
- Schema (`issueVcsBranchSearch`, `Issue.branchName`, `WorkflowState.type`
  incl. `duplicate`, `Project.progress`, `Cycle.progress`, `issueUpdate`,
  `attachmentLinkGitHubPR`):
  <https://github.com/linear/linear/blob/master/packages/sdk/src/schema.graphql>

GitHub:

- Check runs: <https://docs.github.com/en/rest/checks/runs>
- Job logs (302 to plain text, 1 min) and jobs:
  <https://docs.github.com/en/rest/actions/workflow-jobs>
- Workflow runs, re-run failed jobs:
  <https://docs.github.com/en/rest/actions/workflow-runs>
- Reviews: <https://docs.github.com/en/rest/pulls/reviews>
- Review comments: <https://docs.github.com/en/rest/pulls/comments>
- Review requests: <https://docs.github.com/en/rest/pulls/review-requests>
- Merge a PR: <https://docs.github.com/en/rest/pulls/pulls>
- Notifications (Last-Modified, 304, X-Poll-Interval, scopes):
  <https://docs.github.com/en/rest/activity/notifications>
- Releases: <https://docs.github.com/en/rest/releases/releases>
- GraphQL schema (`reviewThreads`, `MergeStateStatus`,
  `enablePullRequestAutoMerge`, `markPullRequestReadyForReview`):
  <https://docs.github.com/public/fpt/schema.docs.graphql>
- GraphQL limits:
  <https://docs.github.com/en/graphql/overview/rate-limits-and-query-limits-for-the-graphql-api>
- REST limits and conditional requests:
  <https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api>,
  <https://docs.github.com/en/rest/using-the-rest-api/best-practices-for-using-the-rest-api>
- Search qualifiers (`review-requested:`):
  <https://docs.github.com/en/search-github/searching-on-github/searching-issues-and-pull-requests>
- OAuth scopes: <https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/scopes-for-oauth-apps>
- gh manual: [`gh auth token`](https://cli.github.com/manual/gh_auth_token),
  [`gh auth login`](https://cli.github.com/manual/gh_auth_login),
  [`gh pr checks`](https://cli.github.com/manual/gh_pr_checks),
  [`gh pr view`](https://cli.github.com/manual/gh_pr_view),
  [`gh pr merge`](https://cli.github.com/manual/gh_pr_merge),
  [`gh pr ready`](https://cli.github.com/manual/gh_pr_ready),
  [`gh run view`](https://cli.github.com/manual/gh_run_view),
  [`gh run rerun`](https://cli.github.com/manual/gh_run_rerun),
  [`gh api`](https://cli.github.com/manual/gh_api),
  [`gh search prs`](https://cli.github.com/manual/gh_search_prs),
  [`gh status`](https://cli.github.com/manual/gh_status)

Checked by hand on 2026-10-03, read-only, with the user's `gh` login
against FredricW/herdr-deck:

- token scopes (`gh auth status`, `X-OAuth-Scopes`);
- `/notifications` answering 200 with `X-Poll-Interval: 60`;
- a job log returned as plain text;
- the search limit of 30 and the GraphQL limit of 5,000 (`gh api
  rate_limit`);
- the cost of 1 for a `review-requested:@me` search;
- the rollup's `detailsUrl` holding run and job IDs.

Linear was not called (no `LINEAR_API_KEY` in the environment).
herdr-projects' PR polling: `src/pr.rs` in the installed plugin.
