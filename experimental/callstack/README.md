# callstack: throwaway prototype

Not part of the deck. This prototype backs the design doc
[docs/research/call-stack-tab.md](../../docs/research/call-stack-tab.md)
(the Flows tab: which entry-point flows behave differently).

It lives in its own Go module, so `golang.org/x/tools` stays out of the
deck's `go.mod`, and the root's `go vet ./...` and `go test ./...` skip
it.

The command:
1. writes two revisions of a Go module into a temp folder with
   `git archive`, so the repo is not touched;
2. type-checks both with go/packages, with `GOPROXY=off`, so it never
   downloads;
3. diffs the call trees of the entry points the change reaches;
4. prints the tab's text at the widths you ask for.

```sh
go run . -repo ../.. -base 0937435^1 -head 0937435 -width 60,80,120
go run . -repo ../.. -base 9fc47b3^1 -head 9fc47b3 -width 80 -unknown
```

Flags:

| Flag | Meaning |
|---|---|
| `-top N` | number of rows in the look-here-first list |
| `-flows N` | number of flow trees to print |
| `-unknown` | list the external calls the side-effect rule table can't classify |

Timings go to stderr.

| File | Does |
|---|---|
| `load.go` | materialize, load, per-function call sites in source order with context; interface calls (CHA); callbacks bound to struct fields and local aliases; `Update` / `handleKey` cases split into entry points |
| `effects.go` | the side-effect rule table and its propagation (all, and synchronous only) |
| `diff.go` | changed / added / removed / renamed functions, refactor check, LCS of call sites, reachability to entries, risk score |
| `render.go` | look here first, flow trees with breadcrumbs, hubs ("shared frames"), new-subtree summaries, who reaches this |

The entry points are hard-coded for herdr-deck (`defaultEntries`).
`samples/` holds the output for PR #53, for PR #52, and for a synthetic
commit that adds a file read to the render path.
