// Package schema holds the JSON Schemas of the dev manifest
// (docs/dev-manifest.md). Its test checks the schemas against the spec's
// examples, so the two cannot drift apart.
package schema

import "embed"

// FS holds the schema files, by their path in this folder
// (v1/dev.schema.json, v1/dev-ports.schema.json).
//
//go:embed v1/*.json
var FS embed.FS
