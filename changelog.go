// Package herdrdeck holds files from the repository's root that the deck
// embeds.
package herdrdeck

import _ "embed"

// Changelog is CHANGELOG.md, which the deck's What's new view shows.
//
//go:embed CHANGELOG.md
var Changelog string
