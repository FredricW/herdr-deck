// Package schema holds the JSON Schemas of the dev manifest
// (docs/dev-manifest.md). Its test checks the schemas against the spec's
// examples, so the two cannot drift apart.
package schema

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const spec = "../docs/dev-manifest.md"

// compile loads a schema file, which also checks it against the draft
// 2020-12 meta-schema.
func compile(t *testing.T, file string) *jsonschema.Schema {
	t.Helper()
	doc, err := readJSON(file)
	if err != nil {
		t.Fatalf("%s: %v", file, err)
	}
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	c.AssertFormat()
	if err := c.AddResource(file, doc); err != nil {
		t.Fatalf("%s: %v", file, err)
	}
	s, err := c.Compile(file)
	if err != nil {
		t.Fatalf("%s: %v", file, err)
	}
	return s
}

func readJSON(file string) (any, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return jsonschema.UnmarshalJSON(bytes.NewReader(b))
}

// exampleRe finds the spec's checked examples: a ```json block right after
// an `<!-- example: <kind> -->` line.
var exampleRe = regexp.MustCompile("(?s)<!-- example: ([a-z.-]+) -->\n```json\n(.*?)\n```")

func TestSpecExamples(t *testing.T) {
	b, err := os.ReadFile(spec)
	if err != nil {
		t.Fatal(err)
	}
	schemas := map[string]*jsonschema.Schema{
		"dev.json":       compile(t, "v1/dev.schema.json"),
		"dev-ports.json": compile(t, "v1/dev-ports.schema.json"),
	}
	counts := map[string]int{}
	for i, m := range exampleRe.FindAllStringSubmatch(string(b), -1) {
		kind, body := m[1], m[2]
		s, ok := schemas[kind]
		if !ok {
			t.Errorf("example %d: unknown kind %q", i+1, kind)
			continue
		}
		counts[kind]++
		doc, err := jsonschema.UnmarshalJSON(strings.NewReader(body))
		if err != nil {
			t.Errorf("example %d (%s): not JSON: %v", i+1, kind, err)
			continue
		}
		if err := s.Validate(doc); err != nil {
			t.Errorf("example %d (%s) does not match the schema:\n%v", i+1, kind, err)
		}
	}
	// A broken marker would silently skip an example.
	if counts["dev.json"] < 4 || counts["dev-ports.json"] < 1 {
		t.Errorf("found %v checked examples, want at least 4 dev.json and 1 dev-ports.json", counts)
	}
}

func TestInvalidManifests(t *testing.T) {
	s := compile(t, "v1/dev.schema.json")
	files, err := filepath.Glob("testdata/invalid/*.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no files in testdata/invalid")
	}
	for _, f := range files {
		doc, err := readJSON(f)
		if err != nil {
			t.Errorf("%s: not JSON: %v", f, err)
			continue
		}
		if err := s.Validate(doc); err == nil {
			t.Errorf("%s: the schema accepts it", f)
		}
	}
}

// TestSchemasAreJSON keeps the schema files plain JSON that encoding/json
// reads, as tools without a schema library will.
func TestSchemasAreJSON(t *testing.T) {
	for _, f := range []string{"v1/dev.schema.json", "v1/dev-ports.schema.json"} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var v map[string]any
		if err := json.Unmarshal(b, &v); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
}
