# effects

## scope

effectful work, background tasks, cancellation, and resource lifetime.

## execution

- use ordinary functions and the language's existing error and async mechanisms.
  make dependencies, side effects, and failure results visible at the call site.
- the owner starting concurrent work also owns its cancellation, completion,
  and error handling. use the existing contexts, wait groups, futures, or
  promises; do not add a runtime framework around them.
- keep concurrency and its result handling together. a goroutine or callback
  must not silently outlive the operation or resource it uses.
- bind stream producers and monitors to the stream lifecycle. shutdown must
  cancel work, unblock pending io, and observe worker completion.
- work that intentionally outlives its caller needs a concrete longer-lived
  owner and an explicit shutdown path, not an unobserved detached task.

## resources

- keep acquisition, use, and release within the owning lifetime. use native
  cleanup mechanisms and make release ordering explicit.
- a value depending on an open resource must not escape the resource's lifetime
  through a return value, closure, mutable reference, or background task.
- when returning an owning handle, make the caller's release responsibility
  explicit. do not return a borrowed value whose owner has already closed.
- preserve product lifetime distinctions: closing a gateway attachment releases
  its transport, pty, and tmux client, not the tmux session or foreground agent.
  the [architecture](../architecture.md) owns that boundary.
