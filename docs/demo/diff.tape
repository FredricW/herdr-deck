# The Files tab and the diff preview: d focuses the Files tab, enter shows the
# file under the cursor's diff where the list is, j picks another file and
# J scrolls; v brings the list back. t switches to the folder tree. ]
# shows Commits: 1 previews the newest commit, j the next, v ends it; l
# expands a commit to its files, enter previews one, h collapses it.
# Render with `make demo`.
Output docs/demo/diff.gif
Source docs/demo/setup.tape

Hide
Type "herdr-deck --fake" Enter
Sleep 1s
Show
Sleep 1.2s
# d focuses the Files tab: a diffstat of every changed file.
Type "d"
Sleep 1.8s
# enter previews the file under the cursor: an untracked Markdown file.
Enter
Sleep 2.2s
# j moves to the next files; the preview follows.
Type "jjj"
Sleep 2.2s
# J scrolls the preview.
Type@250ms "JJJJ"
Sleep 2s
# v again: the list is back as it was.
Type "v"
Sleep 1.5s
# t switches to the folder tree.
Type "t"
Sleep 1.8s
Type "]"
Sleep 1.8s
Type "1"
Sleep 2.2s
Type "j"
Sleep 2.2s
Type "v"
Sleep 1.5s
# l expands the newest commit to its files; enter previews one of them
# in that commit; h collapses it again.
Type "k"
Sleep 0.8s
Type "l"
Sleep 1.8s
Type "j"
Sleep 1s
Enter
Sleep 2.2s
Type "v"
Sleep 1s
Type "h"
Sleep 1.5s
Hide
Type "q"
