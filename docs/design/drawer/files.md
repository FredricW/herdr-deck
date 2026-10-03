# Drawer · Files tab

Today's Files section, given a tab of its own and drawn as a diffstat: per
file a status letter, coloured `+N -M` and a bar proportional to the lines
changed. The letters, colours, count rules and the list/tree toggle are the
ones from the open Files PR (#23): `A` added green, `M` modified yellow, `D`
deleted red, `R` renamed cyan, `?` untracked faint green; an added or
untracked file shows only `+N`, a deleted one only `-M`. Conventions are in
the [README](README.md).

The data is unchanged: `git diff --raw --numstat -z -M <merge-base>` plus
untracked files, read off the UI goroutine as today. Only the drawing is new.

## List view

80 columns, normal height:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Admin rebuild                                               ◐ 2  ◇ 1  ○ 1  ✉ 0 │
│────────────────────────────────────────────────────────────────────────────────│
│    WORK                         THREAD  STATUS              PR      LINKS  DEV │
│  In progress                                                                   │
│ ▸◐ Users page                   t-0002  Building overview           L4         │
│  ◐ Templates page               t-0003  Writing tests               L F    ●●● │
│  ◇ Document select for summary  t-0004  review required     #2320   L F3   ●○  │
│                                                                                │
│  On hold until Monday 2026-10-05                                               │
│  ○ Subscriptions list           t-0001  idle                        L N    ~○  │
│                                                                                │
│  + Backlog (3)                                                                 │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ Users page                                                     ◐ working  ~60% │
│ t-0002 · Members /admin/users · Building overview                   ▰▰▰▰▰▰▱▱▱▱ │
│── Overview ──━┫ Files 11 ┣━── Log 7 ──────────────────────────────── ↓ 7 more ─│
│ 11 files  +447 -68  vs origin/main                               list · t tree │
│ 1 M  apps/admin/src/pages/users/UsersOverviewPage.tsx       +214 -12  ▇▇▇▇▇▇▇▇ │
│ 2 A  apps/admin/src/pages/users/columns.ts                       +48  ▇▇▁▁▁▁▁▁ │
│ 3 M  apps/admin/src/api/users.ts                              +31 -9  ▇▇▁▁▁▁▁▁ │
│ 4 R  apps/admin/src/pages/{members → users}/index.ts           +1 -1  ▇▁▁▁▁▁▁▁ │
│────────────────────────────────────────────────────────────────────────────────│
│ 1-9 file  d d whole diff  t tree  [ ] tab  z drawer  ? help             v9.9.9 │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns, normal height:

```text
┌────────────────────────────────────────────────────────────┐
│ Admin rebuild                           ◐ 2  ◇ 1  ○ 1  ✉ 0 │
│────────────────────────────────────────────────────────────│
│    WORK                        STATUS           LINKS  DEV │
│  In progress                                               │
│ ▸◐ Users page                  Building overv…  L4         │
│  ◐ Templates page              Writing tests    L F    ●●● │
│  ◇ Document select for summa…  #2320 review     L F3   ●○  │
│                                                            │
│  On hold until Monday 2026-10-05                           │
│  ○ Subscriptions list          idle             L N    ~○  │
│                                                            │
│  + Backlog (3)                                             │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│────────────────────────────────────────────────────────────│
│ Users page                                 ◐ working  ~60% │
│ t-0002 · Building overview                      ▰▰▰▰▰▰▱▱▱▱ │
│── Overview ──━┫ Files 11 ┣━── Log 7 ──────────── ↓ 7 more ─│
│ 11 files  +447 -68  vs origin/main           list · t tree │
│ 1 M  …c/pages/users/UsersOverviewPage.tsx  +214 -12  ▇▇▇▇▇ │
│ 2 A  …ps/admin/src/pages/users/columns.ts       +48  ▇▇▁▁▁ │
│ 3 M  apps/admin/src/api/users.ts             +31 -9  ▇▁▁▁▁ │
│ 4 R  …rc/pages/{members → users}/index.ts     +1 -1  ▇▁▁▁▁ │
│────────────────────────────────────────────────────────────│
│ 1-9 file  d d diff  t tree  [ ] tab  ? help         v9.9.9 │
└────────────────────────────────────────────────────────────┘
```

- **Total line.** `11 files` plain, `+447` green, `-68` red, `vs origin/main`
  dim; the right end says which view is on and how to switch, dim. A click
  on it opens the whole diff, as today.
- **File rows.** Digit bold, the letter in its colour (a binary file's letter
  faint), the path plain with its folder part dim, counts green/red. A long
  path loses its start (`…c/pages/users/…`), so the name stays. A rename
  folds its shared parts the way `git diff --stat` does:
  `apps/admin/src/pages/{members → users}/index.ts`.
- **Bar.** Eight cells at 80 columns, five at 60. Cells ∝ the file's added
  plus deleted lines, scaled to the largest file in the list; any change
  gets at least one cell. Added cells green, deleted red, in that order
  (`+214 -12` is seven green and one red), the rest a dim `▁`. A binary
  file has no bar.
- **Numbers.** Files 1–9 take the digits, in display order; the rest show
  a dim `·` and open by click or through the drawer cursor (below).

## Tree view (`t`)

80 columns, normal height:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Admin rebuild                                               ◐ 2  ◇ 1  ○ 1  ✉ 0 │
│────────────────────────────────────────────────────────────────────────────────│
│    WORK                         THREAD  STATUS              PR      LINKS  DEV │
│  In progress                                                                   │
│ ▸◐ Users page                   t-0002  Building overview           L4         │
│  ◐ Templates page               t-0003  Writing tests               L F    ●●● │
│  ◇ Document select for summary  t-0004  review required     #2320   L F3   ●○  │
│                                                                                │
│  On hold until Monday 2026-10-05                                               │
│  ○ Subscriptions list           t-0001  idle                        L N    ~○  │
│                                                                                │
│  + Backlog (3)                                                                 │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ Users page                                                     ◐ working  ~60% │
│ t-0002 · Members /admin/users · Building overview                   ▰▰▰▰▰▰▱▱▱▱ │
│── Overview ──━┫ Files 11 ┣━── Log 7 ─────────────────────────────── ↓ 17 more ─│
│ 11 files  +447 -68  vs origin/main                               tree · t list │
│       apps/admin/                                           +300 -27           │
│         public/                                                                │
│ 1 A       empty-state.png                                     binary           │
│         src/                                                +300 -27           │
│────────────────────────────────────────────────────────────────────────────────│
│ 1-9 file  d d whole diff  t list  [ ] tab  z drawer  ? help             v9.9.9 │
└────────────────────────────────────────────────────────────────────────────────┘
```

Full height, 80 columns:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Admin rebuild                                               ◐ 2  ◇ 1  ○ 1  ✉ 0 │
│────────────────────────────────────────────────────────────────────────────────│
│ Users page                                                     ◐ working  ~60% │
│ t-0002 · Members /admin/users · Building overview                   ▰▰▰▰▰▰▱▱▱▱ │
│── Overview ──━┫ Files 11 ┣━── Log 7 ───────────────────────────────────────────│
│ 11 files  +447 -68  vs origin/main                               tree · t list │
│       apps/admin/                                           +300 -27           │
│         public/                                                                │
│ 1 A       empty-state.png                                     binary           │
│         src/                                                +300 -27           │
│           api/                                                +31 -9           │
│ 2 M         users.ts                                          +31 -9  ▇▇▁▁▁▁▁▁ │
│           pages/                                            +263 -16           │
│             members/                                              -3           │
│ 3 D           legacy.css                                          -3  ▇▁▁▁▁▁▁▁ │
│             users/                                          +263 -13           │
│ 4 M           UsersOverviewPage.tsx                         +214 -12  ▇▇▇▇▇▇▇▇ │
│ 5 A           columns.ts                                         +48  ▇▇▁▁▁▁▁▁ │
│ 6 R           members/index.ts → index.ts                      +1 -1  ▇▁▁▁▁▁▁▁ │
│ 7 M         routes.tsx                                         +6 -2  ▇▁▁▁▁▁▁▁ │
│       docs/                                                      +19           │
│ 8 ?     users-page.md                                            +19  ▇▁▁▁▁▁▁▁ │
│       scripts/                                                   +25           │
│ 9 ?     seed-users.ts                                            +25  ▇▁▁▁▁▁▁▁ │
│ · M   package.json                                             +1 -1  ▇▁▁▁▁▁▁▁ │
│ · M   pnpm-lock.yaml                                        +102 -40  ▇▇▇▇▇▇▁▁ │
│────────────────────────────────────────────────────────────────────────────────│
│ 1-9 file  d d whole diff  t list  [ ] tab  z drawer  ? help             v9.9.9 │
└────────────────────────────────────────────────────────────────────────────────┘
```

Full height, 60 columns:

```text
┌────────────────────────────────────────────────────────────┐
│ Admin rebuild                           ◐ 2  ◇ 1  ○ 1  ✉ 0 │
│────────────────────────────────────────────────────────────│
│ Users page                                 ◐ working  ~60% │
│ t-0002 · Building overview                      ▰▰▰▰▰▰▱▱▱▱ │
│── Overview ──━┫ Files 11 ┣━── Log 7 ───────────────────────│
│ 11 files  +447 -68  vs origin/main           tree · t list │
│       apps/admin/                          +300 -27        │
│         public/                                            │
│ 1 A       empty-state.png                    binary        │
│         src/                               +300 -27        │
│           api/                               +31 -9        │
│ 2 M         users.ts                         +31 -9  ▇▁▁▁▁ │
│           pages/                           +263 -16        │
│             members/                             -3        │
│ 3 D           legacy.css                         -3  ▇▁▁▁▁ │
│             users/                         +263 -13        │
│ 4 M           UsersOverviewPage.tsx        +214 -12  ▇▇▇▇▇ │
│ 5 A           columns.ts                        +48  ▇▇▁▁▁ │
│ 6 R           …index.ts → index.ts            +1 -1  ▇▁▁▁▁ │
│ 7 M         routes.tsx                        +6 -2  ▇▁▁▁▁ │
│       docs/                                     +19        │
│ 8 ?     users-page.md                           +19  ▇▁▁▁▁ │
│       scripts/                                  +25        │
│ 9 ?     seed-users.ts                           +25  ▇▁▁▁▁ │
│ · M   package.json                            +1 -1  ▇▁▁▁▁ │
│ · M   pnpm-lock.yaml                       +102 -40  ▇▇▇▇▁ │
│────────────────────────────────────────────────────────────│
│ 1-9 file  d d diff  t list  [ ] tab  ? help         v9.9.9 │
└────────────────────────────────────────────────────────────┘
```

- As in PR #23: folders first, single-folder chains joined
  (`apps/admin/`), folder rows dim with their summed counts and no bar,
  files numbered in display order. A rename shows under its new folder.
- At the normal height the tree shows little (four rows here), which is
  why the list stays the default; `t` is for the full-height drawer.

## Many files

A thread with 23 changed files. 80 columns, normal height:

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Admin rebuild                                               ◐ 2  ◇ 1  ○ 1  ✉ 0 │
│────────────────────────────────────────────────────────────────────────────────│
│    WORK                         THREAD  STATUS              PR      LINKS  DEV │
│  In progress                                                                   │
│  ◐ Users page                   t-0002  Building overview           L4         │
│ ▸◐ Templates page               t-0003  Writing tests               L F    ●●● │
│  ◇ Document select for summary  t-0004  review required     #2320   L F3   ●○  │
│                                                                                │
│  On hold until Monday 2026-10-05                                               │
│  ○ Subscriptions list           t-0001  idle                        L N    ~○  │
│                                                                                │
│  + Backlog (3)                                                                 │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│                                                                                │
│────────────────────────────────────────────────────────────────────────────────│
│ Templates page                                                 ◐ working  ~40% │
│ t-0003 · Templates /templates · Writing tests · pane w20:p1         ▰▰▰▰▱▱▱▱▱▱ │
│── Overview ──━┫ Files 23 ┣━── Log 4 ─────────────────────────────── ↓ 19 more ─│
│ 23 files  +1143 -382  vs origin/main                             list · t tree │
│ 1 M  api/templates/router.py                                  +31 -6  ▇▇▁▁▁▁▁▁ │
│ 2 M  api/templates/schemas.py                                 +19 -4  ▇▁▁▁▁▁▁▁ │
│ 3 M  api/tests/test_templates.py                              +58 -2  ▇▇▁▁▁▁▁▁ │
│ 4 M  apps/admin/src/api/templates.ts                         +44 -12  ▇▇▁▁▁▁▁▁ │
│────────────────────────────────────────────────────────────────────────────────│
│ 1-9 file  d d whole diff  t tree  [ ] tab  z drawer  ? help             v9.9.9 │
└────────────────────────────────────────────────────────────────────────────────┘
```

60 columns, normal height:

```text
┌────────────────────────────────────────────────────────────┐
│ Admin rebuild                           ◐ 2  ◇ 1  ○ 1  ✉ 0 │
│────────────────────────────────────────────────────────────│
│    WORK                        STATUS           LINKS  DEV │
│  In progress                                               │
│  ◐ Users page                  Building overv…  L4         │
│ ▸◐ Templates page              Writing tests    L F    ●●● │
│  ◇ Document select for summa…  #2320 review     L F3   ●○  │
│                                                            │
│  On hold until Monday 2026-10-05                           │
│  ○ Subscriptions list          idle             L N    ~○  │
│                                                            │
│  + Backlog (3)                                             │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│                                                            │
│────────────────────────────────────────────────────────────│
│ Templates page                             ◐ working  ~40% │
│ t-0003 · Writing tests                          ▰▰▰▰▱▱▱▱▱▱ │
│── Overview ──━┫ Files 23 ┣━── Log 4 ─────────── ↓ 19 more ─│
│ 23 files  +1143 -382  vs origin/main         list · t tree │
│ 1 M  api/templates/router.py                 +31 -6  ▇▁▁▁▁ │
│ 2 M  api/templates/schemas.py                +19 -4  ▇▁▁▁▁ │
│ 3 M  api/tests/test_templates.py             +58 -2  ▇▇▁▁▁ │
│ 4 M  apps/admin/src/api/templates.ts        +44 -12  ▇▇▁▁▁ │
│────────────────────────────────────────────────────────────│
│ 1-9 file  d d diff  t tree  [ ] tab  ? help         v9.9.9 │
└────────────────────────────────────────────────────────────┘
```

Full height, 80 columns, with the cursor in the drawer (`tab`, then `j`
eleven times):

```text
┌────────────────────────────────────────────────────────────────────────────────┐
│ Admin rebuild                                               ◐ 2  ◇ 1  ○ 1  ✉ 0 │
│────────────────────────────────────────────────────────────────────────────────│
│ Templates page                                                 ◐ working  ~40% │
│ t-0003 · Templates /templates · Writing tests · pane w20:p1         ▰▰▰▰▱▱▱▱▱▱ │
│── Overview ──━┫ Files 23 ┣━── Log 4 ───────────────────────────────────────────│
│ 23 files  +1143 -382  vs origin/main                             list · t tree │
│ 1 M  api/templates/router.py                                  +31 -6  ▇▇▁▁▁▁▁▁ │
│ 2 M  api/templates/schemas.py                                 +19 -4  ▇▁▁▁▁▁▁▁ │
│ 3 M  api/tests/test_templates.py                              +58 -2  ▇▇▁▁▁▁▁▁ │
│ 4 M  apps/admin/src/api/templates.ts                         +44 -12  ▇▇▁▁▁▁▁▁ │
│ 5 M  apps/admin/src/api/types.ts                              +18 -2  ▇▁▁▁▁▁▁▁ │
│ 6 M  apps/admin/src/components/Editor/Toolbar.test.tsx        +15 -3  ▇▁▁▁▁▁▁▁ │
│ 7 M  apps/admin/src/components/Editor/Toolbar.tsx             +22 -9  ▇▇▁▁▁▁▁▁ │
│ 8 A  apps/admin/src/components/Editor/variables.test.ts          +41  ▇▇▁▁▁▁▁▁ │
│ 9 A  apps/admin/src/components/Editor/variables.ts               +37  ▇▇▁▁▁▁▁▁ │
│ · M  apps/admin/src/i18n/de.json                              +26 -1  ▇▁▁▁▁▁▁▁ │
│ · M  apps/admin/src/i18n/en.json                              +26 -1  ▇▁▁▁▁▁▁▁ │
│▸· D  apps/admin/src/legacy/templates/Templates.jsx              -212  ▇▇▇▇▇▇▇▇ │
│ · D  apps/admin/src/legacy/templates/templates.css               -88  ▇▇▇▁▁▁▁▁ │
│ · A  apps/admin/src/pages/templates/TemplateEditor.tsx          +240  ▇▇▇▇▇▇▇▇ │
│ · A  apps/admin/src/pages/templates/TemplateList.tsx             +96  ▇▇▇▇▁▁▁▁ │
│ · M  apps/admin/src/pages/templates/TemplatesPage.tsx       +188 -40  ▇▇▇▇▇▇▇▇ │
│ · A  apps/admin/src/pages/templates/columns.ts                   +52  ▇▇▁▁▁▁▁▁ │
│ · A  apps/admin/src/pages/templates/index.ts                      +3  ▇▁▁▁▁▁▁▁ │
│ · A  apps/admin/src/pages/templates/templates.test.tsx          +134  ▇▇▇▇▇▁▁▁ │
│ ↓ 4 more · d d opens the whole diff                                            │
│────────────────────────────────────────────────────────────────────────────────│
│ j k file  ↵ open  d d whole diff  t tree  esc list  ? help              v9.9.9 │
└────────────────────────────────────────────────────────────────────────────────┘
```

Full height, 60 columns:

```text
┌────────────────────────────────────────────────────────────┐
│ Admin rebuild                           ◐ 2  ◇ 1  ○ 1  ✉ 0 │
│────────────────────────────────────────────────────────────│
│ Templates page                             ◐ working  ~40% │
│ t-0003 · Writing tests                          ▰▰▰▰▱▱▱▱▱▱ │
│── Overview ──━┫ Files 23 ┣━── Log 4 ───────────────────────│
│ 23 files  +1143 -382  vs origin/main         list · t tree │
│ 1 M  api/templates/router.py                 +31 -6  ▇▁▁▁▁ │
│ 2 M  api/templates/schemas.py                +19 -4  ▇▁▁▁▁ │
│ 3 M  api/tests/test_templates.py             +58 -2  ▇▇▁▁▁ │
│ 4 M  apps/admin/src/api/templates.ts        +44 -12  ▇▇▁▁▁ │
│ 5 M  apps/admin/src/api/types.ts             +18 -2  ▇▁▁▁▁ │
│ 6 M  …/components/Editor/Toolbar.test.tsx    +15 -3  ▇▁▁▁▁ │
│ 7 M  …n/src/components/Editor/Toolbar.tsx    +22 -9  ▇▁▁▁▁ │
│ 8 A  …components/Editor/variables.test.ts       +41  ▇▁▁▁▁ │
│ 9 A  …/src/components/Editor/variables.ts       +37  ▇▁▁▁▁ │
│ · M  apps/admin/src/i18n/de.json             +26 -1  ▇▁▁▁▁ │
│ · M  apps/admin/src/i18n/en.json             +26 -1  ▇▁▁▁▁ │
│ · D  …/src/legacy/templates/Templates.jsx      -212  ▇▇▇▇▇ │
│ · D  …/src/legacy/templates/templates.css       -88  ▇▇▁▁▁ │
│ · A  …/pages/templates/TemplateEditor.tsx      +240  ▇▇▇▇▇ │
│ · A  …rc/pages/templates/TemplateList.tsx       +96  ▇▇▁▁▁ │
│ · M  …c/pages/templates/TemplatesPage.tsx  +188 -40  ▇▇▇▇▇ │
│ · A  …dmin/src/pages/templates/columns.ts       +52  ▇▇▁▁▁ │
│ · A  …/admin/src/pages/templates/index.ts        +3  ▇▁▁▁▁ │
│ · A  …/pages/templates/templates.test.tsx      +134  ▇▇▇▁▁ │
│ ↓ 4 more · d d opens the whole diff                        │
│────────────────────────────────────────────────────────────│
│ 1-9 file  d d diff  t tree  [ ] tab  ? help         v9.9.9 │
└────────────────────────────────────────────────────────────┘
```

- **`↓ K more`** replaces today's `+K more`: the tab scrolls, so the files
  past the drawer's end are a `pgdn` or a wheel turn away, and it says how
  many. Every file stays reachable, not only the first nine.
- **Bars across many files** scale to the largest file of the thread
  (`TemplateEditor.tsx`, 240 lines), so a big deletion
  (`Templates.jsx -212`, all red) stands out as much as a big addition.
- **The drawer cursor** (80-column full-height frame): after `tab` the
  drawer has the focus, `j`/`k` move a cursor over the files, and `↵` opens
  the file under it, numbered or not. In the frame it sits on file 12,
  `Templates.jsx`, marked `▸` and drawn with the selection background, like the
  list's cursor. The
  list's `▸` turns dim while the drawer has the focus. `esc` gives the focus
  back to the list.
- A missing worktree or base is one dim line under the tab bar, as today;
  a resolved thread has no Files tab count (`Files` dim, skipped).
