# The Files tab and the diff preview: d focuses the Files tab, v shows the
# file under the cursor's diff where the list is, j picks another file and
# J scrolls; v brings the list back. t switches to the folder tree.
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
# v previews the file under the cursor: an untracked Markdown file.
Type "v"
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
Hide
Type "q"
