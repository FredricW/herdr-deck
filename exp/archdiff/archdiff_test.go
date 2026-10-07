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
