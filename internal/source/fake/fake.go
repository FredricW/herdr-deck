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
				ID: "t-0001", Title: "Subscriptions /admin/plans", Status: deck.StatusUnknown,
				StateLine: "idle", PaneID: "w1Y:p1", Branch: "hp/admin-rebuild/t-0001-subscriptions",
				// No manifest port: herdr's workspace token is the fallback.
				PortToken:  14437,
				DevServers: []deck.DevServer{{Name: "port", Port: 14437, Fallback: true}},
				Links:      []deck.Link{{Kind: deck.LinkLocalhost, Label: "~:14437", URL: "http://localhost:14437", Down: true}},
			},
			{
				ID: "t-0002", Title: "Members /admin/users", Status: deck.StatusNeedsYou,
				StateLine: "needs you · ~95%", Activity: "Waiting for you", PaneID: "w1Z:p1",
				Worktree: "/src/worktrees/t-0002", Branch: "hp/admin-rebuild/t-0002-members-admin-users",
				Next:    []string{"Approve phase 1 (ABC-1256 overview)", "Say whether to start ABC-1257 and ABC-1250"},
				Report:  "## Report\n\nPhase 1 is done: layout and overview.\n\n## Next\n\n- Approve phase 1 (ABC-1256 overview)\n",
				DevNote: "not started: no .dev/t-0002/state.json",
			},
			{
				ID: "t-0003", Title: "Templates /templates", Status: deck.StatusWorking,
				StateLine: "working", Activity: "Writing tests", PaneID: "w20:p1",
				Branch: "hp/admin-rebuild/t-0003-templates",
				DevServers: []deck.DevServer{
					{Name: "frontend", Port: 5181, Running: true},
					{Name: "api", Port: 8011, Running: true},
					{Name: "pg", Port: 5441, Running: true},
				},
				Links: []deck.Link{{Kind: deck.LinkLocalhost, Label: "Frontend", URL: "http://localhost:5181"}},
			},
			{
				ID: "t-0004", Title: "Summary select documents", Status: deck.StatusReview,
				StateLine: "ready for review", PaneID: "w21:p1", Branch: "hp/admin-rebuild/t-0004-summary",
				PR: &deck.PullRequest{
					URL: "https://github.com/acme/webshop/pull/2320", Number: 2320,
					State: "OPEN", Review: "REVIEW_REQUIRED", Comments: 2,
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
				Summary: "t-0002 is now Waiting on you", Created: now.Add(-3 * time.Minute),
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
