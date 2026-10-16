package markdown

import (
	"fmt"
	"strings"
	"testing"
)

// show writes spans compactly: styled runs as {attrs:text} and links as
// {text->url}.
func show(spans []Span) string {
	var b strings.Builder
	for _, s := range spans {
		var a string
		for _, f := range []struct {
			bit Attr
			c   string
		}{{Bold, "b"}, {Italic, "i"}, {Code, "c"}, {Strike, "s"}, {Underline, "u"}} {
			if s.Attr&f.bit != 0 {
				a += f.c
			}
		}
		switch {
		case a == "" && s.URL == "":
			b.WriteString(s.Text)
		case s.URL != "":
			fmt.Fprintf(&b, "{%s:%s->%s}", a, s.Text, s.URL)
		default:
			fmt.Fprintf(&b, "{%s:%s}", a, s.Text)
		}
	}
	return b.String()
}

func TestInline(t *testing.T) {
	for _, tc := range []struct{ src, want string }{
		{"", ""},
		{"Plain title", "Plain title"},
		{"An *italic* word", "An {i:italic} word"},
		{"An _italic_ word", "An {i:italic} word"},
		{"A **bold** word", "A {b:bold} word"},
		{"A __bold__ word", "A {b:bold} word"},
		{"Run `make test` now", "Run {c:make test} now"},
		{"Run ``a `b` c`` now", "Run {c:a `b` c} now"},
		{"Drop ~~old~~ page", "Drop {s:old} page"},
		{"An <u>underlined</u> word", "An {u:underlined} word"},
		{"See [the spec](https://example.com/spec) first", "See {:the spec->https://example.com/spec} first"},
		{"[**Bold** link](https://example.com)", "{b:Bold->https://example.com}{: link->https://example.com}"},
		{"**bold and *italic* inside**", "{b:bold and }{bi:italic}{b: inside}"},
		{"***both***", "{bi:both}"},
		{"**ABC-123**: fix totals", "{b:ABC-123}: fix totals"},
		{"(*draft*)", "({i:draft})"},
		{"`**not bold**`", "{c:**not bold**}"},
		{"*a `b*` c*", "{i:a }{ic:b*}{i: c}"},
		{`\*not italic\*`, "*not italic*"},

		// Literal markers stay as written.
		{"Rename snake_case_names", "Rename snake_case_names"},
		{"Compute 2*3*4", "Compute 2*3*4"},
		{"A lone ` backtick", "A lone ` backtick"},
		{"An * asterisk * pair", "An * asterisk * pair"},
		{"Unclosed **bold", "Unclosed **bold"},
		{"Unclosed _italic", "Unclosed _italic"},
		{"~single~ tilde", "~single~ tilde"},
		{"Load my__init__file", "Load my__init__file"},
		{"a [bracket] only", "a [bracket] only"},
		{"[text] (spaced)", "[text] (spaced)"},
		{"[x](has space)", "[x](has space)"},
		{"x < y and <b>", "x < y and <b>"},
		{"****", "****"},
		{"**", "**"},
		{"path/*/glob", "path/*/glob"},
		{`trailing \`, `trailing \`},
	} {
		if got := show(Inline(tc.src)); got != tc.want {
			t.Errorf("Inline(%q) = %q, want %q", tc.src, got, tc.want)
		}
	}
}

func TestPlain(t *testing.T) {
	for src, want := range map[string]string{
		"Fix **totals** in `ledger`":  "Fix totals in ledger",
		"[Spec](https://example.com)": "Spec",
		"snake_case_names":            "snake_case_names",
		"no markers":                  "no markers",
	} {
		if got := Plain(src); got != want {
			t.Errorf("Plain(%q) = %q, want %q", src, got, want)
		}
	}
}

func BenchmarkInlineCached(b *testing.B) {
	src := "Fix **totals** in `ledger` and [the spec](https://example.com)"
	for range b.N {
		Inline(src)
	}
}

func TestInlineURLs(t *testing.T) {
	for _, tc := range []struct{ src, want string }{
		{"See https://example.com/a.", "See {:https://example.com/a->https://example.com/a}."},
		{"(https://example.com)", "({:https://example.com->https://example.com})"},
		{"At <https://example.com/x>", "At {:https://example.com/x->https://example.com/x}"},
		{"**https://example.com**", "{b:https://example.com->https://example.com}"},
		{"time 10:30 and a: b", "time 10:30 and a: b"},
		{"xhttps://example.com", "xhttps://example.com"},
		{"https:// alone", "https:// alone"},
	} {
		if got := show(Inline(tc.src)); got != tc.want {
			t.Errorf("Inline(%q) = %q, want %q", tc.src, got, tc.want)
		}
	}
}
