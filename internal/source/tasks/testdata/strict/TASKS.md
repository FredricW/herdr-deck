# Tasks

## v1

- [ ] M2 herdr-projects reader (claude) · t-0003
  Milestone 2, parallel with M3, after M1 lands. internal/source/projects: PROJECT.md, thread list --json with TOML fallback, ticker.json PRs, inbox. Synthetic fixtures in the same shapes.
- [ ] M3 Tasks and links (claude) · t-0004
  Milestone 3, parallel with M2, after M1 lands. internal/source/tasks: loose TASKS.md parser and link scraper; tests cover the strict format and a loose hand-written file.
- [ ] M4 Live UI
  Milestone 4, after M2, M3 and the chosen design (t-0002). Wire sources into the UI, fsnotify plus 5 s tick, link keys and mouse open URLs.
- [ ] M5 herdr live state
  Milestone 5. Snapshot plus socket subscription (verify streaming first, else poll), pane focus on enter, needs-you from live agent state.
- [ ] M6 Dev servers
  Milestone 6. Reader for herdr-deck's own dev-server manifest and a port probe, per-thread localhost links, port token fallback.
- [ ] M7 herdr plugin
  Milestone 7. herdr-plugin.toml, toggle action, auto-open event, split sizing, install docs. Ask the user before editing their herdr config.

## Backlog

- [ ] M8 Polish
  Milestone 8. Linear issue status next to IDs, u to start dev servers, link handlers, multi-project view.
