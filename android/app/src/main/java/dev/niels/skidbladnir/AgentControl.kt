package dev.niels.skidbladnir

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.decodeFromJsonElement

@Serializable
internal enum class AgentState {
    @SerialName("working") Working,
    @SerialName("blocked") Blocked,
    @SerialName("idle") Idle,
    @SerialName("done") Done,
    @SerialName("failed") Failed,
    @SerialName("stopped") Stopped,
    @SerialName("unknown") Unknown,
}

@Serializable
internal enum class AgentMethod {
    @SerialName("native") Native,
    @SerialName("terminal") Terminal,
    @SerialName("unavailable") Unavailable,
}

@Serializable
internal enum class AgentReason {
    @SerialName("permission") Permission,
    @SerialName("input") Input,
    @SerialName("dialog") Dialog,
    @SerialName("provider_unavailable") ProviderUnavailable,
    @SerialName("unrecognized") Unrecognized,
}

@Serializable
internal data class AgentStatus(val state: AgentState, val source: AgentMethod, val reason: AgentReason? = null)

@Serializable
internal data class AgentMethods(val read: AgentMethod, val send: AgentMethod, val interrupt: AgentMethod)

@Serializable
private data class AgentControlRequest(val identityToken: String, val paneId: String, val pid: Long, val startIdentity: String)

internal fun encodeAgentControlRequest(target: SessionTarget): String {
    val agent = requireNotNull(target.session.agent)
    return productJson.encodeToString(AgentControlRequest.serializer(), AgentControlRequest(
        target.session.identityToken, agent.paneId, agent.pid, agent.startIdentity,
    ))
}

@Serializable
internal data class AgentInterruptResult(val method: AgentMethod, val outcome: String, val turnId: String? = null)

@Serializable
internal data class AgentStopResult(val agent: String, val terminal: String, val reason: String? = null)

internal fun decodeAgentInterruptResult(encoded: String): AgentInterruptResult = decodeProtocol {
    productJson.decodeFromJsonElement<AgentInterruptResult>(strictJsonObject(encoded)).also {
        require(it.method != AgentMethod.Unavailable)
        require(it.outcome in setOf("accepted", "written", "interrupted", "finished", "unknown"))
    }
}

internal fun decodeAgentStopResult(encoded: String): AgentStopResult = decodeProtocol {
    productJson.decodeFromJsonElement<AgentStopResult>(strictJsonObject(encoded)).also {
        require(it.agent in setOf("stopped", "interrupted", "idle", "unconfirmed"))
        require(it.terminal in setOf("closed", "unconfirmed"))
        require(it.reason == null || it.reason in setOf("stale", "unavailable"))
    }
}

internal fun agentStopMessage(machine: MachineLabel, result: AgentStopResult): String =
    "${machine.text}: agent ${result.agent}; terminal ${result.terminal}."

internal fun agentInterruptMessage(machine: MachineLabel, result: AgentInterruptResult): String = when (result.outcome) {
    "written" -> "${machine.text}: interrupt key sent; stopping is not yet confirmed."
    "unknown" -> "${machine.text}: interrupt outcome unknown."
    else -> "${machine.text}: ${result.outcome}."
}
