# inventory ordering inside the controller

problem: `SkidbladnirController.kt` contains both screen orchestration and the
independent polling/mutation ordering algorithms, making their invariants harder
to inspect. `MachineInventoryOperations` adds a one-caller map wrapper.

evidence: `CoalescingPollLane`, `AwaitedInventoryReads`, `InventoryOperationLane`,
and `submitCoalescedInventoryRead` form a cohesive block. controller consumers
already use their explicit operations.

resolved when: give those algorithms one cohesive source file and inline the
map wrapper in its controller owner. characterize coalescing, trailing reads,
mutation fences, and per-machine independence before moving code. preserve
ordering and lifecycle behavior; a smaller file alone is not acceptance.
