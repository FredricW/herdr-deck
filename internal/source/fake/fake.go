// Package fake provides a fixed Snapshot shaped like the design mockups
// (docs/design/d-list-detail.md), for UI tests and `herdr-deck --fake`.
package fake

import (
	"context"
	"time"

	"github.com/FredricW/herdr-deck/internal/deck"
)

func linear(id string) deck.Link {
	return deck.Link{Kind: deck.LinkLinear, Label: id, URL: "https://linear.app/acme/issue/" + id}
}

// issue gives a Linear link the status Linear's API would.
func issue(l deck.Link, state, stateType string) deck.Link {
	l.Issue = &deck.Issue{State: state, StateType: stateType}
	return l
}

func figma(node string) deck.Link {
	return deck.Link{Kind: deck.LinkFigma, Label: node, URL: "https://www.figma.com/design/abc123/Document-select?node-id=" + node}
}

func pct(n int) *int { return &n }

// ev is a Log event at now minus ago.
func ev(now time.Time, ago time.Duration, kind deck.EventKind, text, detail, source string) deck.LogEvent {
	return deck.LogEvent{At: now.Add(-ago), Kind: kind, Text: text, Detail: detail, Source: source}
}

// Snapshot returns the mockups' sample project, Admin rebuild, in the state
// where t-0002 needs the user, for the given project slug. now anchors the
// inbox timestamps.
func Snapshot(slug string, now time.Time) deck.Snapshot {
	return deck.Snapshot{
		Project: deck.Project{
			Slug:  slug,
			Name:  "Admin rebuild",
			Goal:  "Complete the Linear project 'Admin rebuild' page by page.",
			Repos: []string{"/src/webshop"},
			Dir:   "/projects/" + slug,
		},
		ReadAt: now,
		Notes:  []string{"dev servers: billing has no .herdr-deck/dev.json; only herdr's port token, if any"},
		TaskLists: []deck.TaskList{
			{
				Name: "In progress",
				Note: "No PR until the user gives a go-ahead.",
				Tasks: []deck.Task{
					{
						Title: "Users page", Threads: []string{"t-0002"},
						Notes: "Phase 1 = layout + overview (ABC-1256); report on 1257/1250 before starting them.",
						Links: []deck.Link{issue(linear("ABC-1246"), "Done", "completed"), issue(linear("ABC-1256"), "In Progress", "started"), issue(linear("ABC-1257"), "Todo", "unstarted"), linear("ABC-1250")},
					},
					{Title: "Templates page", Threads: []string{"t-0003"}, Links: []deck.Link{
						linear("ABC-1051"),
						{Kind: deck.LinkFigma, Label: "Templates", URL: "https://www.figma.com/design/def456/Templates"},
					}},
					{Title: "Document select for summary", Threads: []string{"t-0004"}, Links: []deck.Link{
						linear("ABC-1191"), figma("598-48083"), figma("1138-88367"), figma("635-76529"),
					}},
				},
			},
			{
				Name: "On hold until Monday 2026-10-05",
				Tasks: []deck.Task{
					{Title: "Subscriptions list", Threads: []string{"t-0001"}, Links: []deck.Link{
						linear("ABC-1472"),
						{Kind: deck.LinkNotion, Label: "Findings", URL: "https://www.notion.so/acme/Findings-0123456789abcdef0123456789abcdef"},
					}},
				},
			},
			{
				Name: "Backlog",
				Tasks: []deck.Task{
					{Title: "Settings page", Notes: "The design shows a dialog. Not cleared yet."},
					{Title: "Home page", Notes: "Depends on Monday's decision."},
					{Title: "Snippets page", Links: []deck.Link{linear("ABC-1251")}},
				},
			},
		},
		Threads: []deck.Thread{
			{
				ID: "t-0001", Title: "Subscriptions /admin/plans", Status: deck.StatusUnknown, Group: "idle",
				StateLine: "idle", PaneID: "w1Y:p1", Repo: "/src/webshop", Branch: "hp/admin-rebuild/t-0001-subscriptions", Base: "origin/main",
				Log: []deck.LogEvent{
					ev(now, 26*time.Hour, deck.EventLaunched, "launched in pane w1Y:p1", "", "threads/t-0001.toml"),
					ev(now, 26*time.Hour+time.Minute, deck.EventCreated, "created from origin/main", "", "threads/t-0001.toml"),
				},
				// No manifest port: herdr's workspace token is the fallback.
				PortToken:  14437,
				DevServers: []deck.DevServer{{Name: "port", Port: 14437, Fallback: true}},
				Links:      []deck.Link{{Kind: deck.LinkLocalhost, Label: "~:14437", URL: "http://localhost:14437", Down: true}},
			},
			{
				ID: "t-0002", Title: "Members /admin/users", Status: deck.StatusNeedsYou, Group: "waiting-on-you",
				StateLine: "needs you · ~95%", Activity: "Waiting for you", PaneID: "w1Z:p1", Percent: pct(95),
				Worktree: "/src/worktrees/t-0002", Repo: "/src/webshop", Branch: "hp/admin-rebuild/t-0002-members-admin-users", Base: "origin/main",
				Created: now.Add(-22*time.Hour - 31*time.Minute), LaunchedAt: now.Add(-22*time.Hour - 30*time.Minute),
				BriefSeenAt: now.Add(-22*time.Hour - 29*time.Minute), LastReportChange: now.Add(-5 * time.Minute),
				Log: []deck.LogEvent{
					func() deck.LogEvent {
						e := ev(now, 3*time.Minute, deck.EventWaiting, "waiting on you", "pane w1Z:p1", "inbox/20261002T143012Z-thread-state-t-0002-7.md")
						e.Unhandled = true
						return e
					}(),
					ev(now, 5*time.Minute, deck.EventReport, "new report", "", "inbox/done/20261002T143600Z-thread-state-t-0002-6.md"),
					ev(now, 61*time.Minute, deck.EventReport, "new report", "", "inbox/done/20261002T134000Z-thread-state-t-0002-5.md"),
					ev(now, 5*time.Hour+29*time.Minute, deck.EventBlocked, "blocked on a prompt", "pane w1Z:p1", "inbox/done/20261002T091200Z-thread-state-t-0002-4.md"),
					ev(now, 21*time.Hour+11*time.Minute, deck.EventReport, "new report", "", "inbox/done/20261001T173000Z-thread-state-t-0002-3.md"),
					ev(now, 22*time.Hour+30*time.Minute, deck.EventLaunched, "launched in pane w1Z:p1", "", "threads/t-0002.toml"),
					ev(now, 22*time.Hour+31*time.Minute, deck.EventCreated, "created from origin/main", "", "threads/t-0002.toml"),
				},
				Next:    []string{"Approve phase 1 (ABC-1256 overview)", "Say whether to start ABC-1257 and ABC-1250"},
				Report:  "## Report\n\nPhase 1 is done: layout and overview.\n\n## Next\n\n- Approve phase 1 (ABC-1256 overview)\n",
				DevNote: "not started: no .dev/t-0002/state.json",
			},
			{
				ID: "t-0003", Title: "Templates /templates", Status: deck.StatusWorking, Group: "working",
				StateLine: "working · ~40%", Activity: "Writing tests", PaneID: "w20:p1", Percent: pct(40),
				Repo: "/src/webshop", Branch: "hp/admin-rebuild/t-0003-templates", Base: "origin/main",
				Log: []deck.LogEvent{
					ev(now, 40*time.Minute, deck.EventReport, "new report", "", "inbox/done/20261002T140100Z-thread-state-t-0003-3.md"),
					ev(now, 3*time.Hour, deck.EventLaunched, "launched in pane w20:p1", "", "threads/t-0003.toml"),
					ev(now, 3*time.Hour+time.Minute, deck.EventCreated, "created from origin/main", "", "threads/t-0003.toml"),
				},
				DevServers: []deck.DevServer{
					{Name: "frontend", Port: 5181, Running: true},
					{Name: "api", Port: 8011, Running: true},
					{Name: "pg", Port: 5441, Running: true},
				},
				Links: []deck.Link{{Kind: deck.LinkLocalhost, Label: "Frontend", URL: "http://localhost:5181"}},
			},
			{
				ID: "t-0004", Title: "Summary select documents", Status: deck.StatusReview, Group: "ready-for-review",
				StateLine: "ready for review", PaneID: "w21:p1", Repo: "/src/webshop", Branch: "hp/admin-rebuild/t-0004-summary", Base: "origin/main",
				PR: &deck.PullRequest{
					URL: "https://github.com/acme/webshop/pull/2320", Number: 2320,
					State: "OPEN", Review: "REVIEW_REQUIRED", Comments: 2, Commenters: []string{"sam", "alex"},
					CheckedAt: now.Add(-time.Minute),
				},
				Log: []deck.LogEvent{
					ev(now, 25*time.Minute, deck.EventPRUpdated, "updated", "open · 2 comments", "inbox/done/20261002T141600Z-pr-t-0004-9.md"),
					ev(now, 3*time.Hour+38*time.Minute, deck.EventPROpened, "opened", "", "inbox/done/20261002T110300Z-pr-t-0004-8.md"),
					ev(now, 4*time.Hour+29*time.Minute, deck.EventWaiting, "waiting on you", "pane w21:p1", "inbox/done/20261002T101200Z-thread-state-t-0004-5.md"),
					ev(now, 5*time.Hour+11*time.Minute, deck.EventRoutine, "prompted by routine pr-followup", "checks-failed", "inbox/done/20261002T093000Z-routine-pr-followup-4.md"),
					ev(now, 5*time.Hour+46*time.Minute, deck.EventLaunched, "launched in pane w21:p1", "", "threads/t-0004.toml"),
					ev(now, 5*time.Hour+47*time.Minute, deck.EventCreated, "created from origin/main", "", "threads/t-0004.toml"),
				},
				Links: []deck.Link{
					{Kind: deck.LinkGitHub, Label: "PR #2320", URL: "https://github.com/acme/webshop/pull/2320"},
					{Kind: deck.LinkLocalhost, Label: "Frontend", URL: "http://localhost:5174"},
					{Kind: deck.LinkLocalhost, Label: "API docs", URL: "http://localhost:8002/docs", Down: true},
				},
				DevServers: []deck.DevServer{
					{Name: "frontend", Port: 5174, Running: true},
					{Name: "api", Port: 8002},
				},
			},
		},
		Inbox: []deck.InboxItem{
			{
				ID: "20261002T143012Z-thread-state-t-0002-7", Kind: "thread-state", Thread: "t-0002", Subject: "t-0002",
				Summary: "t-0002 is now Waiting on you", Event: "waiting on you", Created: now.Add(-3 * time.Minute),
			},
			{
				ID: "20261002T142200Z-routine-pr-followup-6", Kind: "routine", Subject: "pr-followup",
				Summary: "routine `pr-followup` found nothing to follow up", Event: "ran", Created: now.Add(-19 * time.Minute),
			},
		},
	}
}

// Diff is the Files section's sample: what t-0002's worktree changed
// against origin/main. Other threads have no changes.
func Diff(_ context.Context, t deck.Thread) deck.Diff {
	if t.ID != "t-0002" {
		return deck.Diff{Base: "origin/main", Note: "no changes"}
	}
	return deck.Diff{
		Base:      "origin/main",
		MergeBase: "4f1c2a9",
		Files: []deck.DiffFile{
			{Path: "docs/users-page.md", Change: deck.ChangeAdded, Added: 21, Untracked: true},
			{Path: "public/avatar-placeholder.png", Change: deck.ChangeAdded, Binary: true},
			{Path: "src/admin/users/UsersList.tsx", Change: deck.ChangeDeleted, Deleted: 57},
			{Path: "src/admin/users/UsersPage.tsx", Added: 142, Deleted: 18},
			{Path: "src/admin/users/UsersTable.tsx", Change: deck.ChangeAdded, Added: 96},
			{Path: "src/admin/users/overview.ts", OldPath: "src/admin/users/stats.ts", Change: deck.ChangeRenamed, Added: 12, Deleted: 9},
			{Path: "src/admin/users/users.test.tsx", Change: deck.ChangeAdded, Added: 64},
			{Path: "src/api/users.ts", Added: 18, Deleted: 3},
		},
	}
}

// Projects returns the sample projects root for the project picker, with
// snap as its own project: Billing export has a thread waiting on the user
// and an inbox item, Docs site an inbox item, Search spike is paused and
// Mobile onboarding archived. now anchors the times.
func Projects(snap deck.Snapshot, now time.Time) []deck.ProjectInfo {
	resolved := func(id, title string) deck.Thread {
		return deck.Thread{ID: id, Title: title, Status: deck.StatusDone, StateLine: "resolved · merged"}
	}
	return []deck.ProjectInfo{
		{Project: snap.Project, Status: "active", Threads: snap.Threads, Inbox: snap.Inbox},
		{
			Project: deck.Project{Slug: "billing-export", Name: "Billing export", Dir: "/projects/billing-export", PaneID: "w4J:p1"},
			Status:  "active",
			Threads: []deck.Thread{
				{ID: "t-0002", Title: "Invoice PDF fonts", Status: deck.StatusReview, PaneID: "w4L:p1", Pane: &deck.Pane{ID: "w4L:p1", Agent: "claude", AgentStatus: "idle"}},
				{
					ID: "t-0003", Title: "CSV export job", Status: deck.StatusNeedsYou, StateLine: "needs you · blocked on a prompt", PaneID: "w4K:p3",
					Pane:    &deck.Pane{ID: "w4K:p3", Agent: "claude", AgentStatus: "blocked", Since: now.Add(-2 * time.Minute)},
					Changed: now.Add(-9 * time.Minute),
				},
				{ID: "t-0004", Title: "Ledger totals fix", Status: deck.StatusReview, PaneID: "w4M:p1"},
			},
			Inbox: []deck.InboxItem{{
				ID: "20261002T143300Z-thread-state-t-0002-3", Kind: "thread-state", Thread: "t-0002", Subject: "t-0002",
				Summary: "t-0002 has a failing check", Created: now.Add(-8 * time.Minute),
			}},
		},
		{
			Project: deck.Project{Slug: "docs-site", Name: "Docs site", Dir: "/projects/docs-site"},
			Status:  "active",
			Threads: []deck.Thread{
				{ID: "t-0001", Title: "Search page", Status: deck.StatusUnknown, StateLine: "idle", PaneID: "w5A:p1"},
				resolved("t-0002", "Sidebar order"),
			},
			Inbox: []deck.InboxItem{{
				ID: "20261002T120500Z-note-2", Kind: "note", Summary: "Decide the docs domain before launch", Created: now.Add(-2*time.Hour - 36*time.Minute),
			}},
		},
		{
			Project: deck.Project{Slug: "search-spike", Name: "Search spike", Dir: "/projects/search-spike"},
			Status:  "paused",
		},
		{
			Project: deck.Project{Slug: "mobile-onboarding", Name: "Mobile onboarding", Dir: "/projects/mobile-onboarding"},
			Status:  "archived",
			Threads: []deck.Thread{
				resolved("t-0001", "Welcome screens"), resolved("t-0002", "Push permission"), resolved("t-0003", "Sign-in with email"),
				resolved("t-0004", "Profile photo"), resolved("t-0005", "Analytics events"), resolved("t-0006", "Store listing"),
			},
		},
	}
}
