package main

import (
	"slices"
	"testing"
)

func TestLanesCheck(t *testing.T) {
	cfg := &layerConfig{}
	cfg.Layers = append(cfg.Layers,
		struct {
			Name   string   `json:"name"`
			Paths  []string `json:"paths"`
			Closed bool     `json:"closed,omitempty"`
		}{Name: "api", Paths: []string{"api/**"}},
		struct {
			Name   string   `json:"name"`
			Paths  []string `json:"paths"`
			Closed bool     `json:"closed,omitempty"`
		}{Name: "domain", Paths: []string{"domain/**"}, Closed: true},
		struct {
			Name   string   `json:"name"`
			Paths  []string `json:"paths"`
			Closed bool     `json:"closed,omitempty"`
		}{Name: "infra", Paths: []string{"infra/**"}},
	)
	l := configLanes(cfg, []string{"api/routes", "domain/export", "infra/s3", "tools"})
	for _, c := range []struct {
		from, to, want string
	}{
		{"api/routes", "domain/export", ""},
		{"domain/export", "infra/s3", ""},
		{"api/routes", "infra/s3", "skip"},
		{"infra/s3", "domain/export", "up"},
		{"tools", "api/routes", ""},
	} {
		if got, _ := l.check(edgeKey{c.from, c.to}); got != c.want {
			t.Errorf("%s → %s: %q, want %q", c.from, c.to, got, c.want)
		}
	}
	if l.names[l.of["tools"]] != "unassigned" {
		t.Errorf("tools lane = %q", l.names[l.of["tools"]])
	}
}

func TestNewCycles(t *testing.T) {
	base := map[edgeKey]bool{{"api", "domain"}: true, {"domain", "infra"}: true}
	head := map[edgeKey]bool{{"api", "domain"}: true, {"domain", "infra"}: true, {"infra", "api"}: true}
	got := newCycles(base, head)
	if len(got) != 1 || !slices.Equal(got[0], []string{"infra", "api", "domain", "infra"}) {
		t.Fatalf("cycles = %v", got)
	}
	if id := cycleID(got[0]); id != "api→domain→infra→api" {
		t.Errorf("id = %q", id)
	}
}

func TestParseGoTouchesAndShape(t *testing.T) {
	g := parseGo([]byte(`package export

import (
	"net/http"
	"os"
)

const tokenEnv = "ABC_TOKEN"

type Job struct {
	ID   int
	note string
}

func Run() {
	_ = os.Getenv(tokenEnv)
	_, _ = http.Get("https://s3.example.com/b")
	_ = "SELECT id FROM export_jobs WHERE done"
}
`))
	var ts []string
	for _, x := range g.touches {
		ts = append(ts, x.String())
	}
	want := []string{"env ABC_TOKEN", "http s3.example.com", "sql export_jobs"}
	if !slices.Equal(ts, want) {
		t.Errorf("touches = %q, want %q", ts, want)
	}
	for _, d := range g.decls {
		if d.key == "type Job" && d.sig != "type Job struct {\n\tID int\n}" {
			t.Errorf("Job sig = %q", d.sig)
		}
	}
}

// The canvas layout keeps every child inside its parent, siblings apart,
// and gives the same geometry on every run.
func TestCanvasLayout(t *testing.T) {
	mk := func() *cnode {
		leaf := func(p string, lines ...string) *cnode {
			return &cnode{path: p, name: p, pkg: true, impacted: true, kind: '~', lines: lines}
		}
		src := &cnode{path: "internal/source", name: "source", children: []*cnode{
			leaf("dev", "+2362 −735", "api +16 −11 ~4"), leaf("github", "+1007 −0"), leaf("live"),
		}}
		in := &cnode{path: "internal", name: "internal", children: []*cnode{leaf("ui", "+276 −94"), src, leaf("deck")}}
		return &cnode{path: ".", name: "repo", children: []*cnode{leaf("cmd"), in}}
	}
	var walk func(n *cnode, f func(n *cnode))
	walk = func(n *cnode, f func(n *cnode)) {
		f(n)
		for _, c := range n.children {
			walk(c, f)
		}
	}
	for _, w := range []int{60, 80, 120} {
		for _, gap := range []int{0, 1} {
			a, b := mk(), mk()
			a.layout(0, 0, w, gap)
			b.layout(0, 0, w, gap)
			var ga, gb []int
			walk(a, func(n *cnode) { ga = append(ga, n.x, n.y, n.w, n.h) })
			walk(b, func(n *cnode) { gb = append(gb, n.x, n.y, n.w, n.h) })
			if !slices.Equal(ga, gb) {
				t.Fatalf("w=%d: layout not deterministic", w)
			}
			walk(a, func(p *cnode) {
				for i, c := range p.children {
					if c.x <= p.x || c.y <= p.y || c.x+c.w >= p.x+p.w || c.y+c.h >= p.y+p.h {
						t.Errorf("w=%d gap=%d: %s not inside %s", w, gap, c.path, p.path)
					}
					for _, d := range p.children[i+1:] {
						if c.x < d.x+d.w && d.x < c.x+c.w && c.y < d.y+d.h && d.y < c.y+c.h {
							t.Errorf("w=%d gap=%d: %s overlaps %s", w, gap, c.path, d.path)
						}
					}
				}
			})
		}
	}
}
