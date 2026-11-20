# Media kit

Reproduces the site's demo media in `site/assets`:

- `continuity.mp4`: one terminal shared between the desktop session browser and the Android phone, side by side, and its poster `continuity.png`;
- the stills `desktop-browser.png`, `phone-dashboard.png`, `phone-terminal.png` and `phone-forge.png`;
- the dwarf seals and `social-card.png`.

The sessions are staged. Stub `codex` and `claude` commands (`stub/`) paint authored frames from `internal/agentcontrol/testdata/frames` inside fake repositories, and the terminal both devices type into, api, runs a bare shell with a fixed prompt and no rc files, history or login prompt, so no real terminal content or account appears. stage types one setup line into each session's login shell, and the owner's shell history keeps it: on the macbook, whose zsh sets `inc_append_history` and `share_history`, the lines for api, routes and dashboard reach `~/.zsh_history` at once.

## Prerequisites

- The owner's Mac and fleet, with the installed `skid`. `kit.sh` names the hosts and how to reach them.
- Bash 5, Go, Python 3, git, vhs, scrcpy and ffmpeg.
- The Android SDK's adb at `$ANDROID_HOME/platform-tools/adb` (default `~/Library/Android/sdk`).
- Google Chrome, for the card, and uv, for the seals.
- The phone, connected over adb: the only device, or named by `MEDIA_PHONE`; unlocked, with skid open on its dashboard.

## Preconditions

- No owner group sorts before `cobalt`, comparing with ASCII case folded. desktop.tape steps from all to the first group, which must be atlas, and the group bars in the shots must open with the three demo groups. stage refuses otherwise, and also when a demo group, or a demo session's name on its host, is already in use.
- Each host's login shell is zsh or bash: stage types shell lines into it.
- On the phone, Do Not Disturb is off, and the screen timeout is at least a minute. record refuses to start with Do Not Disturb on, turns it on for the run and off again after.
- No other session drives the phone during record (see Approval).

## Approval

`stage`, `record` and `cleanup` act on live state. They create sessions on the owner's fleet, write `~/demo` on every host and drive the owner's phone, so every run of each needs the owner's approval. Approving `record` includes confirming that no other session will drive the phone while it runs: record checks just before and after each input that skid has focus, and stops when it does not, but it cannot see input another session makes between its checks. `compose`, `seals.py` and `card/make.py` only read and write files.

## Order

1. `scripts/media/stage`
2. `scripts/media/record`
3. `scripts/media/compose`
4. `scripts/media/cleanup`, also after a failed stage or record.

The seals and the card stand alone:

```sh
uv run --no-project --with fonttools python scripts/media/seals.py \
  germanic-operatic.alberich norse.fili norse.modsognir norse.nori norse.thekkr tolkien.gimli
python3 scripts/media/card/make.py
```

## Output

The work goes to `$MEDIA_WORK`, by default `~/.cache/skid-media`, outside the repository, and the finished assets to `$MEDIA_WORK/assets`. Review them, then copy them into `site/assets` by hand. Keep `$MEDIA_WORK` private: when a wait in desktop.tape times out, `vhs.log` holds the screen, which can be the browser's hidden all view with the owner's session names.

The kit also writes outside it:

- `~/demo` on every host, from stage until cleanup;
- `~/Library/Fonts/skid-media-jetbrains-mono.ttf` on the Mac while record runs;
- `/data/local/tmp/skid-media-ui.xml` on the phone, the screen dump each tap reads, while record runs.

## When record is killed

On exit, including on a signal it can catch, record stops vhs and scrcpy, removes the font and the dump, and restores the phone's Do Not Disturb and demo mode. It reads both settings back and, if they differ from the originals, says what to restore by hand and exits nonzero, as it does when any restore step fails.

SIGKILL skips all of that. Then restore by hand, with the values record saved in `$MEDIA_WORK/phone-settings.txt` before it changed anything:

```sh
adb -s SERIAL shell cmd notification set_dnd off
adb -s SERIAL shell settings put global sysui_demo_allowed VALUE  # or, when VALUE is null: settings delete global sysui_demo_allowed
adb -s SERIAL shell am broadcast -a com.android.systemui.demo -e command exit
adb -s SERIAL shell rm -f /data/local/tmp/skid-media-ui.xml
```

Stop the `vhs` and `scrcpy` it started; cleanup removes the font.

## Limits

Each still must stay under 200 KB and the site video under 4 MB. `compose` and `card/make.py` print the sizes and fail when one is over.
