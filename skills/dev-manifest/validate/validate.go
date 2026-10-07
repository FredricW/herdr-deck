package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"github.com/FredricW/herdr-deck/internal/source/dev/manifest"
)

// Result holds a manifest's problems. Errors break the schema or a MUST of
// the spec; warnings are dangling references found by the dry check.
type Result struct {
	Errors   []string
	Warnings []string
}

// Validate checks the manifest in file. With a non-empty root it also runs
// the dry check against that repository.
func Validate(file, root string) Result {
	b, err := os.ReadFile(file)
	if err != nil {
		return Result{Errors: []string{err.Error()}}
	}
	return ValidateBytes(b, root)
}

// ValidateBytes is Validate on a manifest's content. The checks are the
// deck's own (internal/source/dev/manifest), except that a key the schema
// does not know is an error here: tools ignore it, editors should not.
func ValidateBytes(b []byte, root string) Result {
	var res Result
	p := manifest.Check(b)
	for _, k := range p.Unknown {
		res.Errors = append(res.Errors, "schema: unknown key "+k)
	}
	res.Errors = append(res.Errors, p.Errors...)
	if len(res.Errors) > 0 || root == "" {
		return res
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		res.Errors = append(res.Errors, "not JSON: "+err.Error())
		return res
	}
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	d := dryChecker{m: m, root: root, services: map[string]map[string]any{}}
	for name, s := range obj(m["services"]) {
		d.services[name] = obj(s)
	}
	d.check()
	res.Warnings = d.warns
	return res
}

func obj(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func sorted[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// isPerService reports whether a command value is a per-service map.
func isPerService(v any) bool {
	m, ok := v.(map[string]any)
	if !ok {
		return false
	}
	_, hasRun := m["run"]
	return !hasRun
}
