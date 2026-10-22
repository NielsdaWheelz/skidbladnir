# a touch re-shows the IME and drops a released selection's highlight

problem: once xterm's helper textarea holds focus and Back has hidden Gboard,
any touch on the terminal re-shows the IME (WebView behaviour, with or without
terminal input). after a long press, the resulting grid refit clears xterm's
highlight while the page stays `Selected` and the floating `Copy` toolbar stays
up. under tmux, a mouse-mode re-send also drops a released highlight.

impact: the person sees a `Copy` toolbar with nothing highlighted. Copy still
writes the release snapshot, so no wrong text reaches the clipboard, but the
screen no longer shows what will be copied.

evidence: the terminal-page emulator differential (api 35, Gboard, 2026-10-22)
recorded it identically on the base (`c4399a3`) and terminal-page builds:
textarea focused, IME hidden by Back, long press on a word gives `[selected,
IME shown, toolbars 1, highlight false]`.

reproduce: tap the terminal (Gboard opens), press Back once (Gboard closes),
long-press a word and release.

resolved when: in that sequence the highlight stays visible while `Copy` is
offered, or the toolbar closes with it; and a tmux mode re-send after release
leaves the highlight in place.
