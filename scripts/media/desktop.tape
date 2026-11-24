# The desktop half of the continuity shot, in the installed browser. record
# runs it from $MEDIA_WORK after writing select-api.tape there. stage gave api a
# bare shell whose prompt is "atlas $ "; each attach waits for it before typing,
# so no keys reach the browser.
Output desktop.mp4
Require skid
Set Shell zsh
Set FontFamily "JetBrains Mono"
Set FontSize 22
Set Width 1380
Set Height 780
Set Padding 24
Set Framerate 30
Set TypingSpeed 75ms
Set CursorBlink false
Set Theme { "name": "Nidavellir", "background": "#0c0d0f", "foreground": "#f3f0e8", "cursor": "#d6a85f", "selection": "#3a3833", "black": "#15171a", "red": "#d74e33", "green": "#4f925c", "yellow": "#ac7e35", "blue": "#538bac", "magenta": "#bb5897", "cyan": "#459c93", "white": "#aaa69d", "brightBlack": "#5c6370", "brightRed": "#e46c55", "brightGreen": "#76b082", "brightYellow": "#d6a85f", "brightBlue": "#78a9c6", "brightMagenta": "#cd70ab", "brightCyan": "#64c4ba", "brightWhite": "#f3f0e8" }

Hide
Type "skid"
Enter
# The browser opens on all; l steps to the first group, which stage made atlas.
Wait+Screen@30s /atlas/
Type "l"
Wait+Screen@30s /\breview\b/
Wait+Screen@30s /\bmigrate\b/
Wait+Screen@30s /\bapi\b/
Wait+Screen@30s /\broutes\b/
Wait+Screen@30s /\bschema\b/
# Up clamps at the first row; select-api.tape moves down to api's row.
Up 9
Source select-api.tape
Wait+Screen@5s /▌ [$]  api /
Show

Sleep 3s
Enter
Wait+Screen@10s /atlas \$/
Sleep 1500ms
Type "git log --oneline"
Sleep 300ms
Enter
Sleep 3s
Ctrl+]
Type "d"
# Meanwhile the phone opens api, runs ls and detaches; record checks it did.
Sleep 20s
Enter
Wait+Screen@10s /atlas \$/
Sleep 2s
Type "git status"
Sleep 300ms
Enter
Sleep 4s
