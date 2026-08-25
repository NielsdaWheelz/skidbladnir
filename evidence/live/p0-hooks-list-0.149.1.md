# P0 Codex hook discovery and trust boundary

Recorded: 2026-08-25T05:03:14Z

Result: **PASS**

## Artifacts

- Codex: `0.149.1` / `rust-v0.149.1`
- Source commit: `ff29a44391deccde0aba0f8390337d7f3c319ea4`
- Hook config SHA-256: `63fb61eaab06309c5a5c537b69e9bf0eb478ff6c162824e6249fc46c08863e69`
- Static helper SHA-256: `b07d2bb68ab963165b0e00b4acc090701db2bc7423f0896a6b894b2663291d07`
- Hook lock SHA-256: `618b2337c00f2f6bf053f7222f9872776389c9c5cac328295e2a02c37a001fb3`

## Procedure

```text
go test ./internal/runtime/appserver ./internal/runtime/codex -count=1
go test -race ./internal/runtime/appserver ./internal/runtime/codex
go test -tags=live ./tests/live -run '^TestLivePinnedHooksTrustDiscovery$' -count=1 -v
```

The live proof started the digest-verified App Server with strict config and an
owner-only, credential-free temporary Codex home. The home contained the exact
committed seven-hook config. It requested `hooks/list` for one exact temporary
cwd, stopped the server, persisted the seven exact normalized hashes in
`config.toml`, restarted the server, and repeated the request.

## Observed result

- The exact pin returned seven singular, enabled, synchronous command hooks
  from the user source and no managed hook.
- Event, command, source path, persisted key shape, normalized hash, matcher,
  and timeout matched the independent reviewed fixture for every hook.
- With no persisted record every hook was `untrusted`; after an App Server
  restart with the exact records persisted every hook was `trusted`.
- The strict decoder rejected non-canonical paths before any transport write,
  unknown or duplicate fields, unknown enum variants, response errors, and
  numerics outside the exact schema ranges. The policy rejected missing,
  changed, duplicate, untrusted, or foreign runnable lifecycle hooks.
- The complete host-live suite passed afterward, and the temporary process,
  socket, home, and cwd were absent after cleanup.

No account, thread, prompt, terminal, or hook-payload content was used or
captured. This record proves exact-pin effective user-hook discovery and
persisted trust classification. Hook execution under `resume --remote`,
managed-source injection, the launcher's pre-exec refusal, binding ancestry,
native subagents, and Stop/continuation semantics remain open P0 proofs.
