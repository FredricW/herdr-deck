package projects

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
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
	PRs         map[string]prSummary `json:"prs"`
	LastPRCheck string               `json:"last_pr_check"`
}

// ticker is what the deck uses of ticker.json: the PR summaries by thread
// id, and when the ticker last asked GitHub about them.
type ticker struct {
	prs     map[string]prSummary
	checked time.Time
}

// readTicker reads the PR summaries the herdr-projects ticker keeps in
// .state/ticker.json.
func (r Reader) readTicker(note func(string, ...any)) ticker {
	data, err := os.ReadFile(filepath.Join(r.Dir(), ".state", "ticker.json"))
	if err != nil {
		note(".state/ticker.json: %v", short(err))
		return ticker{}
	}
	var f tickerFile
	if err := json.Unmarshal(data, &f); err != nil {
		note(".state/ticker.json: %v", err)
		return ticker{}
	}
	return ticker{prs: f.PRs, checked: stamp(f.LastPRCheck)}
}
