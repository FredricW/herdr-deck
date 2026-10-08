package fake

import (
	"context"
	"fmt"
	"maps"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/FredricW/herdr-deck/internal/deck"
	"github.com/FredricW/herdr-deck/internal/source/arch"
)

// Arch is the Impact tab's sample: what t-0002's branch did to the made-up
// webshop's shape, read from the files below as the deck reads a real
// repository. It matches the Files sample, its changed files built from
// the sample patches: UsersPage changed, UsersTable is new, UsersList is
// gone, stats.ts became overview.ts and the users API changed (plus
// package.json, for the new dependency). Other threads changed nothing.
func Arch(_ context.Context, t deck.Thread) *arch.Result {
	if t.ID != "t-0002" {
		return &arch.Result{Base: "origin/main", Note: "no changes to the shape"}
	}
	return sampleArch()
}

var sampleArch = sync.OnceValue(func() *arch.Result {
	return arch.FromFiles("webshop", "origin/main", sampleFiles(false), sampleFiles(true), arch.Config{})
})

// FileAt is a file of the sample repository as commit rev has it: the
// merge-base ("0000000") or the branch's head, for the Impact tab's view
// of an import the branch did not change. ReadFileAt's sample.
func FileAt(_ context.Context, _ deck.Thread, rev, path string) ([]byte, error) {
	src, ok := sampleFiles(rev != sampleArch().MergeBase)[path]
	if !ok {
		return nil, fmt.Errorf("%s: not in %s", path, rev)
	}
	return []byte(src), nil
}

// sampleFiles is the sample repository at the merge-base or at the head:
// the files below, and the changed ones as the Files tab's sample patches
// show them, so a line the Impact tab points at is the line the diff has.
func sampleFiles(head bool) map[string]string {
	files := maps.Clone(webshopBase)
	if head {
		maps.Copy(files, webshopChange)
	}
	for _, p := range []string{"src/admin/users/UsersPage.tsx", "src/api/users.ts", "src/admin/users/UsersList.tsx", "src/admin/users/UsersTable.tsx"} {
		old, now := patchSides(patches[p])
		if head {
			old = now
		}
		files[p] = old
	}
	// stats.ts became overview.ts.
	old, now := patchSides(patches["src/admin/users/overview.ts"])
	if head {
		files["src/admin/users/overview.ts"] = now
	} else {
		files["src/admin/users/stats.ts"] = old
	}
	for p, src := range files {
		if src == "" {
			delete(files, p)
		}
	}
	return files
}

var hunkHeader = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

// patchSides rebuilds the old and the new file from a patch's hunks, with
// blank lines where the hunks show nothing; "" for a side the file does
// not have (an added file has no old side).
func patchSides(patch string) (old, now string) {
	olds, news := map[int]string{}, map[int]string{}
	var o, n int
	hasOld, hasNew := false, false
	for line := range strings.Lines(patch) {
		line = strings.TrimSuffix(line, "\n")
		if m := hunkHeader.FindStringSubmatch(line); m != nil {
			o, _ = strconv.Atoi(m[1])
			n, _ = strconv.Atoi(m[3])
			hasOld = hasOld || m[1] != "0" || m[2] != "0"
			hasNew = hasNew || m[3] != "0" || m[4] != "0"
			continue
		}
		if line == "" {
			continue
		}
		switch text := line[1:]; line[0] {
		case ' ':
			olds[o], news[n] = text, text
			o++
			n++
		case '-':
			olds[o] = text
			o++
		case '+':
			news[n] = text
			n++
		}
	}
	join := func(m map[int]string, has bool) string {
		if !has {
			return ""
		}
		last := 0
		for k := range m {
			last = max(last, k)
		}
		var b strings.Builder
		for i := 1; i <= last; i++ {
			b.WriteString(m[i] + "\n")
		}
		return b.String()
	}
	return join(olds, hasOld), join(news, hasNew)
}

// webshopBase is the sample repository at the merge-base, but for the
// files the sample patches change (sampleFiles adds those).
var webshopBase = map[string]string{
	".config/dev.json": `{
  "services": { "frontend": { "run": "pnpm dev", "ports": ["frontend"] } },
  "x-herdr-deck": {
    "architecture": {
      "roots": ["src"],
      "layers": [
        { "name": "entry", "paths": ["src"] },
        { "name": "pages", "paths": ["src/admin/**"] },
        { "name": "shared", "paths": ["src/ui/**", "src/format/**"] },
        { "name": "api", "paths": ["src/api/**"], "closed": true },
        { "name": "infra", "paths": ["src/db/**"] }
      ]
    }
  }
}
`,
	"package.json":  `{"name": "webshop", "dependencies": {"react": "19.1.0"}}`,
	"tsconfig.json": `{"compilerOptions": {"baseUrl": ".", "paths": {"@/*": ["src/*"]}}}`,
	"src/main.tsx": `import { routes } from './routes'
export function main() { return routes }
`,
	"src/routes.tsx": `import { UsersPage } from '@/admin/users/UsersPage'
import { MembersPage } from '@/admin/members/MembersPage'
import { TemplatesPage } from '@/admin/templates/TemplatesPage'
export const routes = [UsersPage, MembersPage, TemplatesPage]
`,
	"src/admin/members/MembersPage.tsx": `import { fetchMembers } from '@/api/members'
import { Button } from '@/ui/Button'
export function MembersPage() { return Button(fetchMembers()) }
`,
	"src/admin/templates/TemplatesPage.tsx": `import { Button } from '@/ui/Button'
export function TemplatesPage() { return Button() }
`,
	"src/admin/settings/SettingsPage.tsx": `import { Button } from '@/ui/Button'
export function SettingsPage() { return Button() }
`,
	"src/api/client.ts": `import { query } from '@/db'
export function get(path: string) { return query(path) }
`,
	"src/api/members.ts": `import { get } from './client'
export function fetchMembers() { return get('/members') }
`,
	"src/ui/Button.tsx":     "export function Button(...args: unknown[]) { return args }\n",
	"src/ui/Table.tsx":      "export function Table(props: unknown) { return props }\nexport type Column<T> = { key: keyof T }\n",
	"src/format/numbers.ts": "export function formatCount(n: number) { return String(n) }\n",
	"src/format/dates.ts":   "export function formatDate(d: Date) { return d.toISOString() }\n",
	"src/db/index.ts": `export function query(sql: string) { return sql }
export function userCounts() { return 0 }
`,
}

// webshopChange is the rest of t-0002's branch: the new dependency.
var webshopChange = map[string]string{
	"package.json": `{"name": "webshop", "dependencies": {"react": "19.1.0", "@tanstack/react-table": "8.21.3"}}`,
}
