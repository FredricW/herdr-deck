package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// Save sets key to value in the config file at path, or removes the key
// when value is nil, so the next source decides. It changes only the lines
// of that key: comments, key order and keys the deck does not know stay as
// they are. The file and its folder are created when missing, and the new
// file replaces the old in one rename. A symlinked file is written through
// the link.
//
// Save refuses a value Check rejects and a file that does not parse, and
// it checks that the result reads back with only that key changed.
func Save(path, key string, value *string) error {
	if path == "" {
		return errors.New("no config file: no home folder to put it in")
	}
	sp, ok := SpecFor(key)
	if !ok {
		return fmt.Errorf("unknown setting %q", key)
	}
	var lit string
	if value != nil {
		v := strings.TrimSpace(*value)
		if err := Check(key, v); err != nil {
			return err
		}
		if sp.Kind == KindBool {
			b, _ := strconv.ParseBool(v)
			lit = strconv.FormatBool(b)
		} else {
			lit = tomlString(v)
		}
	}

	target := path
	if p, err := filepath.EvalSymlinks(path); err == nil {
		target = p
	}
	old, err := os.ReadFile(target)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	var before map[string]any
	if _, err := toml.Decode(string(old), &before); err != nil {
		return fmt.Errorf("the config file does not parse (%s); fix it by hand first", parseErr(err))
	}

	text, err := patch(string(old), key, lit, value != nil)
	if err != nil {
		return err
	}
	if text == string(old) {
		return nil
	}
	if err := verify(before, text, key, sp.Kind, lit, value != nil); err != nil {
		return err
	}
	return writeAtomic(target, []byte(text))
}

// verify decodes the patched text and checks that key holds the new value
// and nothing else changed.
func verify(before map[string]any, text, key string, kind Kind, lit string, set bool) error {
	var after map[string]any
	if _, err := toml.Decode(text, &after); err != nil {
		return fmt.Errorf("saving %s would break the file (%s); nothing was written", key, parseErr(err))
	}
	if before == nil {
		before = map[string]any{}
	}
	want := cloneMap(before)
	table, name := splitKey(key)
	m := want
	if table != "" {
		sub, _ := m[table].(map[string]any)
		sub = cloneMap(sub)
		m[table] = sub
		m = sub
	}
	if set {
		var v any
		if kind == KindBool {
			v = lit == "true"
		} else {
			var s struct{ V string }
			if _, err := toml.Decode("V = "+lit, &s); err != nil {
				return err
			}
			v = s.V
		}
		m[name] = v
	} else {
		delete(m, name)
	}
	// Removing a table's last key leaves an empty table behind; that
	// reads the same.
	if table != "" && len(m) == 0 {
		if _, ok := after[table]; !ok {
			delete(want, table)
		}
	}
	if !reflect.DeepEqual(want, after) {
		return fmt.Errorf("saving %s would change more than that key; edit the file by hand", key)
	}
	return nil
}

func cloneMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// writeAtomic writes b to a temp file next to path and renames it over
// path, keeping an existing file's permissions.
func writeAtomic(path string, b []byte) (err error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	mode := fs.FileMode(0o644)
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode().Perm()
	}
	tmp, err := os.CreateTemp(dir, ".config.toml.*")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = os.Remove(tmp.Name())
		}
	}()
	if _, err = tmp.Write(b); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Chmod(tmp.Name(), mode); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// splitKey splits "editor.command" into its table and key.
func splitKey(key string) (table, name string) {
	if i := strings.IndexByte(key, '.'); i >= 0 {
		return key[:i], key[i+1:]
	}
	return "", key
}

// tomlString quotes s as a TOML string: a literal string when s has quotes
// or backslashes (common in commands) and no ', else a basic string.
func tomlString(s string) string {
	plain := !strings.ContainsFunc(s, func(r rune) bool { return r < 0x20 || r == 0x7f })
	if plain && strings.ContainsAny(s, `"\`) && !strings.Contains(s, "'") {
		return "'" + s + "'"
	}
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"' || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\n':
			b.WriteString(`\n`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\u%04X`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// entry is a key/value line of a TOML document.
type entry struct {
	table      string   // the [table] it sits under; "" at the top
	key        []string // the key's dotted parts, unquoted
	start, end int      // its lines: from the line's start to after its newline
	vs, ve     int      // the value
}

// header is a [table] line.
type header struct {
	name       string // "" for an array of tables, which never matches
	start, end int
}

// scan finds the key/value lines and table headers of a TOML document that
// parses. It knows just enough TOML to skip strings, arrays and inline
// tables that span lines.
func scan(s string) ([]entry, []header, error) {
	var es []entry
	var hs []header
	table := ""
	i := 0
	for i < len(s) {
		ls := i
		i = skipSpace(s, i)
		switch {
		case i >= len(s):
		case s[i] == '\n' || s[i] == '\r' || s[i] == '#':
		case s[i] == '[':
			array := strings.HasPrefix(s[i:], "[[")
			j := i + 1
			if array {
				j++
			}
			parts, k, err := scanKey(s, j)
			if err != nil {
				return nil, nil, err
			}
			table = strings.Join(parts, ".")
			if array {
				table = "[[" + table + "]]"
			}
			name := table
			if array {
				name = ""
			}
			i = k
			hs = append(hs, header{name: name, start: ls, end: lineEnd(s, i)})
		default:
			parts, k, err := scanKey(s, i)
			if err != nil {
				return nil, nil, err
			}
			k = skipSpace(s, k)
			if k >= len(s) || s[k] != '=' {
				return nil, nil, fmt.Errorf("no = after key %s", strings.Join(parts, "."))
			}
			vs := skipSpace(s, k+1)
			ve, err := scanValue(s, vs)
			if err != nil {
				return nil, nil, err
			}
			es = append(es, entry{table: table, key: parts, start: ls, end: lineEnd(s, ve), vs: vs, ve: ve})
			i = ve
		}
		i = lineEnd(s, i)
	}
	return es, hs, nil
}

func skipSpace(s string, i int) int {
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	return i
}

// lineEnd is the offset after the newline that ends the line holding i.
func lineEnd(s string, i int) int {
	if j := strings.IndexByte(s[min(i, len(s)):], '\n'); j >= 0 {
		return i + j + 1
	}
	return len(s)
}

// scanKey reads a dotted key at i, up to the = or ] after it.
func scanKey(s string, i int) ([]string, int, error) {
	var parts []string
	for {
		i = skipSpace(s, i)
		if i >= len(s) {
			return nil, i, errors.New("key ends the file")
		}
		switch s[i] {
		case '"', '\'':
			end, err := scanValue(s, i)
			if err != nil {
				return nil, i, err
			}
			var v struct{ K string }
			if _, err := toml.Decode("K = "+s[i:end], &v); err != nil {
				return nil, i, err
			}
			parts = append(parts, v.K)
			i = end
		default:
			j := i
			for j < len(s) && (s[j] == '_' || s[j] == '-' || s[j] >= 'a' && s[j] <= 'z' || s[j] >= 'A' && s[j] <= 'Z' || s[j] >= '0' && s[j] <= '9') {
				j++
			}
			if j == i {
				return nil, i, fmt.Errorf("unexpected %q in a key", s[i])
			}
			parts = append(parts, s[i:j])
			i = j
		}
		i = skipSpace(s, i)
		if i < len(s) && s[i] == '.' {
			i++
			continue
		}
		return parts, i, nil
	}
}

// scanValue returns the offset just after the value that starts at i.
func scanValue(s string, i int) (int, error) {
	if i >= len(s) {
		return i, errors.New("missing value")
	}
	switch {
	case strings.HasPrefix(s[i:], `"""`):
		for j := i + 3; j < len(s); j++ {
			if s[j] == '\\' {
				j++
				continue
			}
			if strings.HasPrefix(s[j:], `"""`) {
				// Up to two quotes may sit right before the closing ones.
				end := j + 3
				for end < len(s) && s[end] == '"' && end-j < 5 {
					end++
				}
				return end, nil
			}
		}
		return 0, errors.New("unterminated string")
	case strings.HasPrefix(s[i:], `'''`):
		j := strings.Index(s[i+3:], `'''`)
		if j < 0 {
			return 0, errors.New("unterminated string")
		}
		end := i + 3 + j + 3
		for end < len(s) && s[end] == '\'' && end-(i+3+j) < 5 {
			end++
		}
		return end, nil
	case s[i] == '"':
		for j := i + 1; j < len(s) && s[j] != '\n'; j++ {
			if s[j] == '\\' {
				j++
				continue
			}
			if s[j] == '"' {
				return j + 1, nil
			}
		}
		return 0, errors.New("unterminated string")
	case s[i] == '\'':
		j := strings.IndexAny(s[i+1:], "'\n")
		if j < 0 || s[i+1+j] != '\'' {
			return 0, errors.New("unterminated string")
		}
		return i + 1 + j + 1, nil
	case s[i] == '[' || s[i] == '{':
		depth := 0
		for j := i; j < len(s); {
			switch s[j] {
			case '[', '{':
				depth++
				j++
			case ']', '}':
				depth--
				j++
				if depth == 0 {
					return j, nil
				}
			case '#':
				j = lineEnd(s, j)
			case '"', '\'':
				end, err := scanValue(s, j)
				if err != nil {
					return 0, err
				}
				j = end
			default:
				j++
			}
		}
		return 0, errors.New("unterminated array or table")
	}
	j := i
	for j < len(s) && s[j] != '\n' && s[j] != '\r' && s[j] != '#' {
		j++
	}
	for j > i && (s[j-1] == ' ' || s[j-1] == '\t') {
		j--
	}
	return j, nil
}

// patch returns s with key set to the TOML literal lit, or removed when set
// is false.
func patch(s, key, lit string, set bool) (string, error) {
	es, hs, err := scan(s)
	if err != nil {
		return "", fmt.Errorf("cannot edit the config file (%v); edit it by hand", err)
	}
	table, name := splitKey(key)
	nl := "\n"
	if strings.Contains(s, "\r\n") {
		nl = "\r\n"
	}

	for _, e := range es {
		if table != "" && e.table == "" && len(e.key) == 1 && e.key[0] == table {
			return "", fmt.Errorf("%s is not a [%s] table in the config file; edit %s by hand", table, table, key)
		}
		inTable := e.table == table && len(e.key) == 1 && e.key[0] == name
		dotted := table != "" && e.table == "" && len(e.key) == 2 && e.key[0] == table && e.key[1] == name
		if !inTable && !dotted {
			continue
		}
		if !set {
			return s[:e.start] + s[e.end:], nil
		}
		return s[:e.vs] + lit + s[e.ve:], nil
	}
	if !set {
		return s, nil
	}

	// A new key goes after the last key of its table.
	line := name + " = " + lit + nl
	if table == "" {
		at := -1
		for _, e := range es {
			if e.table == "" {
				at = e.end
			}
		}
		if at < 0 && len(hs) > 0 {
			// Before the first table and the comments right above it.
			at = commentsAbove(s, hs[0].start)
			return s[:at] + line + nl + s[at:], nil
		}
		if at < 0 {
			at = len(s)
		}
		return insertLine(s, at, line, nl), nil
	}
	at := -1
	for _, e := range es {
		if e.table == "" && len(e.key) == 2 && e.key[0] == table {
			at = e.end
		}
	}
	if at >= 0 {
		line = table + "." + line
	} else {
		for _, h := range hs {
			if h.name == table {
				at = h.end
				for _, e := range es {
					if e.table == table {
						at = max(at, e.end)
					}
				}
			}
		}
	}
	if at >= 0 {
		return insertLine(s, at, line, nl), nil
	}
	head := "[" + table + "]" + nl + line
	switch {
	case s == "":
		return head, nil
	case strings.HasSuffix(s, nl+nl) || strings.HasSuffix(s, "\n\n"):
		return s + head, nil
	case strings.HasSuffix(s, "\n"):
		return s + nl + head, nil
	}
	return s + nl + nl + head, nil
}

// insertLine puts line at offset at, the start of a line; at the end of a
// file without a final newline it adds one first.
func insertLine(s string, at int, line, nl string) string {
	if at == len(s) && s != "" && !strings.HasSuffix(s, "\n") {
		return s + nl + line
	}
	return s[:at] + line + s[at:]
}

// commentsAbove moves the line start at back over the comment lines right
// above it.
func commentsAbove(s string, at int) int {
	for at > 0 {
		prev := strings.LastIndexByte(s[:at-1], '\n') + 1
		if !strings.HasPrefix(strings.TrimLeft(s[prev:at], " \t"), "#") {
			break
		}
		at = prev
	}
	return at
}
