# terminal observation has no live ordinary-provider smoke

problem: spec §8 asks for a live ordinary-provider smoke separately from the
scripted runs. every classifier qualification of the cutover drove codex and
claude against local scripted model endpoints, which prove TUI and protocol
behaviour, not service behaviour.

impact: screens only a real service produces are unexercised through the
implemented classifier: real pacing and streaming, rate limits and usage notices,
account and plan notices, server-gated layouts. a misread there is invisible
until a user meets it.

evidence: darwin and linux runs used scripted codex responses and claude messages
endpoints with dummy credentials; the no-credential scope excluded real services.
earlier managed-launch checks rendered the codex run-state line against the real
service before the classifier existed; they do not qualify classification.

resolved when: with current-turn approval and the user's chosen accounts, each
provider's managed launch runs against its real service on darwin and linux, and
idle, working and at least one request classify as expected, recorded
content-free in the
[qualification](../terminal-agent-control-qualification.md#terminal-observation-qualification);
then delete this record.
