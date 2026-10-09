package fake

import (
	"context"
	"fmt"

	"github.com/FredricW/herdr-deck/internal/deck"
	"github.com/FredricW/herdr-deck/internal/source/diff"
)

// Patch is the diff preview's sample: made-up diffs of t-0002's files in
// a few languages, as `git diff` would print them.
func Patch(_ context.Context, t deck.Thread, _ string, f deck.DiffFile) deck.Patch {
	if t.ID != "t-0002" {
		return deck.Patch{Note: "no changes"}
	}
	if f.Binary {
		return deck.Patch{Binary: true}
	}
	body, ok := patches[f.Path]
	if !ok {
		body = fmt.Sprintf("@@ -1 +1 @@\n-// %s\n+// %s, updated\n", f.Path, f.Path)
	}
	return diff.ParsePatch([]byte(body), diff.MaxPatchLines)
}

var patches = map[string]string{
	"src/admin/users/UsersPage.tsx": `@@ -1,14 +1,24 @@
 import { useState } from "react";
-import { UsersList } from "./UsersList";
+import { UsersTable } from "./UsersTable";
+import { useUsers } from "../../api/users";
+import { overview } from "./overview";
+import { userCounts } from "@/db";
 
-// Members page: a plain list of everyone in the workspace.
-export function MembersPage() {
-  const [query, setQuery] = useState("");
-  return <UsersList query={query} onQuery={setQuery} />;
+// Users page: the table, its filters and the overview cards (ABC-1256).
+export function UsersPage() {
+  const [query, setQuery] = useState("");
+  const [role, setRole] = useState<Role | "all">("all");
+  const pageSize = Number(import.meta.env.VITE_USERS_PAGE_SIZE);
+  const { users, loading } = useUsers({ query, role, pageSize });
+  const stats = overview(users, userCounts());
+
+  return (
+    <Page title="Users" actions={<InviteButton />}>
+      <OverviewCards active={stats.active} invited={stats.invited} />
+      <UsersTable users={users} loading={loading} onQuery={setQuery} onRole={setRole} />
+    </Page>
+  );
 }
@@ -40,7 +50,9 @@ function InviteButton() {
   const open = useInviteDialog();
   return (
-    <Button onClick={open}>Invite member</Button>
+    <Button onClick={open} icon="plus">
+      Invite user
+    </Button>
   );
 }
`,
	"src/api/users.ts": `@@ -1,5 +1,25 @@
 import { get } from "./client";
+import { overview } from "@/admin/users/overview";
 
-export async function listMembers(): Promise<Member[]> {
-  return get("/api/members");
+export type Role = "owner" | "admin" | "member";
+
+export interface UsersQuery {
+  query?: string;
+  role?: Role | "all";
+}
+
+// listUsers pages through /api/users; the server filters by role.
+export async function listUsers(q: UsersQuery = {}): Promise<User[]> {
+  const params = new URLSearchParams();
+  if (q.query) params.set("q", q.query);
+  if (q.role && q.role !== "all") params.set("role", q.role);
+  return get(` + "`/api/users?${params}`" + `);
+}
+
+export function useUsers(q: UsersQuery) {
+  return useQuery(["users", q], () => listUsers(q), { staleTime: 30_000 });
 }
+
+export async function fetchOverview() {
+  return overview(await fetch("https://api.example.com/admin/users/overview"));
+}
`,
	"src/admin/users/overview.ts": `@@ -1,9 +1,12 @@
-export function stats(members: Member[]) {
-  let active = 0;
-  for (const m of members) if (m.active) active++;
-  return { active };
+// overview counts what the Users page's cards show.
+export function overview(users: User[]) {
+  const active = users.filter((u) => u.status === "active").length;
+  const invited = users.filter((u) => u.status === "invited").length;
+  return { active, invited, total: users.length };
 }
`,
	"docs/users-page.md": `@@ -0,0 +1,21 @@
+# Users page
+
+The admin's **Users** page replaces *Members* (ABC-1256).
+
+## Layout
+
+- Overview cards: active and invited users.
+- The table: name, email, role, last seen.
+- Filters: a search box and the role menu.
+
+## Open questions
+
+1. Do invited users count towards the seat limit?
+2. Should owners be able to demote themselves?
+
+` + "```ts" + `
+const stats = overview(users);
+` + "```" + `
+
+See the Figma file for the empty state.
\ No newline at end of file
`,
	"src/admin/users/UsersList.tsx": `@@ -1,8 +0,0 @@
-import { Member } from "../../api/members";
-
-export function UsersList({ members }: { members: Member[] }) {
-  return (
-    <ul>{members.map((m) => <li key={m.id}>{m.name}</li>)}</ul>
-  );
-}
-
`,
	"src/admin/users/UsersTable.tsx": `@@ -0,0 +1,13 @@
+import { useReactTable } from "@tanstack/react-table";
+import { Table, Column } from "@/ui/Table";
+
+const columns: Column<User>[] = [
+  { key: "name", title: "Name", sortable: true },
+  { key: "email", title: "Email" },
+  { key: "role", title: "Role", width: 120 },
+  { key: "lastSeen", title: "Last seen", render: (u) => ago(u.lastSeen) },
+];
+
+export function UsersTable({ users, loading }: Props) {
+  return <Table rows={users} columns={columns} loading={loading} />;
+}
`,
	"src/admin/users/users.test.tsx": `@@ -0,0 +1,10 @@
+import { render, screen } from "@testing-library/react";
+import { UsersPage } from "./UsersPage";
+
+test("shows the overview cards", async () => {
+  render(<UsersPage />);
+  expect(await screen.findByText("Active")).toBeVisible();
+  expect(screen.getByText("Invited")).toBeVisible();
+});
+
+test.todo("filters by role");
`,
}
