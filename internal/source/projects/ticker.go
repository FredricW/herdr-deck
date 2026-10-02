package projects

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// prSummary is one entry of ticker.json's `prs`, keyed by thread id
// (herdr-projects src/pr.rs, Summary).
type prSummary struct {
	State          string   `json:"state"`
	ReviewDecision string   `json:"review_decision"`
	FailingChecks  []string `json:"failing_checks"`
	CommentCount   int      `json:"comment_count"`
	Commenters     []string `json:"commenters"`
}

type tickerFile struct {
	PRs map[string]prSummary `json:"prs"`
}

// readTicker reads the PR summaries the herdr-projects ticker keeps in
// .state/ticker.json.
func (r Reader) readTicker(note func(string, ...any)) map[string]prSummary {
	data, err := os.ReadFile(filepath.Join(r.Dir(), ".state", "ticker.json"))
	if err != nil {
		note(".state/ticker.json: %v", short(err))
		return nil
	}
	var f tickerFile
	if err := json.Unmarshal(data, &f); err != nil {
		note(".state/ticker.json: %v", err)
		return nil
	}
	return f.PRs
}
