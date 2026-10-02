package tasks

import (
	"reflect"
	"testing"

	"github.com/FredricW/herdr-deck/internal/deck"
)

func load(t *testing.T, dir string) File {
	t.Helper()
	f, err := Load("testdata/"+dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func names(lists []deck.TaskList) []string {
	var out []string
	for _, l := range lists {
		out = append(out, l.Name)
	}
	return out
}

func labels(links []deck.Link) []string {
	var out []string
	for _, l := range links {
		out = append(out, l.Label)
	}
	return out
}

func TestStrictFile(t *testing.T) {
	f := load(t, "strict")
	if got, want := names(f.Lists), []string{"v1", "Backlog"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("lists = %q, want %q", got, want)
	}
	if n := len(f.Lists[0].Tasks); n != 6 {
		t.Fatalf("v1 has %d tasks, want 6", n)
	}
	m3 := f.Lists[0].Tasks[1]
	if m3.Title != "M3 Tasks and links" || m3.Owner != "claude" || m3.Done {
		t.Errorf("M3 = %+v", m3)
	}
	if !reflect.DeepEqual(m3.Threads, []string{"t-0004"}) {
		t.Errorf("M3 threads = %q", m3.Threads)
	}
	if m3.Notes == "" {
		t.Error("M3 lost its indented note")
	}
	// A thread id in a note only mentions that thread: it is not the task's.
	if m4 := f.Lists[0].Tasks[2]; m4.Title != "M4 Live UI" || m4.Threads != nil {
		t.Errorf("M4 = %+v", m4)
	}
}

func TestStrictLine(t *testing.T) {
	f := Parse("## Now\n\n- [x] Ship it [sam] · t-0007, t-0008\n  see ABC-12\n- [ ] Plain title\n", Options{LinearWorkspace: "acme"})
	got := f.Lists[0].Tasks
	if len(got) != 2 {
		t.Fatalf("tasks = %+v", got)
	}
	want := deck.Task{
		Title:   "Ship it",
		Done:    true,
		Owner:   "sam",
		Threads: []string{"t-0007", "t-0008"},
		Links:   []deck.Link{{Kind: deck.LinkLinear, Label: "ABC-12", URL: "https://linear.app/acme/issue/ABC-12"}},
		Notes:   "see ABC-12",
	}
	if !reflect.DeepEqual(got[0], want) {
		t.Errorf("got  %+v\nwant %+v", got[0], want)
	}
	if got[1].Title != "Plain title" || got[1].Done {
		t.Errorf("second = %+v", got[1])
	}
}

func TestLooseFile(t *testing.T) {
	f := load(t, "loose")
	want := []string{"In progress", "On hold until Monday 2026-10-05", "Backlog", "Elsewhere"}
	if got := names(f.Lists); !reflect.DeepEqual(got, want) {
		t.Fatalf("lists = %q, want %q", got, want)
	}
	if f.Note == "" {
		t.Error("preamble paragraphs lost")
	}
	inProgress := f.Lists[0]
	if inProgress.Note != "No PR until the user gives a go-ahead." {
		t.Errorf("list note = %q", inProgress.Note)
	}

	var titles []string
	for _, task := range inProgress.Tasks {
		titles = append(titles, task.Title)
	}
	if want := []string{"Users page", "Templates page", "Document select for summary"}; !reflect.DeepEqual(titles, want) {
		t.Errorf("titles = %q, want %q", titles, want)
	}

	users := inProgress.Tasks[0]
	if !reflect.DeepEqual(users.Threads, []string{"t-0002"}) || users.Owner != "" {
		t.Errorf("users = %+v", users)
	}
	// Links from the line first, then the thread's brief and report.
	wantLabels := []string{
		"ABC-1246", "ABC-1256",
		"Design", "Users page spec",
		"PR #2320", "Design · node 380-32562", "issue #77",
	}
	if got := labels(users.Links); !reflect.DeepEqual(got, wantLabels) {
		t.Errorf("users links = %q\nwant %q", got, wantLabels)
	}

	held := f.Lists[1].Tasks[0]
	if held.Title != "Subscriptions list page" || !reflect.DeepEqual(held.Threads, []string{"t-0001"}) {
		t.Errorf("held = %+v", held)
	}
	if got := labels(held.Links); !reflect.DeepEqual(got, []string{"ABC-1472"}) {
		t.Errorf("held links = %q", got)
	}

	backlog := f.Lists[2].Tasks
	if len(backlog) != 6 || backlog[0].Title != "Settings page" || backlog[0].Notes != "the design shows a dialog. Not cleared yet." {
		t.Errorf("backlog = %+v", backlog)
	}
}

func TestScrape(t *testing.T) {
	text := `ABC-110 and ABC-1100, not P-ABC-49 or abc-3 or t-0002.
Twice: ABC-110. https://linear.app/other/issue/ENG-7/slug
[Figma](https://www.figma.com/file/KEY/My-File?node-id=1-2), https://figma.com/community/x
https://acme.notion.site/abc0123456789abcdef0123456789abcdef https://github.com/o/r/pull/9/files.`
	want := []deck.Link{
		{Kind: deck.LinkLinear, Label: "ABC-110", URL: "https://linear.app/ws/issue/ABC-110"},
		{Kind: deck.LinkLinear, Label: "ABC-1100", URL: "https://linear.app/ws/issue/ABC-1100"},
		{Kind: deck.LinkLinear, Label: "ENG-7", URL: "https://linear.app/other/issue/ENG-7"},
		{Kind: deck.LinkFigma, Label: "My File · node 1-2", URL: "https://www.figma.com/file/KEY/My-File?node-id=1-2"},
		{Kind: deck.LinkNotion, Label: "abc", URL: "https://acme.notion.site/abc0123456789abcdef0123456789abcdef"},
		{Kind: deck.LinkGitHub, Label: "PR #9", URL: "https://github.com/o/r/pull/9"},
	}
	if got := Scrape(text, "ws"); !reflect.DeepEqual(got, want) {
		t.Errorf("got\n%+v\nwant\n%+v", got, want)
	}
}

// Without a workspace, bare IDs are still found but have no URL, and two
// different IDs stay two links.
func TestScrapeNoWorkspace(t *testing.T) {
	got := Scrape("ABC-1 and ABC-2, ABC-1 again", "")
	want := []deck.Link{{Kind: deck.LinkLinear, Label: "ABC-1"}, {Kind: deck.LinkLinear, Label: "ABC-2"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v", got)
	}
}

func TestDesktopURL(t *testing.T) {
	l := deck.Link{Kind: deck.LinkFigma, URL: "https://www.figma.com/design/K/D?node-id=1-2"}
	if got := l.DesktopURL(); got != "figma://design/K/D?node-id=1-2" {
		t.Errorf("DesktopURL = %q", got)
	}
	if got := (deck.Link{Kind: deck.LinkNotion, URL: "https://notion.so/x"}).DesktopURL(); got != "" {
		t.Errorf("non-Figma DesktopURL = %q", got)
	}
}

func TestLoadMissing(t *testing.T) {
	if _, err := Load("testdata/nowhere", Options{}); err == nil {
		t.Error("want an error for a missing TASKS.md")
	}
}

func TestTicketFromBranch(t *testing.T) {
	for branch, want := range map[string]string{
		"dev/abc-1246-users-page":       "ABC-1246",
		"ABC-110":                       "ABC-110",
		"feature/abc-1100":              "ABC-1100",
		"sync/release-1.1.0-to-main":    "",
		"renovate/node-18.x":            "",
		"deps/go-1.22":                  "",
		"hp/herdr-deck/t-0004-m3-tasks": "",
		"main":                          "",
	} {
		if got := TicketFromBranch(branch); got != want {
			t.Errorf("TicketFromBranch(%q) = %q, want %q", branch, got, want)
		}
	}
}

func TestScrapeSkipsStandards(t *testing.T) {
	if got := Scrape("Files are UTF-8, hashed with SHA-256, dates ISO-8601, see ABC-3", ""); !reflect.DeepEqual(labels(got), []string{"ABC-3"}) {
		t.Errorf("got %q", labels(got))
	}
}

func TestExplicitLinearURLWins(t *testing.T) {
	got := Scrape("[ACME-5](https://linear.app/acme/issue/ACME-5)", "")
	if len(got) != 1 || got[0].URL != "https://linear.app/acme/issue/ACME-5" {
		t.Errorf("got %+v", got)
	}
	// Across sources: the task line names the ID, the thread file has the URL.
	f := Parse("## L\n- Fix ACME-5: t-0001\n", Options{ThreadText: func(string) string {
		return "https://linear.app/acme/issue/ACME-5/slug"
	}})
	if l := f.Lists[0].Tasks[0].Links; len(l) != 1 || l[0].URL != "https://linear.app/acme/issue/ACME-5" {
		t.Errorf("links = %+v", l)
	}
}

func TestLooseTitles(t *testing.T) {
	for line, want := range map[string]deck.Task{
		"- Bump deps (2)":                  {Title: "Bump deps"},
		"- Use e.g. Redis for caching":     {Title: "Use e.g. Redis for caching"},
		"- Cache (see notes. later): it":   {Title: "Cache", Notes: "it"},
		"- [ ] · t-0003":                   {Title: "· t-0003", Threads: []string{"t-0003"}},
		"- [ ] Strict (wip) · t-0003":      {Title: "Strict", Owner: "wip", Threads: []string{"t-0003"}},
		"- Settings page: shows a dialog.": {Title: "Settings page", Notes: "shows a dialog."},
	} {
		got := Parse("## L\n"+line+"\n", Options{}).Lists[0].Tasks[0]
		got.Links = nil
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%q:\n got %+v\nwant %+v", line, got, want)
		}
	}
}

func TestPreambleBeforeLooseItem(t *testing.T) {
	f := Parse("# T\n\nIntro para.\n\n- loose item\n\n## Active\n- [ ] x\n", Options{})
	if f.Note != "Intro para." {
		t.Errorf("Note = %q", f.Note)
	}
	if len(f.Lists) != 2 || f.Lists[0].Note != "" || f.Lists[0].Tasks[0].Title != "loose item" {
		t.Errorf("lists = %+v", f.Lists)
	}
}

func TestOnlyOwningThreadsLink(t *testing.T) {
	text := map[string]string{
		"t-0001": "Fixes ABC-1.",
		"t-0002": "Fixes ABC-2.",
		"t-0003": "Fixes ABC-3.",
	}
	src := "## L\n" +
		"- [ ] Strict (sam) · t-0001\n  after t-0002 lands\n" +
		"- Loose: thread t-0002, with t-0003\n  see t-0001 for context\n" +
		"- [ ] Strict without suffix, thread t-0003\n"
	f := Parse(src, Options{ThreadText: func(id string) string { return text[id] }})
	for i, want := range []struct {
		threads, links []string
	}{
		{[]string{"t-0001"}, []string{"ABC-1"}},
		{[]string{"t-0002", "t-0003"}, []string{"ABC-2", "ABC-3"}},
		{[]string{"t-0003"}, []string{"ABC-3"}},
	} {
		task := f.Lists[0].Tasks[i]
		if !reflect.DeepEqual(task.Threads, want.threads) || !reflect.DeepEqual(labels(task.Links), want.links) {
			t.Errorf("task %d: threads %q links %q, want %q %q", i, task.Threads, labels(task.Links), want.threads, want.links)
		}
	}
}

func TestScrapeSkipsCodeAndRemember(t *testing.T) {
	report := "PR: https://github.com/acme/webshop/pull/9\n\n" +
		"## Report\n\n" +
		"Fixed ABC-7. A bare `ABC-12` no longer doubles, and ``ABC-13 `x` `` neither.\n" +
		"An unclosed ` keeps ABC-8.\n\n" +
		"```go\nid := \"ABC-14\"\n```\n\n" +
		"~~~\nABC-15\n~~~\n\n" +
		"## Remember\n\n" +
		"- IDs right after a dash are skipped, so P-ABC-49 is not ABC-49; ABC-110 is not ABC-1100.\n" +
		"- Docs: https://linear.app/acme/issue/ABC-16\n\n" +
		"### Detail\n\nABC-17 here too.\n\n" +
		"## Next\n\n- Ship ABC-9\n"
	got := labels(Scrape(report, "acme"))
	want := []string{"PR #9", "ABC-7", "ABC-8", "ABC-16", "ABC-9"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

// A brief in the shape herdr-projects writes: the task named no ticket, but
// the copied project memory is full of example IDs in code and Remember-like
// prose. Only the task's own ticket links.
func TestScrapeBriefShape(t *testing.T) {
	brief := "Fix the login page for ABC-301.\n\n" +
		"# Project memory\n\n" +
		"- Test fixtures use made-up tickets like `ABC-123`.\n\n" +
		"## Remember\n\n- The scanner must skip `P-ABC-49` and keep ABC-110 apart from ABC-1100.\n"
	if got := labels(Scrape(brief, "")); !reflect.DeepEqual(got, []string{"ABC-301"}) {
		t.Errorf("got %q", got)
	}
}
