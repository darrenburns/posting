# The README demo. Run `make readme-demo`, which sets up DEMO_HOME and runs
# this with VHS (https://github.com/charmbracelet/vhs).
#
# Jump labels depend on what's on screen, so check them if the layout changes.

Output docs/assets/readme.gif

Set Shell bash
Set FontFamily "BlexMono Nerd Font Mono"
Set FontSize 16
Set Width 1400
Set Height 760
Set Padding 0
Set TypingSpeed 70ms

Hide
Type "export PATH=$DEMO_HOME/bin:$PATH XDG_CONFIG_HOME=$DEMO_HOME/config XDG_DATA_HOME=$DEMO_HOME/data; python3 demo/server.py & clear" Enter
Sleep 1s
Type "posting -c demo/sample-api -e local" Enter
Sleep 3s
Show
Sleep 1.5s

# Jump to List users, open it and send it.
Ctrl+O
Sleep 1s
Type "c"
Sleep 1.2s
Enter
Sleep 800ms
Ctrl+J
Sleep 2.5s

# Open Create user in a second tab.
Ctrl+O
Sleep 1s
Type "gf"
Sleep 600ms
Enter
Sleep 1.2s

# Jump to the new header row and add a header.
Ctrl+O
Sleep 1s
Type "c"
Sleep 400ms
Type "X-Request-Id"
Tab
Type "readme-demo"
Sleep 800ms

# Send it, then look at the response headers and the trace.
Ctrl+J
Sleep 2.5s
Ctrl+O
Sleep 1s
Type "s"
Sleep 2.5s
Ctrl+O
Sleep 1s
Type "f"
Sleep 3.5s
