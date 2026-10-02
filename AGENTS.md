# Agent notes

- The plan and its milestones are in `docs/PLAN.md`. Read it before starting; update it when a decision changes.
- Stack: Go + Bubble Tea v2, Bubbles, Lip Gloss. Data code lives in `internal/source/*` and never imports the UI.
- Read-only towards herdr-projects: never write under `~/.herdr-projects`, and never run `herdr-projects context` without `--peek`.
- Do not edit the user's herdr config (`~/.config/herdr/config.toml`) without asking.
- Before finishing: `gofmt -l .` is empty, `go vet ./...` and `go test ./...` pass.
