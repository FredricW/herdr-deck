# Shared settings for every tape: sourced first, so each GIF looks alike.
Set Shell "bash"
Set FontSize 16
Set Width 1000
Set Height 740
Set Padding 16
Set Theme "Catppuccin Mocha"
Set TypingSpeed 60ms
Set CursorBlink false

# The demo environment, set up by `make demo`: the demo build and a no-op
# `open` first on PATH, a scratch home folder whose config file is a copy of
# the demo config, and no herdr, herdr-deck, Linear or XDG variables from the
# shell that started it. The settings page shows the home folder's paths.
Hide
Type `unset $(compgen -v | grep -E '^(HERDR|LINEAR|XDG)'); export PATH="$DEMO_DIR:$PWD/docs/demo/shim:$PATH" HOME="$DEMO_HOME" PS1='$ '; clear`
Enter
Show
