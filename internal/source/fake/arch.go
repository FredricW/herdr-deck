package fake

import (
	"context"
	"maps"
	"sync"

	"github.com/FredricW/herdr-deck/internal/deck"
	"github.com/FredricW/herdr-deck/internal/source/arch"
)

// Arch is the Impact tab's sample: what t-0002's branch did to the made-up
// webshop's shape, read from the files below as the deck reads a real
// repository. It matches the Files sample: UsersPage changed, UsersTable
// is new, UsersList is gone, stats.ts became overview.ts and the users API
// changed (plus package.json, for the new dependency). Other threads
// changed nothing.
func Arch(_ context.Context, t deck.Thread) *arch.Result {
	if t.ID != "t-0002" {
		return &arch.Result{Base: "origin/main", Note: "no changes to the shape"}
	}
	return sampleArch()
}

var sampleArch = sync.OnceValue(func() *arch.Result {
	after := maps.Clone(webshopBase)
	maps.Copy(after, webshopChange)
	for p, src := range after {
		if src == "" {
			delete(after, p)
		}
	}
	return arch.FromFiles("webshop", "origin/main", webshopBase, after, arch.Config{})
})

// webshopBase is the sample repository at the merge-base.
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
	"src/admin/users/UsersPage.tsx": `import { fetchUsers } from '@/api/users'
import { UsersList } from './UsersList'
import { userStats } from './stats'
export function UsersPage() { return UsersList(fetchUsers(), userStats()) }
`,
	"src/admin/users/UsersList.tsx": `import { Button } from '@/ui/Button'
export function UsersList(users: unknown, stats: unknown) { return Button(users, stats) }
`,
	"src/admin/users/stats.ts": `import { formatCount } from '@/format/numbers'
export function userStats() { return formatCount(0) }
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
	"src/api/users.ts": `import { get } from './client'
export function fetchUsers() { return get('/users') }
`,
	"src/api/members.ts": `import { get } from './client'
export function fetchMembers() { return get('/members') }
`,
	"src/ui/Button.tsx":     "export function Button(...args: unknown[]) { return args }\n",
	"src/format/numbers.ts": "export function formatCount(n: number) { return String(n) }\n",
	"src/format/dates.ts":   "export function formatDate(d: Date) { return d.toISOString() }\n",
	"src/db/index.ts": `export function query(sql: string) { return sql }
export function userCounts() { return 0 }
`,
}

// webshopChange is t-0002's branch: files it adds or changes, and "" for
// the ones it deletes.
var webshopChange = map[string]string{
	"package.json": `{"name": "webshop", "dependencies": {"react": "19.1.0", "@tanstack/react-table": "8.21.3"}}`,
	"src/admin/users/UsersPage.tsx": `import { fetchUsers, fetchOverview } from '@/api/users'
import { userCounts } from '@/db'
import { UsersTable } from './UsersTable'
import { overview } from './overview'
const pageSize = Number(import.meta.env.VITE_USERS_PAGE_SIZE)
export function UsersPage() {
  const counts = userCounts()
  return UsersTable(fetchUsers(pageSize), overview(fetchOverview(), counts))
}
`,
	"src/admin/users/UsersTable.tsx": `import { useReactTable } from '@tanstack/react-table'
import { Button } from '@/ui/Button'
import { formatDate } from '@/format/dates'
export function UsersTable(users: unknown, overview: unknown) {
  return Button(useReactTable(users), overview, formatDate(new Date()))
}
`,
	"src/admin/users/UsersList.tsx": "",
	"src/admin/users/stats.ts":      "",
	"src/admin/users/overview.ts": `import { formatCount } from '@/format/numbers'
export function overview(data: unknown, counts: number) { return formatCount(counts) + String(data) }
`,
	// The API reaching up into a page for a helper: the upward edge, and
	// with the pages' imports of the API, a cycle.
	"src/api/users.ts": `import { get } from './client'
import { overview } from '@/admin/users/overview'
export function fetchUsers(limit?: number) { return get('/users?limit=' + limit) }
export function fetchOverview() { return overview(fetch("https://api.example.com/admin/users/overview"), 0) }
`,
}
