# The project picker: the list's last line says another project needs
# you, p lists every project with its waiting threads, typing filters, tab
# adds the archived ones. Render with `make demo`.
Output docs/demo/projects.gif
Source docs/demo/setup.tape

Hide
Type "herdr-deck --fake" Enter
Sleep 1s
Show
Sleep 2.5s
Type "p"
Sleep 2.5s
Down
Sleep 1s
Down
Sleep 1s
Down
Sleep 1.5s
Tab
Sleep 2s
Tab
Type "doc"
Sleep 2s
Escape
Sleep 1s
Escape
Sleep 1.5s
Hide
Type "q"
