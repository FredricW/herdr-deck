package tasks

import (
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/FredricW/herdr-deck/internal/deck"
)

var (
	// anyURL finds URLs in prose and Markdown. Brackets, parentheses and
	// quotes end a URL so `[text](url)` and `<url>` come out clean.
	anyURL = regexp.MustCompile("https?://[^\\s<>()\\[\\]\"'`]+")

	// linearID is a Linear issue ID: a 2-6 character team key, a dash and a
	// number, the same team-key rule as ticketInBranch. The leading
	// group stands in for a lookbehind (RE2 has none): the ID must not follow
	// a letter, digit, underscore or dash, so "P-ABC-49" is not ABC-49. The
	// trailing \b keeps the whole number, so ABC-1100 never reads as ABC-110.
	linearID = regexp.MustCompile(`(?:^|[^A-Za-z0-9_-])([A-Z][A-Z0-9]{1,5}-\d+)\b`)

	linearURL  = regexp.MustCompile(`^https?://linear\.app/([^/]+)/issue/([A-Z][A-Z0-9]{1,5}-\d+)`)
	githubURL  = regexp.MustCompile(`^https?://(?:www\.)?github\.com/([\w.-]+)/([\w.-]+)/(pull|issues)/(\d+)`)
	figmaURL   = regexp.MustCompile(`^https://(?:www\.)?figma\.com/(?:design|file|proto|board)/`)
	notionHost = regexp.MustCompile(`^(?:[\w-]+\.)?notion\.(?:so|site)$`)
	notionID   = regexp.MustCompile(`-?[0-9a-f]{32}$`)
)

// notTeams are prefixes that look like Linear team keys but name standards,
// algorithms and runtimes: UTF-8, SHA-256, ISO-8601, node-18.
var notTeams = map[string]bool{
	"AES": true, "CVE": true, "ECMA": true, "ES": true, "GO": true,
	"HTTP": true, "IPV": true, "ISO": true, "MD": true, "NODE": true,
	"PEP": true, "PG": true, "RFC": true, "RSA": true, "SHA": true,
	"SSL": true, "TLS": true, "UTF": true, "WIN": true, "X": true,
}

// isTicket reports whether id, such as "ABC-12", names a Linear issue rather
// than a standard like UTF-8.
func isTicket(id string) bool {
	team, _, _ := strings.Cut(id, "-")
	return !notTeams[strings.ToUpper(team)]
}

// Scrape returns the Linear, Figma, Notion and GitHub links in text, in the
// order they appear, without duplicates. Bare Linear IDs link into workspace;
// with no workspace they have no URL.
func Scrape(text, workspace string) []deck.Link {
	type found struct {
		pos  int
		link deck.Link
	}
	var all []found

	// URLs first, then blank them out so IDs inside them (a Notion slug,
	// a Linear URL's own ID) are not read a second time as bare IDs.
	masked := []byte(text)
	for _, loc := range anyURL.FindAllStringIndex(text, -1) {
		raw := trimURL(text[loc[0]:loc[1]])
		if l, ok := classify(raw); ok {
			all = append(all, found{loc[0], l})
		}
		for i := loc[0]; i < loc[1]; i++ {
			masked[i] = ' '
		}
	}
	for _, m := range linearID.FindAllSubmatchIndex(masked, -1) {
		id := string(masked[m[2]:m[3]])
		if !isTicket(id) {
			continue
		}
		all = append(all, found{m[2], linearLink(workspace, id)})
	}

	sort.SliceStable(all, func(i, j int) bool { return all[i].pos < all[j].pos })
	links := make([]deck.Link, 0, len(all))
	for _, f := range all {
		links = appendLink(links, f.link, workspace)
	}
	return links
}

// appendLink adds l unless an equal link is already there. Linear links are
// equal when their IDs are, whatever workspace their URL names; an explicit
// Linear URL then replaces one guessed from a bare ID in workspace.
func appendLink(links []deck.Link, l deck.Link, workspace string) []deck.Link {
	for i, have := range links {
		if have.Kind != l.Kind {
			continue
		}
		if l.Kind == deck.LinkLinear && have.Label == l.Label {
			if have.URL == linearLink(workspace, have.Label).URL {
				links[i] = l
			}
			return links
		}
		if have.Same(l) {
			return links
		}
	}
	return append(links, l)
}

// trimURL drops trailing punctuation that belongs to the prose around a URL.
func trimURL(u string) string {
	return strings.TrimRight(u, ".,;:!?*_")
}

// linearLink links id into workspace; with no workspace the link has no URL.
func linearLink(workspace, id string) deck.Link {
	l := deck.Link{Kind: deck.LinkLinear, Label: id}
	if workspace != "" {
		l.URL = "https://linear.app/" + workspace + "/issue/" + id
	}
	return l
}

// classify turns a URL into a link of a kind the deck knows, or reports false.
func classify(raw string) (deck.Link, bool) {
	if m := linearURL.FindStringSubmatch(raw); m != nil {
		return linearLink(m[1], m[2]), true
	}
	if m := githubURL.FindStringSubmatch(raw); m != nil {
		label := "PR #" + m[4]
		if m[3] == "issues" {
			label = "issue #" + m[4]
		}
		return deck.Link{Kind: deck.LinkGitHub, Label: label, URL: m[0]}, true
	}
	if figmaURL.MatchString(raw) {
		return deck.Link{Kind: deck.LinkFigma, Label: figmaLabel(raw), URL: raw}, true
	}
	if u, err := url.Parse(raw); err == nil && notionHost.MatchString(u.Host) {
		return deck.Link{Kind: deck.LinkNotion, Label: notionLabel(u), URL: raw}, true
	}
	return deck.Link{}, false
}

// figmaLabel names a Figma link by the file name in its path and its node:
// "Design · node 380-32562".
func figmaLabel(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "Figma"
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	name := "Figma"
	if len(parts) > 2 && parts[2] != "" {
		if n, err := url.PathUnescape(parts[2]); err == nil {
			name = strings.ReplaceAll(n, "-", " ")
		}
	}
	if node := u.Query().Get("node-id"); node != "" {
		return name + " · node " + node
	}
	return name
}

// notionLabel names a Notion link by its page slug without the page ID.
func notionLabel(u *url.URL) string {
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	slug := notionID.ReplaceAllString(parts[len(parts)-1], "")
	if slug == "" {
		return "Notion"
	}
	return strings.ReplaceAll(slug, "-", " ")
}

// ticketInBranch finds a Linear ID in a branch name: a 2-6 character
// team key after the start or a non-alphanumeric, so "sync/release-1.1.0"
// does not read as RELEASE-1. The number must not run on into a version
// ("node-18.x", "go-1.22").
var ticketInBranch = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])([a-z][a-z0-9]{1,5})-(\d+)(?:[^0-9.]|$)`)

// TicketFromBranch returns the Linear ID a branch name refers to, such as
// ABC-1246 for "dev/abc-1246-users-page", or "".
func TicketFromBranch(branch string) string {
	for _, m := range ticketInBranch.FindAllStringSubmatch(branch, -1) {
		if id := strings.ToUpper(m[1]) + "-" + m[2]; isTicket(id) {
			return id
		}
	}
	return ""
}
