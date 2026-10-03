// Package syntax colours source lines by their file's language for the
// diff preview, with chroma's lexers (which glamour already brings in).
// Tokens get named ANSI colours, so the terminal's theme decides the
// shades and they read on dark and light backgrounds alike.
package syntax

import (
	"path"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/charmbracelet/x/ansi"
)

// TabWidth is how many columns a tab takes.
const TabWidth = 4

// Lines returns lines coloured as the language of the file at name (a
// path; only its base name and extension count). Tabs become spaces and
// control characters a visible `�`, so a line's width is its text's.
// An unknown language, or a lexer that trips, gives the cleaned lines
// uncoloured. The result has one line per input line.
func Lines(name string, lines []string) []string {
	clean := make([]string, len(lines))
	for i, l := range lines {
		clean[i] = Clean(l)
	}
	lx := lexer(name)
	if lx == nil || len(lines) == 0 {
		return clean
	}
	it, err := chroma.Coalesce(lx).Tokenise(nil, strings.Join(clean, "\n")+"\n")
	if err != nil {
		return clean
	}
	out := make([]string, 0, len(lines)+1)
	var b strings.Builder
	for tok := it(); tok != chroma.EOF; tok = it() {
		sgr := style(tok.Type)
		for i, part := range strings.Split(tok.Value, "\n") {
			if i > 0 {
				out = append(out, b.String())
				b.Reset()
			}
			if part == "" {
				continue
			}
			if sgr == "" {
				b.WriteString(part)
			} else {
				b.WriteString("\x1b[" + sgr + "m" + part + "\x1b[m")
			}
		}
	}
	if b.Len() > 0 {
		out = append(out, b.String())
	}
	// The colours must not change the text; if a lexer dropped or added
	// something, the lines go out plain.
	if len(out) < len(lines) {
		return clean
	}
	out = out[:len(lines)]
	for i := range out {
		if ansi.Strip(out[i]) != clean[i] {
			return clean
		}
	}
	return out
}

// Known says whether name's language has a lexer, so it gets colours.
func Known(name string) bool { return lexer(name) != nil }

func lexer(name string) chroma.Lexer {
	lx := lexers.Match(path.Base(name))
	if lx == nil {
		return nil
	}
	switch strings.ToLower(lx.Config().Name) {
	case "plaintext", "text", "plain text":
		return nil
	}
	return lx
}

// Clean expands tabs to TabWidth columns and replaces control characters,
// which would move the cursor or change the terminal, with `�`.
func Clean(s string) string {
	if !strings.ContainsFunc(s, isControl) {
		return s
	}
	var b strings.Builder
	col := 0
	for _, r := range s {
		switch {
		case r == '\t':
			n := TabWidth - col%TabWidth
			b.WriteString(strings.Repeat(" ", n))
			col += n
		case isControl(r):
			b.WriteRune('�')
			col++
		default:
			b.WriteRune(r)
			col += max(ansi.StringWidth(string(r)), 0)
		}
	}
	return b.String()
}

func isControl(r rune) bool { return r < 0x20 || r == 0x7f || (r >= 0x80 && r < 0xa0) }

// style is a token type's SGR parameters: named colours only, "" for
// plain text.
func style(t chroma.TokenType) string {
	switch {
	case t.InCategory(chroma.Comment):
		return "2"
	case t == chroma.KeywordType:
		return "36"
	case t.InCategory(chroma.Keyword):
		return "35"
	case t.InSubCategory(chroma.LiteralString):
		return "33"
	case t.InSubCategory(chroma.LiteralNumber), t == chroma.Literal, t == chroma.LiteralDate:
		return "36"
	case t == chroma.NameFunction, t == chroma.NameClass, t == chroma.NameBuiltin, t == chroma.NameTag,
		t == chroma.NameFunctionMagic, t == chroma.NameBuiltinPseudo:
		return "34"
	case t == chroma.NameAttribute, t == chroma.NameDecorator, t == chroma.NameConstant:
		return "36"
	case t == chroma.GenericHeading, t == chroma.GenericSubheading, t == chroma.GenericStrong:
		return "1"
	case t == chroma.GenericEmph:
		return "3"
	}
	return ""
}
