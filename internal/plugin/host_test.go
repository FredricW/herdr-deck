package plugin

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/FredricW/herdr-deck/internal/source/herdr"
)

// fakeSocket is a herdr socket that answers each method with a fixed result
// and records the params of each request.
type fakeSocket struct {
	path   string
	mu     sync.Mutex
	params map[string]string
}

func newFakeSocket(t *testing.T, results map[string]string) *fakeSocket {
	t.Helper()
	// A short path: Unix socket paths are limited to about 100 bytes.
	dir, err := os.MkdirTemp("", "hd")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	s := &fakeSocket{path: filepath.Join(dir, "s.sock"), params: map[string]string{}}
	ln, err := net.Listen("unix", s.path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			sc := bufio.NewScanner(conn)
			for sc.Scan() {
				var req struct {
					ID     string          `json:"id"`
					Method string          `json:"method"`
					Params json.RawMessage `json:"params"`
				}
				if json.Unmarshal(sc.Bytes(), &req) != nil {
					break
				}
				s.mu.Lock()
				s.params[req.Method] = string(req.Params)
				s.mu.Unlock()
				_, _ = conn.Write([]byte(`{"id":"` + req.ID + `","result":` + results[req.Method] + "}\n"))
			}
			conn.Close()
		}
	}()
	return s
}

func (s *fakeSocket) sent(method string) map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	var m map[string]any
	_ = json.Unmarshal([]byte(s.params[method]), &m)
	return m
}

// The replies are trimmed from herdr 0.9.3's.
func TestSocketLeftOf(t *testing.T) {
	s := newFakeSocket(t, map[string]string{
		"pane.neighbor": `{"type":"pane_neighbor","neighbor":{"direction":"left","pane_id":"w1:p5","neighbor_pane_id":"w1:p1","layout":{}}}`,
	})
	h := Socket{Client: herdr.Client{Socket: s.path}}
	if left, err := h.LeftOf(context.Background(), "w1:p5"); err != nil || left != "w1:p1" {
		t.Fatalf("LeftOf = %q, %v", left, err)
	}
	if p := s.sent("pane.neighbor"); p["pane_id"] != "w1:p5" || p["direction"] != "left" {
		t.Errorf("params = %v", p)
	}

	s = newFakeSocket(t, map[string]string{
		"pane.neighbor": `{"type":"pane_neighbor","neighbor":{"direction":"left","pane_id":"w1:p5","neighbor_pane_id":null,"layout":{}}}`,
	})
	h = Socket{Client: herdr.Client{Socket: s.path}}
	if left, err := h.LeftOf(context.Background(), "w1:p5"); err != nil || left != "" {
		t.Fatalf("LeftOf with no neighbor = %q, %v", left, err)
	}
}

func TestSocketOpenFocus(t *testing.T) {
	s := newFakeSocket(t, map[string]string{
		"plugin.pane.open": `{"type":"plugin_pane_opened","plugin_pane":{"pane":{"pane_id":"w1:p2"}}}`,
	})
	h := Socket{Client: herdr.Client{Socket: s.path}}
	for _, focus := range []bool{true, false} {
		if id, err := h.Open(context.Background(), Open{Target: "w1:p1", Focus: focus}); err != nil || id != "w1:p2" {
			t.Fatalf("Open = %q, %v", id, err)
		}
		if p := s.sent("plugin.pane.open"); p["focus"] != focus || p["target_pane_id"] != "w1:p1" {
			t.Errorf("focus %v: params = %v", focus, p)
		}
	}
}
