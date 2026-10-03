package projects

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/FredricW/herdr-deck/internal/deck"
)

func TestThreadFieldsForTheDrawer(t *testing.T) {
	snap := reader("admin-rebuild", jsonRun(t)).Read(context.Background())
	byID := map[string]deck.Thread{}
	for _, th := range snap.Threads {
		byID[th.ID] = th
	}
	if p := byID["t-0001"].Percent; p != nil {
		t.Errorf("t-0001 percent = %d, want nil (null in the JSON)", *p)
	}
	if p := byID["t-0002"].Percent; p == nil || *p != 95 {
		t.Errorf("t-0002 percent = %v, want 95", p)
	}
	t1 := byID["t-0001"]
	if want := time.Date(2026, 10, 2, 11, 13, 18, 0, time.UTC); !t1.Created.Equal(want) {
		t.Errorf("t-0001 created = %v, want %v", t1.Created, want)
	}
	if t1.LaunchedAt.IsZero() || t1.BriefSeenAt.IsZero() || t1.Changed.IsZero() || t1.LastReportChange.IsZero() {
		t.Errorf("t-0001 timestamps missing: %+v", t1)
	}
	if t1.Group != "waiting-on-you" {
		t.Errorf("t-0001 group = %q", t1.Group)
	}
	// Its log: created, launched, the blocked and waiting items from
	// inbox/done/, newest first; the report change at 14:07:24 is the
	// waiting item's time, so it adds a report event of its own.
	var kinds []deck.EventKind
	for _, e := range t1.Log {
		kinds = append(kinds, e.Kind)
		if e.Unhandled {
			t.Errorf("handled item marked unhandled: %+v", e)
		}
	}
	want := []deck.EventKind{deck.EventWaiting, deck.EventReport, deck.EventBlocked, deck.EventLaunched, deck.EventCreated}
	if !reflect.DeepEqual(kinds, want) {
		t.Errorf("t-0001 log kinds = %v, want %v", kinds, want)
	}
	if e := t1.Log[0]; e.Detail != "pane w1Y:p1" || e.Source != "inbox/done/20261002T140724Z-thread-state-t-0001-2.md" {
		t.Errorf("waiting event = %+v", e)
	}
	if e := t1.Log[3]; e.Text != "launched in pane w1Y:p1" {
		t.Errorf("launched event = %+v", e)
	}
	if e := t1.Log[4]; e.Text != "created from origin/main" {
		t.Errorf("created event = %+v", e)
	}
}

func TestTickerCommentersAndLastCheck(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "p")
	write(t, filepath.Join(dir, ".state", "ticker.json"), `{"last_pr_check":"2026-10-02T15:02:05.795169Z","prs":{"t-0001":{"state":"OPEN","review_decision":"REVIEW_REQUIRED","failing_checks":["lint"],"comment_count":2,"commenters":["sam","alex"]}}}`)
	write(t, filepath.Join(dir, "threads", "t-0001.toml"), `id = "t-0001"
status = "open"
pr = "https://github.com/acme/webshop/pull/7"
`)
	snap := Reader{Root: root, Slug: "p", Run: failRun}.Read(context.Background())
	if len(snap.Threads) != 1 || snap.Threads[0].PR == nil {
		t.Fatalf("threads = %+v", snap.Threads)
	}
	pr := snap.Threads[0].PR
	if !reflect.DeepEqual(pr.Commenters, []string{"sam", "alex"}) {
		t.Errorf("commenters = %q", pr.Commenters)
	}
	if want := time.Date(2026, 10, 2, 15, 2, 5, 795169000, time.UTC); !pr.CheckedAt.Equal(want) {
		t.Errorf("checked = %v, want %v", pr.CheckedAt, want)
	}
}

func TestThreadLogFromInbox(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "p")
	write(t, filepath.Join(dir, "threads", "t-0004.toml"), `id = "t-0004"
status = "open"
base = "origin/main"
pane_id = "w21:p1"
created = "2026-10-02T08:54:00Z"
launched_at = "2026-10-02T08:55:00Z"
pr = "https://github.com/acme/webshop/pull/2320"
`)
	write(t, filepath.Join(dir, "threads", "t-0005.toml"), `id = "t-0005"
status = "open"
`)
	item := func(sub, name, kind, subject, created, event, summary string) {
		write(t, filepath.Join(dir, sub, name+".md"), "+++\nid = \""+name+"\"\nkind = \""+kind+"\"\nsubject = \""+subject+"\"\ncreated = \""+created+"\"\nsummary = '"+summary+"'\nevent = \""+event+"\"\n+++\n")
	}
	item("inbox/done", "20261002T093000Z-routine-pr-followup-1", "routine", "pr-followup", "2026-10-02T09:30:00Z", "prompted its thread", "routine `pr-followup` prompted t-0004 (agent was idle) about: checks-failed")
	item("inbox/done", "20261002T110300Z-pr-t-0004-2", "pr", "t-0004", "2026-10-02T11:03:00Z", "PR opened", `t-0004 "Summary": pull request state OPEN; 0 comment(s)`)
	item("inbox/done", "20261002T141600Z-pr-t-0004-3", "pr", "t-0004", "2026-10-02T14:16:00Z", "PR updated", `t-0004 "Summary": pull request state OPEN; 2 comment(s)`)
	item("inbox", "20261002T143900Z-pr-t-0004-4", "pr", "t-0004", "2026-10-02T14:39:00Z", "PR checks failing", `t-0004 "Summary": pull request state OPEN; failing checks: lint, test (ubuntu-latest); 2 comment(s)`)
	item("inbox/done", "20261002T150000Z-thread-state-t-0005-5", "thread-state", "t-0005", "2026-10-02T15:00:00Z", "resolved", `t-0005 "Other": resolved (merged): worktree removed`)
	item("inbox/done", "20261002T150100Z-space-w24-6", "space", "w24", "2026-10-02T15:01:00Z", "closed empty Space", "closed empty Space p (w24)")

	h := &History{}
	r := Reader{Root: root, Slug: "p", Run: failRun, History: h}
	snap := r.Read(context.Background())
	var t4, t5 deck.Thread
	for _, th := range snap.Threads {
		switch th.ID {
		case "t-0004":
			t4 = th
		case "t-0005":
			t5 = th
		}
	}
	type ev struct {
		kind         deck.EventKind
		text, detail string
		unhandled    bool
	}
	var got []ev
	for _, e := range t4.Log {
		got = append(got, ev{e.Kind, e.Text, e.Detail, e.Unhandled})
	}
	want := []ev{
		{deck.EventChecksFailing, "checks failing: lint, test (ubuntu-latest)", "", true},
		{deck.EventPRUpdated, "updated", "open · 2 comments", false},
		{deck.EventPROpened, "opened", "", false},
		{deck.EventRoutine, "prompted by routine pr-followup", "checks-failed", false},
		{deck.EventLaunched, "launched in pane w21:p1", "", false},
		{deck.EventCreated, "created from origin/main", "", false},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("t-0004 log:\n got %+v\nwant %+v", got, want)
	}
	if len(t5.Log) != 1 || t5.Log[0].Kind != deck.EventResolved || t5.Log[0].Detail != "merged" {
		t.Errorf("t-0005 log = %+v", t5.Log)
	}
	if len(snap.Inbox) != 1 || snap.Inbox[0].Event != "PR checks failing" {
		t.Errorf("inbox = %+v", snap.Inbox)
	}

	// Items are cached by path: a second read does not open them again,
	// so a file that became unreadable still shows.
	if len(h.items) != 5 {
		t.Errorf("cached %d items, want 5 (the space item is not read)", len(h.items))
	}
	if err := os.WriteFile(filepath.Join(dir, "inbox", "done", "20261002T110300Z-pr-t-0004-2.md"), []byte("garbage"), 0o644); err != nil {
		t.Fatal(err)
	}
	again := r.Read(context.Background())
	for _, th := range again.Threads {
		if th.ID == "t-0004" && len(th.Log) != len(t4.Log) {
			t.Errorf("second read: %d events, want %d", len(th.Log), len(t4.Log))
		}
	}
}

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}
