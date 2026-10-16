package markdown

import (
	"regexp"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

// Attr is a set of inline styles.
type Attr uint8

const (
	Bold Attr = 1 << iota
	Italic
	Code
	Strike
	Underline
)

// Span is a run of text with one set of inline styles. URL is set inside
// a link.
type Span struct {
	Text string
	Attr Attr
	URL  string
}

// markers are the bytes that can start inline markup; text without any
// of them (and without a URL) is a single plain span.
const markers = "*_`~[<\\:"

var (
	inlineMu    sync.Mutex
	inlineCache = map[string][]Span{}
)

// Inline parses one line of inline Markdown: *italic* and _italic_,
// **bold** and __bold__, `code`, ~~strikethrough~~, <u>underline</u> and
// [text](url) links, with backslash escapes; bare http(s) URLs and
// <url> autolinks are links too. Markers that open or close
// nothing stay as text, and so do markers inside a word, so snake_case and
// 2*3*4 read as written. Block syntax (headings, lists) is not parsed. The
// result is cached, since list rows render on every frame.
func Inline(src string) []Span {
	if !strings.ContainsAny(src, markers) {
		if src == "" {
			return nil
		}
		return []Span{{Text: src}}
	}
	inlineMu.Lock()
	defer inlineMu.Unlock()
	if spans, ok := inlineCache[src]; ok {
		return spans
	}
	var p inlineParser
	p.parse(src, 0, "")
	p.flush()
	if len(inlineCache) >= maxCached {
		inlineCache = map[string][]Span{}
	}
	inlineCache[src] = p.out
	return p.out
}

// block matches a line that starts with block syntax: a heading, quote,
// list item, fence, table row or rule.
var block = regexp.MustCompile("^\\s*(?:#{1,6}(?:\\s|$)|>|[-*+](?:\\s|$)|\\d{1,9}[.)](?:\\s|$)|```|~~~|\\||(?:[-*_]\\s*){3,}$)")

// IsInline reports whether line is inline Markdown only, which Inline can
// show in full; a line with block syntax needs Render.
func IsInline(line string) bool {
	return !block.MatchString(line)
}

type inlineParser struct {
	out []Span
	cur Span
	buf strings.Builder
}

// text adds s in the given style, merging it with the previous run when
// the style is the same.
func (p *inlineParser) text(s string, a Attr, url string) {
	if s == "" {
		return
	}
	if p.buf.Len() > 0 && (p.cur.Attr != a || p.cur.URL != url) {
		p.flush()
	}
	p.cur.Attr, p.cur.URL = a, url
	p.buf.WriteString(s)
}

func (p *inlineParser) flush() {
	if p.buf.Len() == 0 {
		return
	}
	p.cur.Text = p.buf.String()
	p.out = append(p.out, p.cur)
	p.cur = Span{}
	p.buf.Reset()
}

// parse adds s in style a inside the link url ("" outside one).
func (p *inlineParser) parse(s string, a Attr, url string) {
	start := 0 // the start of the plain text not yet added
	for i := 0; i < len(s); {
		c := s[i]
		if strings.IndexByte(markers, c) < 0 {
			i++
			continue
		}
		var end int
		var inner func()
		switch c {
		case '\\':
			if i+1 < len(s) && isPunct(s[i+1]) {
				p.text(s[start:i], a, url)
				p.text(s[i+1:i+2], a, url)
				i += 2
				start = i
				continue
			}
		case '`':
			if e, body, ok := codeSpan(s, i); ok {
				end, inner = e, func() { p.text(body, a|Code, url) }
			}
		case '[':
			if url == "" {
				if e, label, target, ok := link(s, i); ok {
					end, inner = e, func() { p.parse(label, a, target) }
				}
			}
		case ':':
			if url == "" {
				if from, to, ok := bareURL(s, i); ok && from >= start {
					p.text(s[start:from], a, url)
					p.text(s[from:to], a, s[from:to])
					i, start = to, to
					continue
				}
			}
		case '<':
			if url == "" {
				if j := strings.IndexByte(s[i:], '>'); j > 0 {
					if t := s[i+1 : i+j]; isURL(t) && !strings.ContainsAny(t, " <") {
						end, inner = i+j+1, func() { p.text(t, a, t) }
					}
				}
			}
			if inner == nil && strings.HasPrefix(s[i:], "<u>") {
				if j := strings.Index(s[i+3:], "</u>"); j > 0 {
					body := s[i+3 : i+3+j]
					end, inner = i+3+j+4, func() { p.parse(body, a|Underline, url) }
				}
			}
		case '*', '_', '~':
			if e, open, body, ok := emphasis(s, i); ok {
				add := Italic
				switch {
				case c == '~':
					add = Strike
				case open == 2:
					add = Bold
				}
				end, inner = e, func() { p.parse(body, a|add, url) }
			}
		}
		if inner == nil {
			// Nothing opens here: skip the whole run of this marker, so
			// the middle of a run can't open either.
			j := i + 1
			for j < len(s) && s[j] == c && strings.IndexByte("*_`~", c) >= 0 {
				j++
			}
			i = j
			continue
		}
		p.text(s[start:i], a, url)
		inner()
		i, start = end, end
	}
	p.text(s[start:], a, url)
}

// codeSpan matches a code span opening at s[i]: a run of backticks, the
// code, and a run of the same length. It returns the end of the span and
// the code, with one space trimmed from each end when both have one.
func codeSpan(s string, i int) (end int, body string, ok bool) {
	n := run(s, i)
	for j := i + n; j < len(s); {
		k := strings.IndexByte(s[j:], '`')
		if k < 0 {
			return 0, "", false
		}
		j += k
		m := run(s, j)
		if m == n {
			body = s[i+n : j]
			if len(body) > 2 && body[0] == ' ' && body[len(body)-1] == ' ' && strings.TrimSpace(body) != "" {
				body = body[1 : len(body)-1]
			}
			if body == "" {
				return 0, "", false
			}
			return j + m, body, true
		}
		j += m
	}
	return 0, "", false
}

// link matches [label](url) at s[i]. The label may hold brackets in
// pairs, the url parentheses in pairs; the url has no spaces.
func link(s string, i int) (end int, label, url string, ok bool) {
	depth := 0
	j := i
	for ; j < len(s); j++ {
		switch s[j] {
		case '\\':
			j++
			continue
		case '[':
			depth++
		case ']':
			depth--
		}
		if depth == 0 {
			break
		}
	}
	if j >= len(s) || j+1 >= len(s) || s[j+1] != '(' {
		return 0, "", "", false
	}
	label = s[i+1 : j]
	depth = 0
	for k := j + 2; k < len(s); k++ {
		switch s[k] {
		case ' ', '\t':
			return 0, "", "", false
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
				continue
			}
			url = s[j+2 : k]
			if label == "" || url == "" {
				return 0, "", "", false
			}
			return k + 1, label, url, true
		}
	}
	return 0, "", "", false
}

// emphasis matches emphasis opening at s[i]: one or two of * or _, or two
// of ~, then text, then the same marker. The opener must not follow a
// letter or digit nor precede a space; the closer must not follow a space
// nor precede a letter or digit. A run of the marker inside a word is
// skipped for _ (snake_case) and ends the search for *, which can't tell
// 2*3*4 from a closer there. It returns the end, the marker's length and
// the text between.
func emphasis(s string, i int) (end, n int, body string, ok bool) {
	c := s[i]
	r := run(s, i)
	switch {
	case c == '~' && r == 2:
		n = 2
	case c != '~' && r <= 3:
		n = min(r, 2)
	default:
		return 0, 0, "", false
	}
	if wordBefore(s, i) || i+n >= len(s) || isSpaceAt(s, i+n) {
		return 0, 0, "", false
	}
	for j := i + n; j < len(s); {
		switch s[j] {
		case '\\':
			j += 2
			continue
		case '`':
			if e, _, ok := codeSpan(s, j); ok {
				j = e
				continue
			}
			j += run(s, j)
			continue
		case c:
		default:
			j++
			continue
		}
		m := run(s, j)
		if wordBefore(s, j) && wordAt(s, j+m) {
			if c == '_' {
				j += m
				continue
			}
			return 0, 0, "", false
		}
		// A run of the other length belongs to inner emphasis; a run of
		// three may carry an inner closer before this one (***), so the
		// marker is its last n characters.
		fits := m == n || (m == 3 && c != '~')
		at := j + m - n
		if fits && at > i+n && !isSpaceBefore(s, j) && !wordAt(s, j+m) {
			return j + m, n, s[i+n : at], true
		}
		j += m
	}
	return 0, 0, "", false
}

// bareURL finds the bare http(s) URL whose "://" starts at s[i]: from its
// scheme to the next space, without trailing punctuation or a closing
// parenthesis that has no opening one in the URL.
func bareURL(s string, i int) (from, to int, ok bool) {
	if !strings.HasPrefix(s[i:], "://") {
		return 0, 0, false
	}
	switch {
	case i >= 5 && s[i-5:i] == "https":
		from = i - 5
	case i >= 4 && s[i-4:i] == "http":
		from = i - 4
	default:
		return 0, 0, false
	}
	if wordBefore(s, from) {
		return 0, 0, false
	}
	to = i + 3
	for to < len(s) && !isSpaceAt(s, to) && s[to] != '<' {
		to++
	}
	for to > i+3 {
		last := s[to-1]
		if strings.IndexByte(".,;:!?*~'\"", last) < 0 &&
			(last != ')' || strings.Count(s[from:to], "(") >= strings.Count(s[from:to], ")")) {
			break
		}
		to--
	}
	if to <= i+3 {
		return 0, 0, false
	}
	return from, to, true
}

func isURL(s string) bool {
	return strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "http://")
}

// run is the length of the run of s[i] starting at i.
func run(s string, i int) int {
	j := i
	for j < len(s) && s[j] == s[i] {
		j++
	}
	return j - i
}

func wordBefore(s string, i int) bool {
	if i == 0 {
		return false
	}
	r, _ := utf8.DecodeLastRuneInString(s[:i])
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

func wordAt(s string, i int) bool {
	if i >= len(s) {
		return false
	}
	r, _ := utf8.DecodeRuneInString(s[i:])
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

func isSpaceAt(s string, i int) bool {
	r, _ := utf8.DecodeRuneInString(s[i:])
	return unicode.IsSpace(r)
}

func isSpaceBefore(s string, i int) bool {
	r, _ := utf8.DecodeLastRuneInString(s[:i])
	return unicode.IsSpace(r)
}

// isPunct reports whether a backslash escapes c, as CommonMark has it.
func isPunct(c byte) bool {
	return c < utf8.RuneSelf && unicode.IsPunct(rune(c)) || strings.IndexByte("$+<=>^`|~", c) >= 0
}
