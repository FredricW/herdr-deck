# The list: Needs you pinned on top, the drawer's card and tabs following
# the selection, and the Backlog folding open and shut. Render with `make demo`.
Output docs/demo/hero.gif
Source docs/demo/setup.tape

Hide
Type "herdr-deck --fake" Enter
Sleep 1s
Show
Sleep 2s
# Users page needs you: the card, then Overview's Next and links. ] shows
# its Files, its Commits, then its Log.
Type "]"
Sleep 1.8s
Type "]"
Sleep 2s
Type "]"
Sleep 2s
# The tab stays as the cursor moves: Templates page's Log.
Down 2
Sleep 1.8s
# Back to Overview; the review row shows its PR, links and dev servers.
Type "]"
Sleep 1s
Down
Sleep 2.2s
Down 3
Sleep 1s
Space
Sleep 1.2s
Down 2
Sleep 1.2s
Up 2
Sleep 0.5s
Space
Sleep 1s
Hide
Type "q"
