# timing

## scope

clocks, expiry, deadlines, and recurring work.

## clocks and intervals

- use the clock owned by the fact being measured. do not compare host clocks to
  each other or replace host observations with a client receipt time.
- represent stored or transmitted instants with an explicit time zone. use a
  monotonic elapsed clock for local durations and freshness where available.
- expiry is the first invalid instant: valid while `now < expiresAt`, expired
  once `now >= expiresAt`. pairing's gateway owns both issuance and redemption.
- other threshold inclusivity follows its product contract. the
  [architecture](../architecture.md) owns activity and freshness clocks;
  activity's inclusive threshold is not an expiry rule.

## deadlines and recurring work

- express durations in native units: go `time.Duration`, kotlin duration values
  or explicitly named millisecond values, and named millisecond values in js.
- give reused timings or product limits meaningful names. keep a one-use value
  inline when its operation and unit already explain it.
- keep cadence, attempt limits, and termination behavior visible in the owning
  loop. propagate cancellation and deadlines through blocking work.
- a caller's overall deadline and a child operation's local timeout protect
  different scopes. neither may silently extend the other.
- stop timers and recurring work with their owner. see [effect.md](effect.md).
