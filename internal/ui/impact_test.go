package ui

import (
	"context"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/FredricW/herdr-deck/internal/deck"
	"github.com/FredricW/herdr-deck/internal/source/arch"
	"github.com/FredricW/herdr-deck/internal/source/fake"
)

// impactModel is the sample deck on t-0002 (it needs the user, so the
// cursor rests on it) with the Impact tab shown and its read done.
func impactModel(t *testing.T, w, h int, read func(context.Context, deck.Thread) *arch.Result) Model {
	t.Helper()
	m, _ := newModelWith(t, fakeSnap(), w, h, func(o *Options) {
		o.Diff = fake.Diff
		o.Arch = read
	})
	m.switchTab(tabImpact)
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyF13}) // any key: reads follow
	return m
}

func TestImpactGolden(t *testing.T) {
	for _, w := range []int{60, 80, 120} {
		h := 28
		if w == 120 {
			h = 36
		}
		m := impactModel(t, w, h, fake.Arch)
		if r, ok := m.selected(); !ok || r.key == "" || m.curTab() != tabImpact {
			t.Fatalf("%d: not on the Impact tab", w)
		}
		golden(t, "impact-"+strconv.Itoa(w), m)
		full, _ := press(m, keys("z")...)
		golden(t, "impact-full-"+strconv.Itoa(w), full)
	}
}

func TestImpactStates(t *testing.T) {
	// Before the first read: the label and the tab say so.
	m, _ := newModelWith(t, fakeSnap(), 80, 28, func(o *Options) {
		o.Diff = fake.Diff
		o.Arch = fake.Arch
	})
	m.switchTab(tabImpact)
	golden(t, "impact-reading-80", m)
	if s := screen(m); !strings.Contains(s, "Impact …") || !strings.Contains(s, "reading the change's shape…") {
		t.Errorf("reading:\n%s", s)
	}

	note := impactModel(t, 80, 28, func(context.Context, deck.Thread) *arch.Result {
		return &arch.Result{Base: "origin/main", Note: "cannot compare with origin/main: unknown revision"}
	})
	if s := screen(note); !strings.Contains(s, "cannot compare with origin/main") {
		t.Errorf("note:\n%s", s)
	}

	// Off: no tab at all.
	off, _ := newModelWith(t, fakeSnap(), 80, 28, func(o *Options) { o.Diff = fake.Diff })
	if strings.Contains(screen(off), "Impact") {
		t.Error("the tab shows while off")
	}
}

// The tab reads on every reload, from the reader's cache, and only for a
// thread with a worktree.
func TestImpactReads(t *testing.T) {
	var reads []string
	m := impactModel(t, 80, 28, func(_ context.Context, th deck.Thread) *arch.Result {
		reads = append(reads, th.ID)
		return fake.Arch(context.Background(), th)
	})
	if len(reads) != 1 || reads[0] != "t-0002" {
		t.Fatalf("reads = %v", reads)
	}
	// Moving within the same thread reads nothing new.
	m, _ = press(m, keys("[]")...)
	if len(reads) != 1 {
		t.Errorf("reads after switching tabs = %v", reads)
	}
	// A reload reads again (the reader answers from its cache).
	m, _ = press(m, snapshotMsg(fakeSnap()))
	if len(reads) != 2 {
		t.Errorf("reads after a reload = %v", reads)
	}
	if !strings.Contains(m.impactLabel(), "Impact ") {
		t.Errorf("label = %q", m.impactLabel())
	}
}
