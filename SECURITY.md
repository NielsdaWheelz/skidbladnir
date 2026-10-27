# Security policy

## Reporting a vulnerability

Report a vulnerability privately through GitHub's private vulnerability reporting:
open the repository's **Security** tab and choose **Report a vulnerability**, or go
straight to [the form](https://github.com/NielsdaWheelz/skidbladnir/security/advisories/new).
Please do not open a public issue.

Include the release (`skid version` on each computer involved, and the app's
version from Android's App info for Skíðblaðnir), the platform and the steps that
reproduce the problem. Leave out bearers, invitation lines, `fleet.json`, terminal
content and real tailnet names; write `example.ts.net` instead.

Skíðblaðnir has one maintainer, so no response time is promised. Fixes ship in a
new release.

## Supported versions

Only the latest release. `skid update` moves a computer to it, and the phone takes
the APK from the same release.

## Threat model

Skíðblaðnir is built for one person and one trust domain: your tailnet, plus one
bearer for each device you enroll. [Architecture §3](docs/architecture.md#3-topology-and-trust)
states it exactly. In plain terms:

- **A device in the fleet is a shell on every machine.** Any enrolled device, phone
  or computer, can list, open, type into and close the terminals of every enrolled
  computer. If a device is lost, run `skid remove DEVICE_ID` on an enrolled
  computer; `skid status` lists the IDs. The hub refuses the device at once, and
  every other computer at its next fetch from the hub. While the hub is
  unreachable, the others keep admitting it.
- **An agent that can run skid can drive every terminal.** The `skid` command is
  on `PATH` in your shells, and any program running as you on an enrolled computer
  can read that computer's bearer. An agent with a shell can therefore read and
  type into every terminal on every machine. Skíðblaðnir offers no containment
  between programs that run as you.
- **Remote access is your tailnet, and every request needs a credential.** Each
  computer's gateway listens only on `127.0.0.1:7341`. `skid connect` publishes it
  to your tailnet with Tailscale Serve on HTTPS port 8443, and nothing else exposes
  it. Each device mints its own random 256-bit bearer and shares only its SHA-256
  verifier; joining takes a five-minute, single-use invitation instead. Arriving
  over loopback or the tailnet grants nothing by itself.
- **Computers administer; phones are clients.** The hub refuses a phone's bearer
  for invitations, removals and applying settings.
- **Credentials stay where they are.** Provider logins never leave their host.
  Android push messages, when configured, carry no session content. The phone
  keeps its bearer encrypted under an Android Keystore key and out of backups; the
  app has no lock of its own, so the phone's screen lock guards it.

## What is a vulnerability

For example:

- reaching a gateway's terminals, or the hub's fleet routes, without a valid
  bearer or invitation;
- a phone's bearer inviting, removing or applying settings;
- a web page, or another Unix user on an enrolled computer, driving a gateway;
- another Unix user reading skid's private state;
- `bash install` or `skid update` accepting an archive the release key did not
  sign;
- bearers, terminal content or provider credentials in logs or push messages.

## What is not

These follow from the model above:

- anything that needs code execution or file access as your user on an enrolled
  computer, such as reading `fleet.json`, running `skid`, or editing `config.yaml`
  or skid's shell blocks;
- anything an enrolled device can do, including in the hands of someone holding
  it unlocked, or of an agent misled by hostile input;
- configured commands running as written: a profile's `command` is ordinary shell
  text, run in your shell;
- a removed device still reaching computers that have not yet heard from the hub,
  and the other accepted limits under
  [trust invariants](docs/architecture.md#trust-invariants);
- attacks that need control of your tailnet or of Tailscale itself, such as a
  compromised tailnet administrator.
