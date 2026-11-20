# claude reset qualification and cutover

skid reads claude’s structured reset instant through the managed statusline.
on the macbook, both renderers carried eligible dates through the gateway and
desktop, including after narrowing the pane. `claude-work` did not supply a
five-hour window. its weekly date agreed with `/usage`. for `claude-work2`,
the independent `/usage` view reported both dates exactly one minute earlier
than the statusline input. that comparison failed. the owner accepted the
one-minute difference and chose to display the explicit statusline instant. [provider field](https://code.claude.com/docs/en/statusline#rate-limit-usage)

arch and devbox renderer acceptance and installed fleet acceptance are
`NOT_RUN`. source qualification and release artifacts are separate.

## cutover

1. wait for fable usage to land. select the combined revisions and rerun the
   quota and worker compatibility checks before signing. require hosted checks
   and a signed skid release verified against the exact source and asset digests.
   stage the same-schema jarvis release and qualified contained host.
   [release](../architecture.md#release)
2. pause jarvis through its existing control, drain worker calls and account
   for entered effects and pending actions. close old skid browsers, then
   stop `jarvis.service` and `jarvis-memory.service`. keep the pause.
3. install the matching signed skid gateway and client across enrolled hosts.
   require gateway readiness at the selected release. if the installer calls
   for terminal-runtime replacement, pause for a separately agreed drain.
   then atomically install the reviewed dev-server formatter at
   `~/bin/claude-statusline` with mode 0755. devbox also needs the matching
   managed source asset. a full workstation apply is outside this cut.
4. while jarvis is stopped, install the matching worker cli through
   `deploy/install-agent-client` with the existing read-only fleet state.
   install its exact contained host revision and activate through
   `deploy/activate-release` with the existing schema. retain the prior
   action records and containment settings.
5. with the pause held, verify each gateway and the installed worker cli.
   exercise the selected jarvis list and info readers under the service’s
   restrictions. record outcomes without account or terminal content.
   reopen browsers and resume jarvis after the checks pass. after a grouped
   receipt is staged, preserve it and repair forward.
