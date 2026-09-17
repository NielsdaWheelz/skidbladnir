# concurrency

## scope

concurrent execution and mutation linearization. [retries.md](retries.md) owns
retry semantics; [mutation-ordering.md](mutation-ordering.md) owns ordering
across ownership boundaries.

## rules

- requests within a gateway may run concurrently. each gateway owns its local
  tmux connection and state; there is no cross-host transaction or coordinator.
- a mutation's linearization point is where its effect takes place. concurrent
  mutations must preserve the owning operation's invariants and ordering contract.
- use the existing owner's synchronization for shared in-process state. a local
  mutex does not serialize independent tmux clients or external processes.
- keep a target check and its guarded mutation together at the authoritative
  boundary. a preceding read alone cannot protect a later write.
- the [host architecture](../architecture.md#5-host-architecture) owns tmux
  command-queue predicates and session mutation locking. [agent control](../agent-control.md)
  owns process-lifetime checks and the limits of terminal delivery.
- preserve those limits. do not imply atomicity across independent systems or
  claim protection against a hostile same-user process.
- release locks when their protected work ends. do not hold them across unrelated
  provider calls or client interaction.
- reads spanning independently changing resources must handle disappearance and
  stale observations according to the product contract.
