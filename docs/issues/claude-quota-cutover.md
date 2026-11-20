# claude reset qualification and cutover

the gateway and footer must use the same claude profile-cache reader on each
host. the formatter invokes the local skid executable, so deploy the selected
skid release first. fable lives on the profile-usage route. the worker session
wire retains its grouped codex windows and omits claude terminal usage.
[v0.27.0](https://github.com/NielsdaWheelz/skidbladnir/releases/tag/v0.27.0)
is published and pinned. arch is enrolled and was offline in tailnet checks
on 2026-11-20 utc. macbook and devbox were reachable and both ran v0.26.0.
the switch needs arch reachable so an old gateway cannot rejoin after the
wire changes. [installed cache checks](claude-quota-memory.md) and fleet cutover
are `NOT_RUN`.

the running jarvis release, `e7f5916`, already admits grouped quota and its
database matched schema `0018` at preflight. replace its worker cli and restart
the assistant on that release. this cut does not require an application upgrade
or schema migration.

## cutover

1. require all enrolled hosts reachable. recheck the selected installer and
   pinned release, the companion formatter revision, and the installed jarvis
   reader before changing live services. [release](../architecture.md#release)
2. pause jarvis through its existing control, drain worker calls and account
   for entered effects and pending actions. close old skid browsers, then
   stop `jarvis.service`. keep the pause.
3. install the matching signed skid gateway and client across enrolled hosts.
   require gateway readiness at the selected release. if the installer calls
   for terminal-runtime replacement, pause for a separately agreed drain.
   then atomically install the reviewed dev-server formatter at
   `~/bin/claude-statusline` with mode 0755. devbox also needs the matching
   managed source asset. a full workstation apply is outside this cut.
4. while jarvis is stopped, install the matching worker cli through
   `deploy/install-agent-client` with the existing read-only fleet state.
   start the selected assistant with the existing schema after replacing
   the cli. retain prior action records and containment settings.
5. with the pause held, verify each gateway and the installed worker cli.
   exercise the selected jarvis list and info readers under the service’s
   restrictions. record outcomes without account or terminal content.
   reopen browsers and resume jarvis after the checks pass. after a grouped
   receipt is staged, preserve it and repair forward.
