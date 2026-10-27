# Publication campaign

Working tracker for the publication campaign begun 2026-10-25: goal, decisions,
canonical copy, process, waves and progress. The coordinator updates it as each
pull request merges and deletes it when the campaign closes; git keeps it. The
[architecture](architecture.md) stays the system contract; this file only records
the campaign.

## Goal

Make Skíðblaðnir legible to a reader who will probably never install it, and
dependable for one who does. A reader can:

1. see it work without installing it;
2. understand how it is built and why, from the architecture down to single files;
3. judge the code, the design and the project's history from evidence;
4. install it on a fresh computer and an Android phone from the published docs alone.

Deliverables: a license; an install path verified on a fresh macOS VM, with Linux
package managers in containers; a front door (README, user docs, community files)
in correct capitalization; a reading path through the code and a history of its
decisions; demo media; a static site at skid.nielseriknandal.com; an essay on
nielseriknandal.com.

Non-goals: growth or conversion work, search optimization, support commitments,
product capabilities beyond the fixes below, iOS, an app store listing, telemetry,
site analytics, rewriting git history.

## Decisions

Owner, 2026-10-25.

| area | decision | accepted cost |
| --- | --- | --- |
| license | Apache-2.0 for the repository; bundled fonts and xterm.js keep their own licenses | a longer license than MIT |
| contribution | anyone may open issues; only the owner merges; pull requests limited to collaborators | outside fixes arrive as issues, not patches |
| voice | public prose (README, docs, AGENTS.md, site, essay, release notes) uses correct capitalization, with product names as their owners spell them: Tailscale, macOS, Android, Claude Code, Codex, tmux. CLI, installer and error output keep the lowercase command-line convention. skid is always lowercase, even at the start of a sentence; Skíðblaðnir is always capitalized | the lowercase house style ends in prose; issue files convert as they are touched |
| name | Skíðblaðnir is the proper name; skid is the everyday name and the command; pronounced SKEETH-blahth-neer | |
| authorship | public copy says nothing about the tools used to write the code and claims nothing either way; history, trailers and AGENTS.md stay as they are | |
| positioning | a different contract, never "superior"; alternatives are named graciously, their strengths first | |
| distribution | Android only; the APK ships on GitHub releases with an Obtainium badge; no store | Play Protect warnings; no store discovery |
| fleet trust | computers administer, phones are clients: hub administration refuses phone bearers | a phone cannot invite, remove or apply |
| tests | pure-core unit tests deferred; the proposal is recorded in [test system](issues/test-system-reset.md) | the first objection a reviewer raises stays open |
| docs | user docs cover procedures (install, phone). References keep their owners: `skid --help` for the CLI, [architecture §5](architecture.md#5-configuration) for configuration, [§3](architecture.md#3-topology-and-trust) for trust; the README links them. The site exhibits and links rather than copies | readers leave the site for depth |
| site | static HTML and CSS in `site/`, no framework or build step; a Vercel project rooted at `site/`; a DNS-only `skid` CNAME; dark only; no analytics; self-hosted subsets of Big Shoulders and JetBrains Mono; JavaScript only for copy buttons and tabs; under 100 KB before media | no light theme; no visitor data |
| launch | an essay on nielseriknandal.com leads; Show HN is optional and later | |
| history | no rewrite; the Firebase client key in history is handled in Google Cloud, not git | the tailnet name and one hostname email stay in history |
| Linux walk | no fresh Linux host; the fresh Linux install is `NOT_RUN`; Linux fixes are proven by temporary tests and package-manager containers | Linux first-install failures can ship unseen |
| test tailnet | test machines join the owner's tailnet as ephemeral nodes, removed after W4, each use approved; no throwaway tailnet; the HTTPS-off path is proven by a temporary test against a faked `tailscale status --json` | a stranger's tailnet defaults are never seen live |

### Proposed, for the owner before W2

| item | proposal | cost |
| --- | --- | --- |
| PATH | one unguarded line inside the existing managed shell block prepends `~/.local/bin` when it is absent; uninstall removes it with the block; dev-server's dotfiles keep the block | every shell, not only skid's, gains `~/.local/bin` |
| prerequisites | detect and instruct with the exact command; never change tailnet or host policy ([non-goals](architecture.md#non-goals)). `skid connect` checks the Tailscale operator, MagicDNS and HTTPS certificates; the installer checks Arch's `tailscaled` and Linux linger | the user runs one or two commands by hand |
| port 8443 | document that `skid connect` takes over HTTPS port 8443; no code change | an existing handler is still replaced |
| CLI reference | `skid --help` stays the one owner of the command reference and the automation guide: a short section for people first, the guide after it | the help stays long |
| fleet output | human text by default and a new `--json`; the fleet interfaces change; W2.2 first checks dev-server and jarvis for parsers | a contract change |
| `--version` | an alias of `skid version`, whose output the installer parses and which stays unchanged | |
| release integrity | `SHA256SUMS` also lists `install` and gains a detached signature by the release key; third-party notices ship in the APK and host archives | two more release assets |
| APK key | a new release key with a real distinguished name, generated and held by the owner; the phone reinstalls and re-pairs once | one re-pair |
| voice in the design language | [§4](design-language.md#4-voice) governs product copy; public prose may quote, with attribution, the primary sources its provenance ledger cites, and uses the canonical copy below. W6 adds that sentence to §4 | |

## Canonical copy

Used verbatim wherever it appears.

The line:

> Every tmux session on every machine you own, on your desktop and your Android
> phone. The same live terminals, straight over your tailnet. Nothing hosted, no
> skid account, nothing to migrate.

The epigraph, *Gylfaginning* 43 in Arthur Gilchrist Brodeur's 1916 translation,
which is in the public domain. Before first use, confirm the attribution and the
elided words against a second copy of Brodeur's text.

> It is so great that all the Æsir may man it, with their weapons and armaments,
> and it has a favoring wind as soon as the sail is hoisted, whithersoever it is
> bound; but … it may be folded together like a napkin and kept in one's pouch.

The three clauses, under the epigraph:

- **Room for all:** every session on every machine.
- **A fair wind wherever it's bound:** straight over your tailnet, with no relay of
  ours.
- **Folds into a pouch:** your phone.

Where the dwarves are introduced, once: "Dwarves built the ship, and in skid
dwarves crew it." The name, once per surface: "Skíðblaðnir (SKEETH-blahth-neer).
Call it skid."

Copy never overstates: Tailscale needs an account, and Tailscale may carry
traffic through its encrypted DERP relays when no direct path exists. skid itself
hosts nothing and has no account.

Positioning. Remote access to coding agents sorts on three questions: who carries
the bytes, who owns the process, and what the phone shows. skid answers your
tailnet, your tmux, and a terminal. A comparison concedes each alternative's
strength first, then states the one contract that differs:

- herdr, with droidtty on Android: the most capable agent-aware multiplexer, and a
  native Android client for it. skid keeps tmux as the owner of your sessions, so
  nothing migrates and sessions outlive skid.
- Claude Code Remote Control and Codex remote: free and first-party; traffic goes
  through the vendor; one provider each, in a conversation view.
- T3 Code and Happy: polished multi-provider phone apps, with their own server or
  relay and a conversation view.
- Termius or Moshi with tmux: no new software on the host; one host at a time.

When not to use skid: an iPhone; Windows; an Intel Mac or macOS before 26; Linux
on arm64; a team; no Tailscale; wanting a hosted relay or a chat interface. Do not
lead with provider independence, agent status, recovery, the cross-machine CLI or
the absence of telemetry; alternatives claim each. State them as facts further
down.

Maturity, stated plainly: a single-user prototype, built and run by one person
([calibration](architecture.md)).

## Process

- One wave at a time. Within a wave, slices on disjoint paths may run in parallel;
  they merge in their numbered order.
- One pull request per slice: worktree `~/Documents/code/skidbladnir-<slug>`,
  branch `publication/<slug>`, rebased on `origin/main` before review. Another
  session merges to main concurrently; check main before merging.
- When a slice changes something dev-server pins or parses (the `scripts/install`
  digest, release assets, CLI output), a paired worktree
  `~/Documents/code/dev-server-<slug>` carries dev-server's half, reviewed with it.
- The coordinator is the root integrator. It alone edits `docs/architecture.md`,
  `docs/roadmap.md`, `scripts/check`'s composition and the catalogue
  ([AGENTS.md](../AGENTS.md)), adding those edits to a slice's pull request before
  review. Each slice names the architecture sections it changes.
- An implementer's brief: Goal, Decisions, Canonical copy and Process from this
  file; its wave section; its slice's paths; AGENTS.md's guardrails verbatim. Opus
  implements design, code and copy; Sonnet takes mechanical passes; at most ten
  agents run at once.
- Every pull request gets an independent review by a fresh agent that receives the
  diff and the wave's "done when", never the implementer's summary. Findings are
  fixed or recorded as issues before merge.
- Code passes `scripts/check verify`. Behavior is proven with temporary tests at
  the affected boundary ([testing](rules.md#testing)). Anything that installs,
  enables services or reaches Tailscale runs in the W2.0 environments. The owner's
  tmux, the installed gateway, Tailscale, the phone, other hosts, real provider
  accounts, releases, GitHub settings, DNS, Vercel and Google Cloud need the
  owner's approval in the current turn. `NOT_RUN` is never a pass.
- The owner merges, or tells the coordinator to. Releases happen only on the
  owner's request, by the [release](architecture.md#release) procedure.
- The coordinator updates [progress](#progress) after each merge. Research notes
  are held privately by the owner, outside the repository.

## Waves

### W0 Plan

Goal: the campaign is written down before it starts. This file, and the deferred
pure-core test proposal in [test system](issues/test-system-reset.md).

Done when: the owner approves it and it merges.

### W1 Trust floor

Goal: anyone may lawfully use the code, and the tree exposes nothing avoidable.

- `license`:
  - `LICENSE` holds the verbatim Apache-2.0 text, so GitHub detects it; "Copyright
    2026 Niels-Erik Nandal" opens a License section in the README.
  - Delete the project-license issue; the root integrator removes its roadmap
    line.
  - Add `LICENSE`, `SECURITY.md`, `CONTRIBUTING.md`, `README.md`,
    `.github/ISSUE_TEMPLATE/*` and `site/*` to the documentation-only paths in
    `.github/workflows/verify.yml`, so they skip the builds.
  - [Cloud resources](issues/cloud-resource-cleanup.md) keeps its problem, impact
    and resolution; its inventory of project ids and numbers, app ids and
    service-account emails moves to the owner's private notes.
- Owner actions:
  - The Firebase client key once committed in `android/app/firebase.json`
    (`b5189e1` to `f21112c`) is already restricted to Firebase APIs. Delete it and
    close GitHub secret-scanning alert #1 as revoked, or keep it and close the
    alert as won't fix with that reason.
  - Limit pull requests to collaborators; enable private vulnerability reporting.
  - Set topics: `tmux`, `android`, `claude-code`, `codex`, `tailscale`, `terminal`,
    `go`, `jetpack-compose`. The description, homepage and social preview wait for
    W8.

Done when: GitHub detects Apache-2.0; alert #1 is closed; `git grep` finds no
Google Cloud project id, project number or service-account email in the tree; the
repository settings show collaborator-only pull requests, private vulnerability
reporting and the topics.

### W2 Host fixes

Goal: what a stranger meets first works, and the trust model is the one the docs
will state. Entry: the owner has settled every row of [proposed](#proposed-for-the-owner-before-w2)
and the open questions marked "before W2".

- W2.0 environments (owner approval): a fresh macOS 26 VM under tart, snapshotted
  clean; `ubuntu:24.04` and `archlinux` amd64 containers for package-manager
  behavior; ephemeral nodes on the owner's tailnet. Nothing touches the owner's
  machines. W3 and W4 reuse them.
- W2.1 `install` (`internal/install`, `scripts/install`):
  - PATH as decided. `internal/install/paths.go:16` links `skid` into
    `~/.local/bin`, which nothing adds to PATH; `shell.go` writes the blocks.
  - Arch: when `tailscaled` is inactive, say so and give the command. Under `curl |
    bash`, `pacman -S` reads its confirmation from the exhausted pipe; make it work
    or document why not.
  - Linux: when linger is off, say that logout stops the gateway and every session,
    and give the command; `services.go:122` only hints.
  - `install`, `update` and `uninstall` accept `-h`, print their help to stdout and
    exit 0; `uninstall` stops advertising flags it rejects.
  - Architecture: [installation](architecture.md#installation), through the
    root integrator. Paired dev-server: its digest of `scripts/install`.
  - Venue: the macOS VM; the containers for `apt` and `pacman`. The rest of Linux
    is `NOT_RUN`.
- W2.2 `cli` (`cmd/skidbladnir`, `internal/agentcli`):
  - Help: `-h`, `--help`, `help` and `help COMMAND` work, print to stdout and exit
    0; every command has its own help; `--version` and `skid --help` as decided;
    examples use neutral machine names, not the owner's hosts.
  - `skid connect` shows Tailscale's errors, which `fleet_cli.go:86` discards, and
    checks the operator, MagicDNS and HTTPS certificates first, as decided.
  - Fleet commands print human text with `--json`, as decided
    (`fleet_cli.go:140`).
  - Architecture: [fleet interfaces](architecture.md#fleet-interfaces) and
    [cli](architecture.md#cli), through the root integrator. Paired dev-server if
    it parses fleet output.
  - Venue: isolated runs for help and output; connect's checks in the W2.0
    environments.
- W2.3 `fleet-admin` (`internal/fleet`): `POST /v1/fleet/invites`, `DELETE
  /v1/fleet/members/*` and `POST /v1/fleet/apply` check authentication only
  (`internal/fleet/http.go:57-98`). They return the existing 403 `Forbidden`,
  "computer membership required" (`hub.go:223`), to phone bearers. Confirm the
  phone calls none of them. Architecture: [trust invariants](architecture.md#trust-invariants)
  and [fleet interfaces](architecture.md#fleet-interfaces), through the root
  integrator. Venue: an isolated loopback hub with a phone bearer and a computer
  bearer.
- W2.4 `release-integrity` (`scripts/release`, `scripts/check-release`,
  `scripts/check-android-signing`, the Android signing configuration,
  `android/app-signing-cert.sha256`):
  - `SHA256SUMS`, its signature and the third-party notices as decided; manual
    verification against the published public key is documented in W6.
  - The APK key as decided. v0.22.0's certificate is named `CN=Android Debug`
    (SHA-256 `7b2ba254…eddc47bf`). The owner generates the key and records it in
    the signing properties; the pin changes.
  - Release notes gain a Changes section built from merged pull request titles.
  - If it must touch `scripts/install`, it starts after W2.1 merges. Architecture:
    [release](architecture.md#release) (its asset list and four-entry
    `SHA256SUMS`), and `scripts/check`'s composition, through the root integrator.
    Paired dev-server: the release pin and its expected digests.
  - Venue: `scripts/check-release` on a local release build.
- Release (owner): cut from main after W2.1 to W2.4; hosts first, then the phone
  (uninstall, reinstall, re-pair); dev-server's pin follows.

Done when: each slice's behavior is shown by a temporary boundary test or a run in
its venue; `scripts/check verify` passes; the release is published and installed
on the owner's fleet.

### W3 Stranger walk

Goal: a person following only the published docs reaches a typing session on the
phone. Entry: the open questions marked "before W3" are settled.

- Restore the W2.0 snapshots; add an Android emulator with Play Services and the
  Tailscale app on the test tailnet.
- Install the W2 release on macOS from the README alone. Record every prompt,
  error and guess.
- Fix what is cheap in follow-up pull requests; record the rest as issues. Write
  `docs/install.md` from the transcript, not from memory.
- Update: install the previous release, then run `skid update`. The phone's
  signer changed in W2, so its in-place update from earlier releases fails by
  design; `docs/install.md` says so.
- Uninstall, and compare what remains with the README's claim.
- Release (owner) after the follow-ups.

Done when: macOS reaches a typing session on the emulator from the docs alone, or
each remaining failure has an issue and a stated reason. Linux is `NOT_RUN`.

### W4 Media

Goal: a reader sees skid work without installing it. Uses the W2.0 environments
and the emulator, never the owner's phone.

- Staged content only: sessions in groups, fake repositories, a lively shell, and
  agent panes painted by `internal/agentcontrol/testdata/frames/paint.py` from the
  authored frames under stub `codex` and `claude` commands. No real terminal
  content and no real accounts.
- The continuity shot: one session opened in the desktop browser, then the same
  terminal on the phone, typed into from each. Desktop by VHS or asciinema; phone by
  `scrcpy` with Android's demo-mode status bar; joined side by side with `ffmpeg`.
- Stills: the phone dashboard, terminal and forge; the desktop browser.
- The social card: 1280×640, Gold on Ink, content inside the central 1200×630.
- SVGs of the ship mark, valknut and fret band, starting from the drafts in the
  owner's private research notes and checked against the frozen sources; a few
  dwarf seals rendered once from `Ornament.kt`'s formula.
- Output: `site/assets/`. The README's video is a GitHub-hosted attachment, with a
  still in `site/assets/`.

Done when: the owner approves each asset; each site video is under 4 MB, each still
under 200 KB, the README video under 10 MB.

### W5 Legibility

Goal: the code and its history read well to a stranger.

- W5.1 `reading-path`: choose six to eight files that best carry the system's
  ideas, and add the path to the top of the [codebase map](codebase-map.md), one
  sentence per file on what it shows. Changes to the files themselves are limited
  to comments and names, proven by build and vet. First candidate:
  `internal/tmux/guard.go`, at-most-once effects inside tmux's own command queue.
  Others to vet: `internal/privatefile`, `internal/agentcontrol/screen.go`,
  `internal/fleet/http.go`, `Ornament.kt`.
- W5.2 `history`: `docs/history.md`, one page. A timeline, then about eight
  decisions as context, choice and consequence: specification first; tmux as the
  store; herdr adopted, then abandoned; the fixed hub; at-most-once over replay;
  screen inference over provider hooks; the 48% reduction. Mined from git; the
  owner checks every fact.
- W5.3 `roadmap` (root integrator): refresh the roadmap against the open issues.
- W5.4 `voice` (root integrator, last): correct capitalization across `docs/*.md`
  and AGENTS.md. Proof: for every changed file, Python's `str.casefold()` of the
  old and new text is identical, so only case changed.

Done when: the casefold proof passes; the owner approves the reading path and the
history; the roadmap matches the issues.

### W6 Front door

Goal: the README and docs carry a stranger from first glance to a working install,
and a reviewer from first glance to the architecture.

- README, about two screens: the mark and the line; the epigraph and the three
  clauses; one paragraph; the maturity statement; the demo; a requirements matrix;
  install (a platform table, inspect first, and the one-liner from
  `https://github.com/NielsdaWheelz/skidbladnir/releases/latest/download/install`);
  a numbered quickstart with real output; how it works (the hub is the control plane
  and tmux the data plane: lose the hub, keep the shells); when not to use skid;
  reading the code; where to go next, as a table from goal to page; FAQ, including
  the name; license.
- `docs/README.md`: an index separating using skid from how it is built.
- `docs/install.md` (from W3) and `docs/phone.md` (sideloading, checking the
  signer, Obtainium, optional push). The push procedure moves from architecture's
  [android push setup](architecture.md#android-push-setup), through the root
  integrator.
- `SECURITY.md`: private reporting; the threat model in plain terms (a device in
  the fleet is a shell on every machine; an agent that can run skid can drive every
  terminal; what is not a vulnerability), linking architecture §3.
- `CONTRIBUTING.md` (the issue-only policy and its reason); one issue form with
  blank issues disabled; the §4 sentence in the design language, as decided.
- Correct these README claims: updates are manual; uninstall also stops Tailscale
  Serve on :8443; Ubuntu derivatives are rejected while Arch derivatives are
  accepted; the configuration example's `claude-work` is a private alias.

Done when: a fresh agent given only the README can say what skid is, whether it
suits them, how to install it and where the architecture lives; every factual claim
is checked against the code at the pull request's head; a re-walk from the W2.0
macOS snapshot following the merged README and `docs/install.md` reaches a typing
session.

### W7 Site

Goal: skid.nielseriknandal.com shows the product, explains it and exhibits its
making.

- `/`: the hero (ship mark, name, pronunciation, the line, install); the epigraph
  and three clauses; the continuity demo; how it works (an SVG diagram drawn in the
  design language, and the architecture's five constraints); what it is not (the
  architecture's non-goals); the three questions as the comparison; install; when
  not to use it; a footer (GitHub, license, author).
- `/design`: the design language shown in itself: palette, cut corners, ship mark,
  valknut, fret band and seals; the provenance ledger with museum links; the name
  in Old Norse and in Brodeur; the fonts. Links to the [design language](design-language.md)
  and the history.
- A Vercel project with a preview deployment per pull request; the DNS-only CNAME
  (owner approval).

Done when: under 100 KB before media; no horizontal scroll at 360 px; the palette's
WCAG ratios hold; the Nu HTML Checker (`vnu`) reports no errors; the owner
approves.

### W8 Site details and repository metadata

Goal: the small things a careful visitor checks are right.

- `/install` redirects to the latest release's installer; the site's install
  section links to the script's source beside the one-liner.
- `curl skid.nielseriknandal.com` returns a plain-text page, which also serves as
  `/llms.txt`.
- OpenGraph and Twitter tags with the social card; a literal 404 ("Not found.").
- GitHub (owner approval): the line as the description, the homepage, the social
  preview; the repository pinned on the owner's profile.

Done when: each works from a clean browser and from `curl`.

### W9 Essay and close

Goal: the work is presented where a reviewer arrives, and the campaign leaves no
loose ends.

- The essay on nielseriknandal.com: the five constraints, architecture from first
  principles, linking to the site and repository.
- A project entry on nielseriknandal.com and a three-line summary for a résumé.
- Optional: Show HN, after the essay.
- Close: record what remains as issues, update the roadmap, delete this file.

Done when: the essay is published and this file is deleted.

## Open questions

- [ ] Before W2: `--config` names a state directory. Rename it, which touches
  dev-server and jarvis if they pass it, or keep it?
- [ ] Before W2: accept Ubuntu derivatives and Debian
  (`internal/install/platform.go:35-43`)? Default: no, documented.
- [ ] Before W3: pairing the emulator. Show the QR code through the emulator
  camera's virtual scene, or add a QR paste fallback, which also serves phones
  without Play Services but is a new capability whose scope the owner decides.
- [x] Before W5: `release-pin.json` at the root: keep, move or delete (carried from
  the cleanup campaign). Kept, 2026-10-26: the published-release gate, `verify.yml`
  and the architecture name it, and a root file states plainly which source built
  which bytes. Cost: one unexplained file in the root listing until W6's README
  explains it.
- [ ] Before W6: Google's Android developer verification, after W2.4's new key:
  register, and when?
- [ ] Before W6: macOS notarization ($99 a year). Default: no, documented.
- [ ] Before W9: release cadence and host/phone lockstep make a stranger update the
  phone often. Record as an issue?

## Progress

| wave | pull request | status | merged as |
| --- | --- | --- | --- |
| W0 plan | `publication/plan` | merged | #175, `2265104` |
| W1 trust floor | `publication/license`, owner actions | merged; alert #1 left open by the owner | #176, `db4188a` |
| W2.0 environments | owner approval | dropped: the owner skipped the container, VM and live connect checks | |
| W2.1 install | `publication/install` | merged | #178, `9e1e04a` |
| W2.2 cli | `publication/cli` | merged | #179, `b57b353` |
| W2.3 fleet admin | `publication/fleet-admin` | merged | #180, `abd943b` |
| W2.4 release integrity | `publication/release-integrity` | merged | #181, `6d54498` |
| W2.4 android key | `publication/android-key` | merged | #182, `677bbb2` |
| W2 release | owner | v0.23.0 published and pinned (#183, dev-server #208); devbox, macbook and arch updated 2026-10-26; the phone reinstalled with the new key and re-paired the same day | `677bbb2` |
| W3 stranger walk | `publication/install-docs`, follow-ups, release | dropped: the owner skipped the pristine macOS walk; W6 writes `docs/install.md` from the code, unwalked | |
| W4 media | `publication/media` | planned | |
| W5.1 reading path | `publication/reading-path` | merged; the owner approved the path | #187, `3396647` |
| W5.2 history | `publication/history` | merged; the owner approved it and gave the herdr reason | #188, `e8af534` |
| W5.3 roadmap | `publication/roadmap` | merged | #185, `d39fa0d` |
| W5.4 voice | `publication/voice`, `publication/voice-close` | merged; casefold and code-span proof for every file | #186, `2118c44`; #189 |
| W6 front door | `publication/front-door` | planned | |
| W7 site | `publication/site` | planned | |
| W8 site details | `publication/site-details`, owner actions | planned | |
| W9 essay and close | owner's site, `publication/close` | planned | |
