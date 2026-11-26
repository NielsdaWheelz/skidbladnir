package dev.niels.skidbladnir

import java.util.UUID
import kotlinx.coroutines.NonCancellable
import kotlinx.coroutines.async
import kotlinx.coroutines.awaitAll
import kotlinx.coroutines.withTimeoutOrNull
import kotlinx.coroutines.withContext
import kotlinx.serialization.Serializable
import kotlinx.serialization.decodeFromString
import kotlinx.serialization.encodeToString

@Serializable internal data class BranchTarget(val tmuxId: String, val identityToken: String, val paneId: String)
@Serializable internal data class BranchNode(
    val target: BranchTarget, val name: String,
    val parent: SessionLifetime? = null, val children: List<SessionLifetime> = emptyList(),
    val pending: Boolean, val previous: List<SessionLifetime> = emptyList(),
)
@Serializable private data class BranchRequest(
    val action: String, val identityToken: String,
    val machine: MachineHandle? = null, val id: String? = null, val child: SessionLifetime? = null,
    val closeId: String? = null, val mode: String? = null, val expected: BranchNode? = null,
    val parent: SessionLifetime? = null,
)
@Serializable private data class Admission(val id: String)

internal data class BranchMember(
    val lifetime: SessionLifetime, val node: BranchNode, val machineName: String,
    val registeredAs: SessionLifetime, val inParent: SessionLifetime?,
    val obsolete: List<SessionLifetime>,
) {
    val label: String get() = "${node.name} on $machineName"
}

internal data class BranchPlan(val members: List<BranchMember>, val terminalOnly: Boolean)

internal data class BranchOutcome(
    val closed: Set<SessionLifetime>, val message: String, val complete: Boolean,
)
internal data class CreatedChild(val session: TmuxSession, val warning: String? = null)

private fun operationId() = UUID.randomUUID().toString().replace("-", "")
private class BranchFailure(val failure: GatewayFailure) : Exception(gatewayFailureMessage(failure))
private fun refuse(message: String): Nothing =
    throw BranchFailure(GatewayFailure.Api(ApiErrorCode.InvalidRequest, message, MutationDispatch.NotSent))
private fun <T> GatewayResult<T>.requireSuccess(): T = when (this) {
    is GatewayResult.Success -> value
    is GatewayResult.Failure -> throw BranchFailure(failure)
}

/** Each request goes directly to its owning machine. A plan holds exact observations through confirmation. */
internal class Branches(private val machines: Machines) {
    private fun credential(machine: MachineHandle): MachineCredential =
        machines.credential(machine)?.takeIf { it.machine.origin != null } ?: refuse("branch machine unavailable: ${machine.encoded}")

    private suspend inline fun <T> request(
        ref: SessionLifetime, body: BranchRequest, status: Int, crossinline decode: (String) -> T,
    ): T = GatewayClient.call(credential(ref.machine), "POST", listOf("v1", "sessions", ref.tmuxId, "branch"),
        productJson.encodeToString(body), status, { decode(it) }).requireSuccess()

    private suspend fun change(ref: SessionLifetime, body: BranchRequest) =
        request(ref, body, 204) { Unit }

    suspend fun create(destination: MachineCredential, draft: ForgeDraft): GatewayResult<CreatedChild> {
        val parent = draft.parent?.lifetime
        if (parent == null) return when (val result = GatewayClient.createSession(destination, draft)) {
            is GatewayResult.Success -> GatewayResult.Success(CreatedChild(result.value))
            is GatewayResult.Failure -> result
        }
        val id = operationId()
        suspend fun complete(child: SessionLifetime?) =
            change(parent, BranchRequest("complete", parent.identityToken, id = id, child = child))
        try {
            request(parent, BranchRequest("reserve", parent.identityToken, machine = destination.machine.handle, id = id), 200) {
                productJson.decodeFromString<Admission>(it)
            }
        } catch (failure: BranchFailure) {
            val definite = gatewayRefusedBeforeDispatch(failure.failure)
            val cleanup = if (definite) "" else try { complete(null); "" } catch (lost: BranchFailure) {
                " parent admission cleanup unconfirmed: ${lost.message}"
            }
            return GatewayResult.Failure(GatewayFailure.Api(ApiErrorCode.InvalidRequest,
                "no session created. ${failure.message}$cleanup", MutationDispatch.NotSent))
        }
        return when (val created = GatewayClient.createSession(destination, draft, id)) {
            is GatewayResult.Success -> {
                val ref = SessionTarget(destination.machine.handle, created.value).lifetime
                val warning = try { complete(ref); null } catch (lost: BranchFailure) {
                    "created ${created.value.tmuxName}; parent registration unconfirmed: ${lost.message}"
                }
                GatewayResult.Success(CreatedChild(created.value, warning))
            }
            is GatewayResult.Failure -> {
                if (gatewayRefusedBeforeDispatch(created.failure)) {
                    try { complete(null) } catch (lost: BranchFailure) {
                        return GatewayResult.Failure(GatewayFailure.Api(ApiErrorCode.InvalidRequest,
                            "${gatewayFailureMessage(created.failure)} parent admission cleanup unconfirmed: ${lost.message}",
                            MutationDispatch.NotSent))
                    }
                }
                created
            }
        }
    }

    suspend fun promote(target: SessionTarget): GatewayResult<String?> {
        val ref = target.lifetime
        val parent = target.session.parent ?: return GatewayResult.Success(null)
        return try {
            change(ref, BranchRequest("promote", ref.identityToken, parent = parent))
            val warning = try {
                unlinkFromParent(parent, ref, target.session.previous)
                null
            } catch (lost: BranchFailure) {
                "session is top-level. former parent registration cleanup unconfirmed: ${lost.message}"
            }
            GatewayResult.Success(warning)
        } catch (lost: BranchFailure) { GatewayResult.Failure(lost.failure) }
    }

    private suspend fun inspect(ref: SessionLifetime): BranchNode =
        request(ref, BranchRequest("inspect", ref.identityToken), 200) { productJson.decodeFromString<BranchNode>(it) }

    private suspend fun unlinkFromParent(parent: SessionLifetime, child: SessionLifetime, previous: List<SessionLifetime>) {
        val node = try { inspect(parent) } catch (lost: BranchFailure) {
            if ((lost.failure as? GatewayFailure.Api)?.code == ApiErrorCode.SessionNotFound) return
            throw lost
        }
        val current = SessionLifetime(parent.machine, node.target.tmuxId, node.target.identityToken)
        if (current != parent && parent !in node.previous) refuse("former parent identity unresolved.")
        for (edge in node.children.filter { it == child || it in previous }) {
            change(current, BranchRequest("unlink", current.identityToken, child = edge))
        }
    }

    /** Only explicit promotion or the owning host's proof of absence retires a registration. */
    private suspend fun inspectChild(ref: SessionLifetime, parent: BranchMember): BranchNode? {
        val node = try { inspect(ref) } catch (missing: BranchFailure) {
            if ((missing.failure as? GatewayFailure.Api)?.code == ApiErrorCode.SessionNotFound) return null
            throw missing
        }
        val current = SessionLifetime(ref.machine, node.target.tmuxId, node.target.identityToken)
        if (ref != current && ref !in node.previous) refuse("child identity unresolved. review the branch again.")
        val owner = node.parent ?: return null
        if (owner != parent.lifetime && owner !in parent.node.previous) refuse("child ownership unresolved. nothing closed.")
        return node
    }

    suspend fun preview(target: SessionTarget, terminalOnly: Boolean): GatewayResult<BranchPlan> = try {
        val members = mutableListOf<BranchMember>()
        val seen = mutableSetOf<SessionLifetime>()
        suspend fun visit(ref: SessionLifetime, parent: BranchMember?): Boolean {
            val node = if (parent == null) inspect(ref) else inspectChild(ref, parent) ?: return false
            val current = SessionLifetime(ref.machine, node.target.tmuxId, node.target.identityToken)
            if (parent == null && current != ref) refuse("session replaced. review the branch again.")
            if (!seen.add(current)) refuse("the branch changed. review it again.")
            if (node.pending) refuse("${node.name} has an unresolved child creation. nothing closed.")
            if (parent == null && !terminalOnly && node.target.paneId != target.session.activePaneId) refuse("the selected pane changed. review the session again.")
            val member = BranchMember(current, node, credential(ref.machine).machine.label.text, ref, parent?.lifetime, emptyList())
            val obsolete = node.children.filterNot { visit(it, member) }
            members += member.copy(obsolete = obsolete)
            return true
        }
        visit(target.lifetime, null)
        GatewayResult.Success(BranchPlan(members, terminalOnly))
    } catch (lost: BranchFailure) { GatewayResult.Failure(lost.failure) }

    suspend fun close(plan: BranchPlan): BranchOutcome {
        val closeId = operationId()
        val mode = if (plan.terminalOnly) "terminal" else "interrupt"
        val prepared = mutableListOf<BranchMember>()
        val outcomes = mutableMapOf<SessionLifetime, TerminalClosure>()
        val interrupts = mutableMapOf<SessionLifetime, TerminalInterrupt>()
        val errors = mutableListOf<String>()
        try {
            for (member in plan.members.asReversed()) {
                val ref = member.lifetime
                try {
                    change(ref, BranchRequest("prepare", ref.identityToken, closeId = closeId, mode = mode, expected = member.node))
                } catch (lost: BranchFailure) {
                    // Only an uncertain reply can have installed our fence.
                    if (!gatewayRefusedBeforeDispatch(lost.failure)) prepared += member
                    throw lost
                }
                prepared += member
            }
            for (member in plan.members) {
                for (stale in member.obsolete) {
                    if (inspectChild(stale, member) != null) refuse("the branch changed. review the targets again.")
                }
            }
            for (member in plan.members) {
                val ref = member.lifetime
                for (stale in member.obsolete) {
                    change(ref, BranchRequest("unlink", ref.identityToken, child = stale, closeId = closeId))
                }
                val host = credential(ref.machine)
                val result = machines.changeInventory(ref.machine) {
                    if (plan.terminalOnly) {
                        when (val closed = GatewayClient.closeTerminal(host, ref, closeId)) {
                            is GatewayResult.Failure -> closed
                            is GatewayResult.Success -> GatewayResult.Success(TerminalClosure.Closed)
                        }
                    } else when (val closed = GatewayClient.interruptAndCloseTerminal(host, ref, member.node.target.paneId, closeId)) {
                        is GatewayResult.Failure -> closed
                        is GatewayResult.Success -> {
                            interrupts[ref] = closed.value.interrupt
                            GatewayResult.Success(closed.value.terminal)
                        }
                    }
                }
                if (result is GatewayResult.Failure) {
                    val notSent = gatewayRefusedBeforeDispatch(result.failure)
                    outcomes[ref] = if (notSent) TerminalClosure.NotClosed else TerminalClosure.Unknown
                    if (!plan.terminalOnly) interrupts[ref] = if (notSent) TerminalInterrupt.NotSent else TerminalInterrupt.Unknown
                    throw BranchFailure(result.failure)
                }
                val closed = result.requireSuccess()
                outcomes[ref] = closed
                if (closed != TerminalClosure.Closed) break
                if (member.node.parent != null) {
                    try {
                        inspect(ref)
                        refuse("${member.label} closed, but its host still resolves its lifetime. branch close stopped.")
                    } catch (lost: BranchFailure) {
                        if ((lost.failure as? GatewayFailure.Api)?.code != ApiErrorCode.SessionNotFound) throw lost
                    }
                }
                if (member.inParent != null) {
                    val parent = member.inParent
                    change(parent, BranchRequest("unlink", parent.identityToken, child = member.registeredAs, closeId = closeId))
                } else if (member.node.parent != null) {
                    unlinkFromParent(member.node.parent, ref, member.node.previous)
                }
                if (interrupts[ref] == TerminalInterrupt.Unknown) break
            }
        } catch (lost: BranchFailure) {
            errors += lost.message.orEmpty()
        } finally {
            errors += withContext(NonCancellable) {
                prepared.filter { outcomes[it.lifetime] != TerminalClosure.Closed }.map { member ->
                    async {
                        val ref = member.lifetime
                        val released = try {
                            withTimeoutOrNull(3_000) {
                                change(ref, BranchRequest("release", ref.identityToken, closeId = closeId))
                                true
                            } == true
                        } catch (lost: BranchFailure) {
                            (lost.failure as? GatewayFailure.Api)?.code == ApiErrorCode.SessionNotFound
                        }
                        if (released) null else "${member.label}: close fence release unconfirmed."
                    }
                }.awaitAll().filterNotNull()
            }
        }
        val lines = plan.members.map { member ->
            val ref = member.lifetime
            val closure = when (outcomes[ref]) {
                TerminalClosure.Closed -> "closed"
                TerminalClosure.NotClosed -> "left open"
                TerminalClosure.Unknown -> "closure unknown"
                null -> "remaining"
            }
            val interrupt = if (plan.terminalOnly) "" else when (interrupts[ref]) {
                TerminalInterrupt.Written -> " · interrupt sent"
                TerminalInterrupt.Unknown -> " · interrupt unknown"
                TerminalInterrupt.NotSent, null -> " · interrupt not sent"
            }
            "${member.label} · $closure$interrupt"
        }
        val closed = outcomes.filterValues { it == TerminalClosure.Closed }.keys
        return BranchOutcome(closed, (errors + lines).joinToString("\n"),
            errors.isEmpty() && closed.size == plan.members.size && interrupts.values.all { it == TerminalInterrupt.Written })
    }
}
