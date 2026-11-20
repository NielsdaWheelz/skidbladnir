# Media kit

Reproduces the site's demo media in `site/assets`:

- `continuity.mp4`: one terminal shared between the desktop session browser and the Android phone, side by side, and its poster `continuity.png`;
- the stills `desktop-browser.png`, `phone-dashboard.png`, `phone-terminal.png` and `phone-forge.png`;
- the dwarf seals and `social-card.png`.

The sessions are staged. Stub `codex` and `claude` commands (`stub/`) paint authored frames from `internal/agentcontrol/testdata/frames` inside fake repositories, so no real terminal content or account appears.

## Prerequisites

- The owner's Mac and fleet, with the installed `skid`. `kit.sh` names the hosts and how to reach them.
- Go, Python 3, git, vhs, scrcpy and ffmpeg.
- The Android SDK's adb at `$ANDROID_HOME/platform-tools/adb` (default `~/Library/Android/sdk`).
- Google Chrome, for the card, and uv, for the seals.
- The phone, connected over adb: the only device, or named by `MEDIA_PHONE`; unlocked, with skid open on its dashboard and a screen timeout of at least a minute.

## Approval

`stage`, `record` and `cleanup` act on live state. They create sessions on the owner's fleet, write `~/demo` on every host and drive the owner's phone, so every run of each needs the owner's approval. `compose`, `seals.py` and `card/make.py` only read and write files.

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

Everything goes to `$MEDIA_WORK`, by default `~/.cache/skid-media`, outside the repository. The finished assets land in `$MEDIA_WORK/assets`. Review them, then copy them into `site/assets` by hand.

## Limits

Each still must stay under 200 KB and the site video under 4 MB. `compose` and `card/make.py` print the sizes and fail when one is over.
