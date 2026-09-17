package dev.niels.skidbladnir

import java.util.concurrent.Executor

internal data class PollRun(val sequence: Long, val startsNow: Boolean)

internal class CoalescingPollLane {
    private var activeSequence: Long? = null
    private var trailingSequence: Long? = null
    private var nextSequence = 1L

    @Synchronized
    fun request(requireTrailing: Boolean = false): PollRun? {
        if (activeSequence == null) {
            val sequence = nextSequence++
            activeSequence = sequence
            return PollRun(sequence, startsNow = true)
        }
        if (!requireTrailing) return null
        val sequence = trailingSequence ?: nextSequence++.also { trailingSequence = it }
        return PollRun(sequence, startsNow = false)
    }

    @Synchronized
    fun finish(completedSequence: Long): PollRun? {
        check(activeSequence == completedSequence)
        val trailing = trailingSequence
        if (trailing != null) {
            activeSequence = trailing
            trailingSequence = null
            return PollRun(trailing, startsNow = true)
        }
        activeSequence = null
        return null
    }

    @Synchronized
    fun abort() {
        activeSequence = null
        trailingSequence = null
    }
}

internal class AwaitedInventoryReads {
    private val requiredSequenceByMachine = mutableMapOf<MachineHandle, Long>()

    val isActive: Boolean get() = requiredSequenceByMachine.isNotEmpty()

    fun requireRead(handle: MachineHandle, sequence: Long) {
        val current = requiredSequenceByMachine[handle]
        check(current == null || sequence >= current)
        requiredSequenceByMachine[handle] = sequence
    }

    fun readLanded(handle: MachineHandle, completedSequence: Long) {
        val required = requiredSequenceByMachine[handle] ?: return
        if (completedSequence >= required) requiredSequenceByMachine.remove(handle)
    }

    fun stop(handle: MachineHandle) { requiredSequenceByMachine.remove(handle) }

    fun clear() { requiredSequenceByMachine.clear() }
}

internal class InventoryOperationLane(
    private val executor: Executor,
    private val onDefect: (RuntimeException) -> Unit,
) {
    private val queued = ArrayDeque<() -> Unit>()
    private var draining = false
    private var submittedMutationFence = 0L
    private var completedMutationFence = 0L

    @Synchronized
    fun submitRead(action: (Long) -> Unit) {
        enqueue { action(completedMutationFence) }
    }

    @Synchronized
    fun submitMutation(onReserved: (Long) -> Unit, action: (Long) -> Unit) {
        val fence = ++submittedMutationFence
        onReserved(fence)
        enqueue {
            try {
                action(fence)
            } finally {
                synchronized(this) { completedMutationFence = fence }
            }
        }
    }

    private fun enqueue(action: () -> Unit) {
        queued.addLast(action)
        if (draining) return
        draining = true
        executor.execute(::drain)
    }

    private fun drain() {
        while (true) {
            val action = synchronized(this) {
                if (queued.isEmpty()) {
                    draining = false
                    return
                }
                queued.removeFirst()
            }
            try {
                action()
            } catch (defect: RuntimeException) {
                onDefect(defect)
            }
        }
    }
}

internal fun submitCoalescedInventoryRead(
    operations: InventoryOperationLane,
    polls: CoalescingPollLane,
    initialRun: PollRun,
    read: (PollRun, Long) -> Unit,
) {
    operations.submitRead { completedMutationFence ->
        try {
            read(initialRun, completedMutationFence)
            val trailing = polls.finish(initialRun.sequence)
            if (trailing != null) {
                submitCoalescedInventoryRead(operations, polls, trailing, read)
            }
        } catch (defect: RuntimeException) {
            polls.abort()
            throw defect
        }
    }
}
