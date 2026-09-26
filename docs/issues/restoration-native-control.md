# native control live qualification

problem: the restored pinned helper has source and no-auth subprocess qualification on macbook, but no installed three-host or authenticated-provider proof for this restoration.

impact: claude native status, saved-message reading, and background stop may be unavailable or differ across installed provider versions. terminal fallback must report its actual coverage and methods; an unexecuted live gate is `NOT_RUN`.

evidence: `llm-calling@ec97adeb9ddd0f91b141f89cc42cff7cc7efdb8f`, uv 0.11.28, python 3.12.13, and frozen `claude-agent-sdk==0.2.130` synced in a disposable macbook checkout. content-free inspection probes proved exact native executable and private `CLAUDE_CONFIG_DIR` dispatch through the skid launcher/shim, including missing-native and missing-shim failures without shared-command fallback. claude 2.1.282 exposed `agents --json --all` and `stop <id>`, and an empty private home returned an empty agents list. no approved live session was used.

resolved when: the installed skid-owned helper, absolute native provider and selected existing account are verified on macbook, devbox, and arch; approved live claude sessions on each host show correct native inspection, bounded history and matched stop behavior; helper failures retain honest terminal fallback; no unrelated provider process is controlled and no herdr control service is used. account history is intentionally shared. record content-free results and remove this file.
