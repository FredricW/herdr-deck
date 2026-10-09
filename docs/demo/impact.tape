# The Impact tab: Look here first and the canvas of nested boxes, z for the
# full height; tab focuses it, j moves through the findings (each selects
# its box, whose imports are drawn as lines), then from box to box with
# the arrows; enter shows a finding's line in the diff preview. Then the
# same view on its own, as O opens it in a herdr pane. Render with
# `make demo`.
Output docs/demo/impact.gif
Source docs/demo/setup.tape

Hide
Type "herdr-deck --fake" Enter
Sleep 1s
Show
Sleep 1.2s
# ] three times: Files, Commits, then Impact.
Type "]]]"
Sleep 2.4s
# z: the full height, with the canvas under the findings.
Type "z"
Sleep 2.4s
# tab: the cursor on the first finding selects its box.
Tab
Sleep 2.4s
Type "j"
Sleep 1.6s
Type "j"
Sleep 2s
# Down into the canvas, then from box to box.
Type@500ms "jjjjjjj"
Sleep 1.6s
Right
Sleep 2s
Left
Sleep 1.4s
Down
Sleep 2s
Up
Sleep 1.2s
# Back on the third finding: enter shows its line in the diff preview.
Type@300ms "kkkkkk"
Sleep 1.4s
Enter
Sleep 2.6s
Escape
Sleep 0.6s
Hide
Type "q"
# The same view on its own, as O opens it in a herdr pane.
Type "clear; herdr-deck arch --fake" Enter
Sleep 1s
Show
Sleep 2.4s
Type@500ms "jjjjjjjjj"
Sleep 1.6s
Right
Sleep 2.4s
Hide
Type "q"
