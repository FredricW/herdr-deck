package manifest

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Vars are the values templates expand to (spec section 3).
type Vars struct {
	Worktree, Repo, Branch string
	// Ports holds the ports whose numbers are known.
	Ports map[string]int
	// Env holds what the manifest's env files set, for $env(…).
	Env map[string]string
}

// MissingPortError says a template uses a port whose number is not known
// yet: its state file does not have it, or the port store is not read.
type MissingPortError struct{ Port string }

func (e *MissingPortError) Error() string {
	return fmt.Sprintf("port %s is not known yet", e.Port)
}

// Expand fills in a template's variables. A $PORT_<name> without a known
// number is a *MissingPortError; Check has ruled out unknown names.
func (v Vars) Expand(s string) (string, error) {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] != '$' {
			b.WriteByte(s[i])
			i++
			continue
		}
		rest := s[i+1:]
		var name string
		switch {
		case strings.HasPrefix(rest, "$"):
			b.WriteByte('$')
			i += 2
			continue
		case strings.HasPrefix(rest, "env("):
			end := strings.IndexByte(rest, ')')
			if end < 0 {
				return "", fmt.Errorf("$env( without a closing )")
			}
			key, fallback, _ := strings.Cut(rest[len("env("):end], "|")
			if val, ok := v.Env[key]; ok {
				b.WriteString(val)
			} else {
				b.WriteString(fallback)
			}
			i += 1 + end + 1
			continue
		case strings.HasPrefix(rest, "{"):
			end := strings.IndexByte(rest, '}')
			if end < 0 {
				return "", fmt.Errorf("${ without a closing }")
			}
			name = rest[1:end]
			i += 1 + end + 1
		default:
			name = varNameRe.FindString(rest)
			if name == "" {
				return "", fmt.Errorf("a lone $ (write $$ for a literal $)")
			}
			i += 1 + len(name)
		}
		val, err := v.lookup(name)
		if err != nil {
			return "", err
		}
		b.WriteString(val)
	}
	return b.String(), nil
}

func (v Vars) lookup(name string) (string, error) {
	switch name {
	case "WORKTREE":
		return v.Worktree, nil
	case "REPO":
		return v.Repo, nil
	case "DIRNAME":
		return filepath.Base(v.Worktree), nil
	case "BRANCH":
		return v.Branch, nil
	}
	if p, ok := strings.CutPrefix(name, "PORT_"); ok {
		if n, ok := v.Ports[p]; ok {
			return strconv.Itoa(n), nil
		}
		return "", &MissingPortError{Port: p}
	}
	return "", fmt.Errorf("unknown variable $%s", name)
}

// Path expands a path template; a relative result is under the worktree.
func (v Vars) Path(s string) (string, error) {
	p, err := v.Expand(s)
	if err != nil {
		return "", err
	}
	if p != "" && !filepath.IsAbs(p) {
		p = filepath.Join(v.Worktree, p)
	}
	return p, nil
}

// PortRefs lists the port names a template's $PORT_<name> variables use.
func PortRefs(s string) []string {
	var out []string
	for i := 0; i < len(s); i++ {
		if s[i] != '$' {
			continue
		}
		rest := s[i+1:]
		if strings.HasPrefix(rest, "$") {
			i++
			continue
		}
		rest = strings.TrimPrefix(rest, "{")
		if name := varNameRe.FindString(rest); strings.HasPrefix(name, "PORT_") {
			out = append(out, strings.TrimPrefix(name, "PORT_"))
		}
	}
	return out
}

// LoadEnv reads the manifest's env files (templates, relative to the
// worktree), the last file setting a key winning. A missing file is
// skipped; a file template with a port not known yet too.
func (m *Manifest) LoadEnv(v Vars) map[string]string {
	out := map[string]string{}
	v.Env = nil
	for _, f := range m.EnvFiles {
		p, err := v.Path(f)
		if err != nil {
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		for k, val := range ParseDotenv(b) {
			out[k] = val
		}
	}
	return out
}

// ParseDotenv reads KEY=value lines: # comments, an optional "export ",
// and values in single quotes (as they are) or double quotes (with \n, \"
// and \\ escapes). An unquoted value ends at " #".
func ParseDotenv(b []byte) map[string]string {
	out := map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, val, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || !envNameRe.MatchString(key) {
			continue
		}
		val = strings.TrimSpace(val)
		switch {
		case len(val) >= 2 && val[0] == '\'' && strings.IndexByte(val[1:], '\'') >= 0:
			val = val[1 : 1+strings.IndexByte(val[1:], '\'')]
		case len(val) >= 2 && val[0] == '"':
			var sb strings.Builder
			for i := 1; i < len(val); i++ {
				c := val[i]
				if c == '"' {
					break
				}
				if c == '\\' && i+1 < len(val) {
					i++
					switch val[i] {
					case 'n':
						sb.WriteByte('\n')
					default:
						sb.WriteByte(val[i])
					}
					continue
				}
				sb.WriteByte(c)
			}
			val = sb.String()
		default:
			if i := strings.Index(val, " #"); i >= 0 {
				val = strings.TrimSpace(val[:i])
			}
		}
		out[key] = val
	}
	return out
}
