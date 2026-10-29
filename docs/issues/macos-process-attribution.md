# macos attributes tmux and providers to skid

problem: tmux and providers have separate process lifetimes but inherit skid's
macos responsibility and resource-accounting group. detachment and a unix parent
of launchd do not establish attribution independence.

impact: provider resource use is grouped with skid, obscuring gateway overhead;
provider file access can produce permission requests naming skid. separating the
accounting would not reduce actual energy consumption
([energy measurement](tmux-exec-overhead.md)).

evidence: 2026-10-29, macos 27.0.1, loaded gateway v0.23.0 (`677bbb2`), tmux 3.8.
read-only `responsibility_get_pid_responsible_for_pid` and
`proc_pidinfo(PROC_PIDCOALITIONINFO)` queries found gateway pid 1118 responsible
for tmux pid 1194 and every sampled codex and claude process. all shared resource
coalition 1156 and jetsam coalition 1157. tmux's unix parent was already launchd
(pid 1). the notification app had its own responsibility and coalitions.

cause in that deployed release: the gateway directly invoked tmux, which could
start its server during creation or recovery. panes then launched `terminal-exec`,
which replaced itself with the shell. no explicit macos attribution boundary existed.
launchd's `AbandonProcessGroup` preserved descendant lifetimes;
`ProcessType=Interactive` intentionally preserved inherited priority. neither separated
responsibility or resource accounting. apple describes the privacy responsibility
model in [on file system permissions](https://developer.apple.com/forums/thread/678819).

follow-up: qualify the implemented [native terminal service](../architecture.md#terminal-service):
an independently supervised signed host supplies the intended provider identity;
the skid installer provisions it. native identity/consent checks are `NOT_RUN`. drain existing
work for one clean cutover; preserve tmux ownership, native execution, usable priority
and survival across gateway restarts.

resolved when: the agreed attribution boundaries are observed in macos
responsibility queries, resource accounting and controlled file-access requests
for both creation and recovery; gateway restart still leaves sessions and agents
running. this does not close the separate
[repeated-permission investigation](macos-permission-prompts.md).
