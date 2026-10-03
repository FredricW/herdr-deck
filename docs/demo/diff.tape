# The Files section: what a thread's worktree changed, coloured by kind,
# and d to open one file or the whole diff in the diff tool (a no-op in
# the demo); d t switches to the folder tree. Render with `make demo`.
Output docs/demo/diff.gif
Source docs/demo/setup.tape

Hide
Type "herdr-deck --fake" Enter
Sleep 1s
Show
Sleep 1.5s
# z makes the drawer the whole pane, so every file shows.
Type "z"
Sleep 2.2s
# d asks which file: a digit opens one, d again the whole diff.
Type "d"
Sleep 1.5s
Type "2"
Sleep 1.5s
# d t switches to the folder tree; the chooser stays open, numbered anew.
Type "d"
Sleep 1s
Type "t"
Sleep 2.5s
Type "5"
Sleep 1.5s
Type "d"
Sleep 1s
Type "d"
Sleep 2s
Hide
Type "q"
