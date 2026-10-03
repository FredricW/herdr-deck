# What's new: the footer's one-time "Updated to" note after an update, and
# w showing the changelog. The scratch home says the deck last ran v1.2.2.
# Render with `make demo`.
Output docs/demo/news.gif
Source docs/demo/setup.tape

Hide
Type "mkdir -p ~/.local/state/herdr-deck && echo 1.2.2 > ~/.local/state/herdr-deck/last-version && clear" Enter
Type "herdr-deck --fake" Enter
Sleep 1s
Show
Sleep 2.5s
Type "w"
Sleep 2.5s
# z gives the changelog the whole pane.
Type "z"
Sleep 3s
Escape
Sleep 1.2s
Hide
Type "q"
