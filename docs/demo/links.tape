# The drawer's numbered links: a digit opens one, a letter opens that kind
# (a chooser when there are several). The demo's `open` is a no-op, so no
# browser starts. Render with `make demo`.
Output docs/demo/links.gif
Source docs/demo/setup.tape

Hide
Type "herdr-deck --fake" Enter
Sleep 1s
Down 3
Sleep 0.5s
Show
Sleep 2s
# Three Figma links: f asks which one.
Type "f"
Sleep 1.5s
Type "3"
Sleep 1.8s
# One PR and one running dev server open straight away.
Type "g"
Sleep 1.8s
Type "o"
Sleep 1.8s
# A digit opens that link.
Type "1"
Sleep 2s
Hide
Type "q"
