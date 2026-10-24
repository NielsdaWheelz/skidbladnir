# attention cutover

problem: v0.22.0 carries hub-owned attention (#144), host-rendered copy (#152)
and the go quota client (#145) without converters. it runs on devbox, macbook,
arch and the phone, but its notification behavior is unverified on that fleet.

impact: a notice that never reaches a device, or never clears, goes unnoticed
until a session waits on it. attention restarted on the hub at install: sessions
ready before it stay quiet until they work and idle again.

evidence: the hub writes `notification-observer-v2.json`
(`cmd/skidbladnir/standalone.go:66`); the phone's push store is `push-v3.json`
(`PushStore.kt:20`) and it re-registers fcm on its first resume. on 2026-10-24
all three computers and the phone joined one fleet with devbox as hub, every
application `applied`, and the phone listed the fleet's sessions. the v0.20.x
notification state was deleted from every host; no rollback remains. no
needs-input notice has been observed since.

resolved when, on the installed fleet:

- a needs-input notice reaches the mac app and the phone;
- a notice dismissed on the phone leaves the mac within seconds;
- entering a ready session from a tui clears its ready notice on the mac and the
  phone;
- the desktop usage page shows `ok` for each codex profile whose daemon runs (the
  go client was proven only against a fake app-server);
- with push configured, the phone's push status reaches `active`.
