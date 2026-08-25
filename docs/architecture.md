# Skíðblaðnir v0: product and architecture

Status: reviewed implementation target after the 2026-08-25 scope reset.

This document supersedes the audited-orchestration architecture (git history
through `6f2d697`). That design was internally consistent and is preserved in
history, but it built an audited Codex runtime system where the product is a
specialized Android remote for tmux. Its platform evidence — tmux 3.4 grouped
sessions, the pane-steal hazard, the Android terminal harness, profile
isolation, TUI key behavior — remains valid and is carried forward. Its
contract, hook-trust, provenance, and lifecycle machinery is retired.

Specification precedence: this document owns product behavior, architecture,
scope, and acceptance; [`roadmap.md`](roadmap.md) owns delivery order;
[`docs/rules`](rules/index.md) applies where it does not conflict with the
v0 scope. A platform fact that contradicts a premise reopens this document.

## 1. Philosophy

- **tmux is the database and the process supervisor.** Session list, pane
  facts, and user options are the only durable state. Gateway restart means
  "list tmux again", never a recovery protocol.
- **The agent is an opaque terminal program.** Codex handles `/quit`, `/new`,
  `resume`, approvals, hooks, git, and its own configuration exactly as it
  normally does. Skíðblaðnir never parses, tracks, or reconstructs its
  conversations. Any future CLI (Claude Code, OpenCode, Kimi) is just another
  fixed launch command.
- **Android and the laptop are two tmux clients.** One process, one screen,
  one draft; either device attaches and detaches freely.

From a Galaxy S22+, Niels can: see every tmux session on the devbox with an
honest status chip and attention badge; create a session in an explicit
directory running one allowlisted profile command; attach the same stock TUI
the laptop sees; type, paste, and dictate through Gboard; detach without
stopping anything; and kill an exact confirmed session.

## 2. Fixed contract

| Concern | Decision |
| --- | --- |
| Product | Skíðblaðnir; ASCII namespace `skidbladnir`; private, one user, one phone, one devbox |
| Phone | Galaxy S22+ `SM-S906W`, Android 16/API 36 |
| Host | One Hetzner VPS; Linux, mosh, systemd user service, proven tmux 3.4 |
| Network | Tailscale Serve TLS to a loopback gateway; Funnel/public ingress forbidden |
| Auth | One devbox-minted bearer, entered once in the app; every `/v1` request requires it |
| Profiles | Closed `personal \| work \| work2` command allowlist; callers never supply `CODEX_HOME` or raw commands |
| Runtime | Opaque terminal programs in ordinary tmux sessions; no hooks trust, provenance, thread tracking, or pin enforcement |
| State | tmux sessions/panes/user options only; no SQLite, facts ledger, or command receipts |
| Handoff | Grouped shadow tmux clients; laptop and phone attach concurrently |
| Client | Kotlin/Compose dashboard; vendored pinned xterm.js terminal |
| Host app | Go, tmux/PTY, `/proc` sampling; standard library HTTP |
| Trust | The agent is trusted as the devbox user; no hostile same-UID containment claim |

Profile mapping is a closed gateway-config table, initially:

| Profile | Command | Environment |
| --- | --- | --- |
| `personal` | `codex-personal` | `CODEX_HOME=/home/niels/.codex-personal` |
| `work` | `codex-work` | `CODEX_HOME=/home/niels/.codex-work` |
| `work2` | `codex-work2` | `CODEX_HOME=/home/niels/.codex-work2` |

Adding a provider is adding one row (`claude`, `opencode`, …) — a config
change, not a design event; the app renders whatever rows the gateway
declares. The gateway execs the row's command with its flags in the requested
cwd. It does not verify binaries, hooks, or configuration; the agent sees
exactly what a laptop launch would see. There is no router interception:
laptop launches are plain shell commands, and their sessions simply appear in
the inventory.

### Product language

Skíðblaðnir (app), Hlíðskjálf (grid motif; literal label `Agents`), The Forge
(new-session sheet; literal `New agent`), and Dvergatal (the append-only dwarf
catalogue at `catalog/characters.json`, ≥100 entries, original portraits)
are retained. Gateway-created sessions are named `ga-<dwarf>` from a free
catalogue key; the key is stored in a tmux user option and selects the card
portrait. Domain, error, and destructive-action language stays literal.

### Guarantees (cheap, disaster-preventing)

- Never kill a tmux session other than the exact confirmed target.
- Detach and kill are visibly different actions; kill always confirms.
- Validate cwd; allowlist the three profile commands; nothing else launches.
- Credentials stay on the devbox; the app holds only its bearer.
- App or gateway restart re-lists tmux; it never replays input or guesses.

### Non-goals

Thread/conversation identity, exact semantic state, hook trust preflight,
`/proc` ancestry authentication, git/project-root resolution, router
interception, SQLite lifecycle facts, durable command receipts and replay,
adoption, pin-parity launch refusal, upgrade rehearsals, proof-ledger
acceptance matrices, App Server integration, project enrollment, quotas,
scheduling, orchestration, and multi-user anything. See §8 for what would
ever bring the retired machinery back.

## 3. Platform evidence carried forward

Recorded on exact versions (tmux 3.4, codex-cli 0.149.1, physical SM-S906W)
and still binding on v0:

- A stock TUI uses the normal terminal buffer and is shareable by tmux
  clients. Grouped sessions share panes/processes while clients keep
  independent current-window context; `active-pane` and `ignore-size` are
  required for phone attachment.
- **Pane-steal hazard:** every pane-level targeting form (`switch-client` with
  a pane target, pane-targeted attach, `select-pane`) mutates the window's
  shared active pane and drags an unflagged laptop client with it. Targeting
  must be session/window-level; a client's own active pane is selected only by
  a key sequence written into that client's PTY.
- Killing the last grouped session kills the shared panes: Detach must run a
  last-link guard and never destroys the source session.
- Stock TUI input: raw Ctrl-J (`0x0a`) is newline-without-submit; raw CR
  (`0x0d`) submits; these bytes survive the tmux 3.4 client path.
- S22+ WebView/xterm.js/Gboard: ANSI, Unicode, IME composition, editable
  dictation, clipboard, automatic DA/DSR/CPR replies, resize, rotation.
- SIGKILL of a TUI emits nothing; only tmux/process facts are reliable
  liveness evidence. (This is why v0 trusts tmux, not agent self-reporting,
  for aliveness.)

The retired hook/identity findings (rollout basenames, fork identity, hook
ordering) remain true and recorded in git history; v0 simply does not consume
them.

## 4. Product behavior

### Dashboard

`GET /sessions` inventory, polled from `tmux list-sessions`/`list-panes` plus
`/proc`, drives a dense card grid:

- **Card facts:** session name, dwarf portrait when `@skid_character` is set,
  profile (`@skid_profile` or unknown), objective (`@skid_objective`,
  optional), pane cwd, active command, attached-client count, status chip
  with signal age, attention badge.
- **Status chips** are heuristic and labeled with their age, never presented
  as exact semantic truth:
  - `ATTENTION` — `@skid_attention` set or window bell flag raised;
  - `WORKING` — an allowlisted agent command is the pane's foreground process
    and output occurred within the named silence threshold;
  - `IDLE` — agent alive, silence past the threshold;
  - `SHELL` — no agent process in the pane (exited to shell);
  - `UNKNOWN` — poll failure.
- Laptop-created sessions appear with whatever facts tmux exposes; absence of
  Skíðblaðnir metadata is displayed, not guessed.
- Grid order: attention first, then `WORKING`, `IDLE`, `SHELL`, name.

### Attention

The one config touch per profile: `notify` in each `config.toml` names a
five-line `skid-notify` script. Codex invokes it on turn completion; it reads
its inherited `$TMUX_PANE` and sets `@skid_attention` (and rings the bell).
Trust-by-default — no provenance, no receiver, no schema. Opening the card's
terminal clears the flag (bell clears natively on view; the gateway unsets the
user option on attach). Installation is idempotent and shown to the user.

### Start (The Forge)

`POST /sessions {cwd, profile, optionalName}`:

1. Cwd: ≤4,096 UTF-8 bytes, no NUL/C0/C1; optional leading `~`/`~/` expands
   against the service UID home; must be an existing directory. Failure is
   typed and mutates nothing.
2. Profile must be one of the three allowlisted commands.
3. The gateway creates a tmux session (named `optionalName` or `ga-<dwarf>`
   from a free catalogue key), sets `@skid_profile`/`@skid_objective`/
   `@skid_character`, and runs the profile command with YOLO in the cwd. No
   prompt is sent; Codex's own onboarding/trust flows appear in the terminal
   like any laptop launch.

### Attach and handoff

- Opening a card starts no second process and never detaches the laptop.
- The gateway creates an ephemeral session grouped with the target, attaches
  one gateway-owned phone PTY as a client with `active-pane` and
  `ignore-size`, and targets **session/window-level only**; the phone's active
  pane is selected by writing a `select-pane` key sequence into the phone PTY
  (per the pane-steal finding). Never mutate another client's selection.
- With an unflagged laptop client present, phone resize changes only its own
  viewport (`CONSTRAINED`); alone, the phone owns sizing (`OWNER`).
- Both devices share process, screen, draft, and turn.

### Detach

Detach closes only the phone client and its shadow through a last-link guard;
the source session and its process are never destroyed by Detach. Phone loss,
app backgrounding, and process recreation destroy only the attachment; the
next open attaches fresh with no byte replay.

### Kill

`DELETE /sessions/:id` requires the client to send both the tmux session id
and the displayed name; the gateway re-verifies the pair immediately before
`kill-session` and refuses on any mismatch. The app confirms destructively
("Kill session ga-durinn? Codex and its work end now.") and never offers kill
and detach in the same gesture. There is no working/idle gate — the human is
looking at the terminal facts; the guarantee is exactness of target, not
semantic safety.

### Pressure

`GET /pressure`: five-second `/proc` sampling of CPU, normalized load, memory,
swap, disk, PSI; WARM/HOT thresholds as previously proven (memory 15%/8%,
disk 15%/5%, load 1.0/2.0, CPU PSI some avg60 20%/50%, memory/I-O PSI full
avg60 1%/5%); escalate immediately, de-escalate after 60s. Missing input shows
UNKNOWN. Pressure never blocks Start.

## 5. Host architecture

```text
Galaxy S22+                       laptop / mosh
  Compose + xterm.js                    |
     | HTTPS/WSS (Tailscale Serve)      v
     v                            ordinary tmux client
  Go gateway ---- tmux commands ----> tmux server (the database)
     |                                  |
     `-- /proc sampling                 `-- panes running codex-* / anything
```

- One systemd user service runs the gateway. Bind loopback; Tailscale Serve
  exposes `/v1` only; `/healthz` stays loopback.
- No SQLite. Session metadata lives in tmux user options (`@skid_profile`,
  `@skid_objective`, `@skid_character`, `@skid_attention`). Poller state is
  in-memory and rebuilt on start.
- The API is:

| Method/path | Contract |
| --- | --- |
| `GET /v1/sessions` | Inventory with card facts above |
| `POST /v1/sessions` | `{cwd, profile, optionalName}`; typed failures |
| `GET /v1/sessions/{id}/terminal` | WSS upgrade after fresh session-exists check |
| `DELETE /v1/sessions/{id}` | Exact id+name confirmed kill |
| `GET /v1/pressure` | Current sample plus recent window |

- WSS: text frames `Hello | Presence | Resize | Detach | Error`; binary
  frames are PTY bytes both ways. One WSS owns one PTY/client/shadow and
  tears all three down on any close, subject to the last-link guard. No byte
  replay or gateway scrollback; slow clients disconnect and reattach fresh.
  Any WSS loss freezes terminal input behind a typed `Reconnect required`.
- Bounds: HTTP body 64 KiB; cwd 4,096 bytes; objective 240 scalars; terminal
  frame 64 KiB; queue 1 MiB; geometry 20–240 × 5–120. Named, not schema-frozen.
- Hand-written DTOs; no generated clients, contract digests, or lock files.

## 6. Android surface

- Compile/target/min SDK 36; one manually installed package.
- Grid, Forge, and terminal per §4. Forge preserves invalid drafts; cwd field
  disables autocorrect/smart punctuation.
- Terminal: the proven harness. Vendored pinned xterm.js in a locked WebView
  (`WebViewAssetLoader`, CSP `default-src 'none'` + bundle, no JS bridge, no
  network/file access; Kotlin owns WSS/auth; bearer never enters WebView).
  Accessory row `Agents | Esc | Ctrl-C | Tab | arrows | Home | End | Newline |
  Detach`; `Newline` sends raw `0x0a`, Enter sends `0x0d`. Paste strips ESC
  and C0 except newline/tab before bracketed paste. Gboard owns typing,
  clipboard, and dictation; dictation stays editable and never auto-sends.
- Rotation, IME resize, and process recreation preserve or cleanly recreate
  the attachment; nothing replays.
- Near-black tonal surfaces, dwarf portraits as landmarks, semantic labels on
  all controls; accessibility beyond labels is best-effort.

## 7. Security

- Tailnet-only ingress; TLS via Tailscale Serve; no Funnel, cookies,
  redirects, or CORS.
- One 256-bit bearer minted by a devbox CLI command and entered once in the
  app; constant-time comparison; re-minting revokes the old token and closes
  live streams.
- The terminal endpoint is shell-equivalent authority: it accepts only a
  server-resolved session id and closes on mismatch. Android never supplies
  raw tmux targets, commands, or homes.
- Logs carry names, timings, and typed errors — never terminal bytes,
  objectives, prompts, tokens, or credentials.
- YOLO agents share the devbox UID; containment requires a separate UID/VM
  and is explicitly out of scope.

## 8. Upgrade ladder (deliberately not in v0)

- **v0.5 — trust-by-default hooks (~50 lines):** Stop/UserPromptSubmit hooks
  that `tmux set-option -p -t "$TMUX_PANE" @skid_status ...`. Status keyed by
  pane, not thread: rollover, fork, and resume identity stay Codex's problem.
- **v0.5 — push:** FCM (or ntfy/UnifiedPush) delivery of the Tier-1 attention
  signal: redacted data-only message, deep link, dedupe bit in a tmux user
  option, and a poller sweep that sends late rather than never. Push-to-glance
  needs nothing more.
- **Retired until push-to-act:** authenticated provenance, exactly-once
  attention, durable receipts, thread identity, and replay return only if
  notifications grow buttons that act unattended (resurrection, approval,
  chaining), or unattended orchestration arrives. That decision reopens this
  document; nothing here scaffolds for it.

## 9. Verification

Right-sized: deterministic unit tests; one real-tmux integration suite
(create/list/attach/geometry/last-link/kill-exactness on an isolated `-L`
socket); one physical S22+ pass (pair, grid, Forge with spaces-and-`~` cwd,
concurrent laptop/phone attach with unchanged laptop view, IME/dictation,
rotation, reconnect, detach-vs-kill); `scripts/test` keeps `static`, `unit`,
`integration`, `platform`, `live` with that reduced meaning. The retired
proof-ledger/acceptance matrix does not return. Existing `evidence/live/`
records remain as historical platform evidence.

Acceptance is one sentence per guarantee in §2 plus: laptop and phone share
one pane/PID/draft with laptop geometry and focus unchanged; a killed session
is exactly the confirmed one; a detached phone leaves Codex running; every
status chip states its signal and age; restart of app or gateway converges to
`tmux list-sessions` truth.
