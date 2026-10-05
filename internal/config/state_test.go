package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDrawerHeightFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", DrawerFile)
	if _, ok := ReadDrawerHeight(path); ok {
		t.Fatal("a missing file holds no height")
	}
	if err := WriteDrawerHeight(path, 0.4166666); err != nil {
		t.Fatal(err)
	}
	if v, ok := ReadDrawerHeight(path); !ok || v != 0.4167 {
		t.Errorf("ReadDrawerHeight = %v, %v; want 0.4167", v, ok)
	}
	// A dragged height may go beyond ui.drawer_height's range.
	if err := WriteDrawerHeight(path, 0.9); err != nil {
		t.Fatal(err)
	}
	if v, ok := ReadDrawerHeight(path); !ok || v != 0.9 {
		t.Errorf("ReadDrawerHeight = %v, %v; want 0.9", v, ok)
	}
	for _, bad := range []float64{0, 1, -0.5} {
		if err := WriteDrawerHeight(path, bad); err == nil {
			t.Errorf("WriteDrawerHeight(%v) must fail", bad)
		}
	}
	for _, text := range []string{"", "half", "1.2", "0"} {
		if err := os.WriteFile(path, []byte(text+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if v, ok := ReadDrawerHeight(path); ok {
			t.Errorf("%q reads as %v", text, v)
		}
	}
	if _, ok := ReadDrawerHeight(""); ok {
		t.Error("no path holds no height")
	}
}
