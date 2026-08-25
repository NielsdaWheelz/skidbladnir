# P0 Codex App Server boundary

Recorded: 2026-08-25T03:55:06Z

Result: **PASS**

## Artifacts

- Codex: `0.149.1` / `rust-v0.149.1`
- Source commit: `ff29a44391deccde0aba0f8390337d7f3c319ea4`
- Binary SHA-256: `73dc5888888f411c1f0fa7b81d866e721dcc86b527ce8e3b2cf4708661e823ba`
- Generated schema files: `408`
- Schema-tree SHA-256: `583d4d37ae75d97964dd110fefd891f8a6fe7df0b27577a27b0bc29a33611e13`
- v2 schema-bundle SHA-256: `6f76cce25156d405f1da54f205751e38f7b9eb42246ac0742b9958dd60275350`
- Host platform: Linux, tmux `3.4`

## Procedure

`./scripts/test upgrade-live`

The gate regenerated the `--experimental` schema tree from the digest-verified
binary and compared every generated path and byte with the committed tree. It
then ran the direct Unix-WebSocket App Server probe independently against the
`personal`, `work`, and `work2` profile homes with strict config.

## Observed result

- Every connection initialized as `skidbladnir/0.149.1` on Linux/Unix and
  reported the exact selected profile home.
- Each profile completed an empty root `thread/start`; the first
  `thread/unsubscribe` returned exactly `unsubscribed`.
- Exact-id `thread/read(includeTurns=false)` returned the allocated thread and
  session identities, cwd `/home/niels/src`, creation time, root parentage, and
  status `idle`.
- The bounded `thread/list` request used source kinds `cli|vscode`. The fresh
  promptless thread remained absent, matching the pin's materialization
  behavior.
- A stock TUI resumed the exact thread through the same Unix listener in a
  uniquely named test-owned tmux session. The observed pinned process, thread
  argument, pane TTY, and cwd all matched.
- Killing only that test-owned tmux session removed the verified TUI process.
  The immediate exact-id read returned `idle`, consistent with the source's
  30-minute no-subscriber unload delay.
- The live test completed in `64.286s`. Test-owned App Server processes,
  sockets, and tmux sessions were absent after cleanup.

No conversation input was sent or captured. This record proves only the
broker-to-App-Server pin/transport/identity boundary. Gateway routing, hook
projection, shared-tmux handoff, Tailscale, terminal transport, and Android
remain `NOT_RUN` in their own proof rows.
