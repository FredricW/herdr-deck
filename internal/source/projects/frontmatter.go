package projects

import (
	"errors"
	"strings"

	"github.com/BurntSushi/toml"
)

var errNoFrontMatter = errors.New("no +++ front matter")

// frontMatter decodes the `+++`-fenced TOML block at the start of text into v
// and returns the text after it.
func frontMatter(text string, v any) (body string, err error) {
	text = strings.TrimPrefix(text, "\ufeff")
	lines := strings.SplitAfter(text, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "+++" {
		return text, errNoFrontMatter
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "+++" {
			block := strings.Join(lines[1:i], "")
			if _, err := toml.Decode(block, v); err != nil {
				return text, err
			}
			return strings.Join(lines[i+1:], ""), nil
		}
	}
	return text, errors.New("unclosed +++ front matter")
}
