# The settings page: every setting, its value and where it comes from;
# enter toggles a switch or edits a value, saved to the config file at once.
# Render with `make demo`.
Output docs/demo/settings.gif
Source docs/demo/setup.tape

Hide
Type "herdr-deck --fake" Enter
Sleep 1s
Show
Sleep 1.2s
Type "s"
Sleep 2s
# A switch flips on enter: linear.status.
Down 4
Sleep 0.8s
Enter
Sleep 1.5s
# A value is edited in place: linear.workspace.
Up
Sleep 0.8s
Enter
Sleep 0.6s
Backspace 4
Type "globex"
Sleep 0.6s
Enter
Sleep 2s
Escape
Sleep 1.2s
Hide
Type "q"
