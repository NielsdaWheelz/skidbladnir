# Contributing

Issues are welcome: bugs, questions and requests within the
[scope](docs/architecture.md#1-scope). Pull requests are limited to the
repository's collaborators, so outside changes arrive as issues, not patches.
Report a vulnerability privately, as the [security policy](SECURITY.md) describes,
never in an issue.

## Why issues and not pull requests

Skíðblaðnir is open source, not open contribution. It is a single-user prototype,
built and run by one person against a written contract, the
[architecture](docs/architecture.md). Most of its mechanisms follow from a few
constraints: tmux is the store, every effect is attempted at most once on an exact
target, and each computer acts only on its own tmux. A small patch can break one of
them in a way that neither the diff nor a test shows, so reviewing an outside change
costs more than making the change from a clear report. One owner also means one
whole understanding of the system, and that understanding is most of what keeps it
correct.

The [license](LICENSE) lets anyone fork and change it.

## Filing a good issue

The [issue form](https://github.com/NielsdaWheelz/skidbladnir/issues/new/choose)
asks for:

- **Versions.** `skid version` on each computer involved, which prints the release
  and its source commit, and the app's version from Android's App info for
  Skíðblaðnir. Computers and the phone should run the same release; `skid status`
  on the hub shows each computer's.
- **Platform.** The macOS version, or the Linux distribution and version; the
  Android version and phone model.
- **What happened, and what you expected.** The exact command, its error text and
  exit code, or what the screen showed, described in words.
- **Steps that reproduce it**, from a state anyone could reach.

For a failure inside the service, its log helps: `journalctl --user -u skidbladnir`
on Linux, or `serve.log` in `${XDG_STATE_HOME:-~/.local/state}/skidbladnir` on
macOS. It records codes, IDs and durations, never terminal content.

Leave out terminal content, prompts, bearers, invitation lines
(`skid://connect/…`), `fleet.json`, provider account details and real tailnet
names; write `example.ts.net` instead.

No issue is promised a reply, a fix or a date. Requests that match a
[non-goal](docs/architecture.md#non-goals) are declined; any new capability needs
the owner's explicit scope decision.

## How changes are made

The owner's own changes follow [AGENTS.md](AGENTS.md) and the
[codebase rules](docs/rules.md), which say how a change is scoped, written and
proven. To read the code, start from the [docs index](docs/README.md) and the
reading path at the top of the [codebase map](docs/codebase-map.md).
