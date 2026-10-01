# deployed host configuration still uses the argument0 signature

problem: the installed generations' `host-config.json` carries the retired claude
signature `{"argument0": …}`. the observation source rejects `argument0` as an
unknown member, so the new binary refuses that file at gateway startup, in
`validate-host-config` and in `agent-hook`.

impact: installing the new binary over the old configuration stops the gateway
from starting, and claude sessions start unregistered because the hook publishes
nothing. the cutover must re-render the configuration from the current template
in the same generation ([deployment schema](../dev-server-handoff.md#host-config-and-validator)).

evidence: on devbox and arch the deployed file, read in place, is refused (not
canonical: unknown member); a copy with only `argument0` replaced by
`executablePath` is admitted, and the current template rendered with each host's
paths is admitted. the macbook's installed configuration carried the same
argument-only claude signature when last inspected.

resolved when: the cutover renders each host's configuration from the current
template, `validate-host-config` admits it on darwin and both linux hosts, and the
gateway and identity hook start with it; then delete this record.
