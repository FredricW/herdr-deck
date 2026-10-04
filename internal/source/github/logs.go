package github

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/FredricW/herdr-deck/internal/deck"
)

// TailLines is how many lines of the failing step a CheckLog keeps.
const TailLines = 200

// ErrNoLog says a check has no log the deck can read: it is not a GitHub
// Actions job (another app's check run, or a commit status).
var ErrNoLog = errors.New("not a GitHub Actions job, so there is no log to show")

// Log returns the tail of check's log, the failing step's last lines. The
// check belongs to the PR at prURL. It downloads the job's log now (never
// on a refresh) and caches a finished job's tail by its ID, since that log
// no longer changes. Logs stay in memory; they may hold secrets GitHub
// failed to mask.
func (r *Reader) Log(ctx context.Context, prURL string, check deck.Check) (deck.CheckLog, error) {
	if check.JobID == 0 {
		return deck.CheckLog{}, ErrNoLog
	}
	ref, ok := parsePR(prURL)
	if !ok {
		return deck.CheckLog{}, errors.New("cannot tell the repository from " + prURL)
	}
	r.mu.Lock()
	cached, ok := r.logs[check.JobID]
	r.mu.Unlock()
	if ok {
		return cached, nil
	}
	ctx, cancel := context.WithTimeout(ctx, LogTimeout)
	defer cancel()
	path := "repos/" + ref.Owner + "/" + ref.Repo + "/actions/jobs/" + strconv.FormatInt(check.JobID, 10) + "/logs"
	// Newer gh refuses to print a log's colour codes without the flag;
	// older gh does not know it. Tail strips them either way.
	out, err := r.run(ctx, []string{"api", "--hostname", ref.Host, "--allow-escape-sequences", path}, maxLog)
	var re *RunError
	if errors.As(err, &re) && strings.Contains(re.Stderr, "unknown flag") {
		out, err = r.run(ctx, []string{"api", "--hostname", ref.Host, path}, maxLog)
	}
	if err != nil {
		if errors.As(err, &re) && strings.Contains(re.Stderr, "HTTP 410") {
			return deck.CheckLog{}, errors.New("GitHub no longer keeps this job's log")
		}
		return deck.CheckLog{}, errors.New(strings.TrimPrefix(r.ghError(err).msg, "off, "))
	}
	log := Tail(string(out), TailLines)
	if check.State == deck.CheckFailed || check.State == deck.CheckPassed {
		r.mu.Lock()
		if r.logs == nil {
			r.logs = map[int64]deck.CheckLog{}
		}
		r.logs[check.JobID] = log
		r.mu.Unlock()
	}
	return log, nil
}

// stamp is the timestamp GitHub puts before every log line.
var stamp = regexp.MustCompile(`^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(\.\d+)?Z ?`)

// Tail cuts a job's log down to its failing step: from the step's
// `##[group]Run …` line (the step's own header group left out) to the end
// of the step, keeping the last max lines. Timestamps, colour codes and
// group markers go; `##[error]` becomes "Error: ". Without an error line
// it keeps the log's last lines.
func Tail(log string, max int) deck.CheckLog {
	log = strings.TrimPrefix(log, "\ufeff")
	raw := strings.Split(strings.ReplaceAll(log, "\r\n", "\n"), "\n")
	lines := make([]string, 0, len(raw))
	for _, l := range raw {
		l = stamp.ReplaceAllString(l, "")
		lines = append(lines, strings.TrimRight(ansi.Strip(l), " \t\r"))
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	errAt := -1
	for i, l := range lines {
		if strings.HasPrefix(l, "##[error]") {
			errAt = i
			break
		}
	}
	var out deck.CheckLog
	from, to := 0, len(lines)
	if errAt >= 0 {
		for i := errAt - 1; i >= 0; i-- {
			if strings.HasPrefix(lines[i], "##[group]Run ") {
				out.Step = strings.TrimSpace(strings.TrimPrefix(lines[i], "##[group]Run "))
				from = i + 1
				// The header group holds the step's script and env.
				for j := i + 1; j < errAt; j++ {
					if lines[j] == "##[endgroup]" {
						from = j + 1
						break
					}
				}
				break
			}
		}
		for i := errAt + 1; i < len(lines); i++ {
			if strings.HasPrefix(lines[i], "##[group]Run ") || strings.HasPrefix(lines[i], "Post job cleanup.") {
				to = i
				break
			}
		}
	}
	var kept []string
	for _, l := range lines[from:to] {
		switch {
		case l == "##[endgroup]":
			continue
		case strings.HasPrefix(l, "##[group]"):
			l = strings.TrimPrefix(l, "##[group]")
		case strings.HasPrefix(l, "##[error]"):
			l = "Error: " + strings.TrimPrefix(l, "##[error]")
		case strings.HasPrefix(l, "##[warning]"):
			l = "Warning: " + strings.TrimPrefix(l, "##[warning]")
		}
		kept = append(kept, l)
	}
	if len(kept) > max {
		out.More = len(kept) - max
		kept = kept[len(kept)-max:]
	}
	out.Lines = kept
	return out
}
