# The list: Needs you pinned on top, the drawer following the selection,
# and the Backlog folding open and shut. Render with `make demo`.
Output docs/demo/hero.gif
Source docs/demo/setup.tape

Hide
Type "herdr-deck --fake" Enter
Sleep 1s
Show
Sleep 2s
# The inbox item, then the In progress tasks.
Down
Sleep 1s
Down 2
Sleep 1.2s
Down
Sleep 1.5s
# Past On hold to the folded Backlog: open it, look in, shut it.
Down 3
Sleep 0.8s
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
