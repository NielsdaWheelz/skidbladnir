package dev.niels.skidbladnir

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonNull
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.decodeFromJsonElement

@Serializable internal enum class AgentState {
    @SerialName("working") Working, @SerialName("blocked") Blocked,
    @SerialName("idle") Idle, @SerialName("done") Done,
    @SerialName("failed") Failed, @SerialName("stopped") Stopped,
    @SerialName("unknown") Unknown,
}
@Serializable internal enum class AgentMethod {
    @SerialName("native") Native, @SerialName("terminal") Terminal,
    @SerialName("unavailable") Unavailable,
}
@Serializable internal data class AgentStatus(val state: AgentState, val source: AgentMethod) {
    init { require(source == AgentMethod.Native || source == AgentMethod.Unavailable && state == AgentState.Unknown) }
}
@Serializable internal data class AgentMethods(
    val read: AgentMethod, val sendPeer: AgentMethod, val sendUser: AgentMethod,
    val queueUser: AgentMethod, val stop: AgentMethod,
) {
    init { require(listOf(read, sendPeer, sendUser, queueUser).none { it == AgentMethod.Terminal }) }
}

@Serializable internal data class Conversation(
    val provider: AgentProvider, val profileKey: String, val historyScope: String,
    val conversationId: String,
) {
    init {
        require(ProfileKey.parse(profileKey) != null)
        require(historyScope.matches(Regex("[0-9a-f]{64}")))
        require(validNativeId(conversationId))
    }
}
@Serializable internal data class AgentView(val viewId: String, val revision: ULong) {
    init { require(viewId.matches(Regex("[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}"))) }
}
@Serializable internal data class AgentBinding(val conversation: Conversation, val view: AgentView? = null) {
    init { require((conversation.provider == AgentProvider.Codex) == (view != null)) }
}
@Serializable internal data class AgentTurn(val id: String, val state: String) {
    init { require(validNativeId(id) && state in setOf("inProgress", "completed", "failed", "interrupted")) }
}

internal fun validNativeId(value: String): Boolean = value.length in 1..128 && value.all { it.code in 0x21..0x7e }

/** Native records consistently omit absent values; null is never another encoding of absence. */
internal fun JsonElement.requireNativeValues() {
    when (this) {
        JsonNull -> throw kotlinx.serialization.SerializationException("native field is null")
        is JsonObject -> {
            for (field in listOf("pid", "revision")) {
                this[field]?.let { value ->
                    require(value is JsonPrimitive && !value.isString && value.content.matches(Regex("[0-9]+")))
                }
            }
            values.forEach { it.requireNativeValues() }
        }
        is JsonArray -> forEach { it.requireNativeValues() }
        is JsonPrimitive -> Unit
    }
}
internal fun nativeJsonObject(encoded: String): JsonObject = strictJsonObject(encoded).also { it.requireNativeValues() }

@Serializable private data class AgentControlRequest(
    val identityToken: String, val paneId: String, val pid: Long, val startIdentity: String,
    val method: AgentMethod, val binding: AgentBinding? = null, val turn: AgentTurn? = null,
)
internal fun encodeAgentControlRequest(target: SessionTarget): String {
    val agent = requireNotNull(target.session.agent)
    require(agent.methods.stop != AgentMethod.Unavailable)
    return productJson.encodeToString(AgentControlRequest.serializer(), AgentControlRequest(
        target.session.identityToken, agent.paneId, agent.pid, agent.startIdentity,
        agent.methods.stop, agent.binding.takeIf { agent.methods.stop == AgentMethod.Native },
        agent.turn.takeIf { agent.methods.stop == AgentMethod.Native },
    ))
}

@Serializable internal data class AgentStopResult(val method: AgentMethod, val outcome: String)
@Serializable internal data class AgentCloseResult(val agent: String, val terminal: String, val reason: String? = null)
@Serializable internal data class AgentResultPage(val conversation: Conversation, val resultIds: List<String>, val nextCursor: String? = null)
@Serializable private data class AgentResultsRequest(val identityToken: String, val conversation: Conversation, val cursor: String? = null)
internal fun encodeAgentResultsRequest(target: SessionTarget, conversation: Conversation, cursor: String?): String =
    productJson.encodeToString(AgentResultsRequest.serializer(), AgentResultsRequest(target.session.identityToken, conversation, cursor))

internal fun decodeAgentResultPage(encoded: String): AgentResultPage = decodeProtocol {
    productJson.decodeFromJsonElement<AgentResultPage>(nativeJsonObject(encoded)).also {
        require(it.resultIds.size <= 128 && it.resultIds.distinct().size == it.resultIds.size)
        require(it.resultIds.all(::validNativeId))
        require(it.nextCursor == null || it.nextCursor.isNotEmpty() && it.nextCursor.utf8ByteCountWithin(4096) != null)
    }
}
internal fun decodeAgentStopResult(encoded: String): AgentStopResult = decodeProtocol {
    productJson.decodeFromJsonElement<AgentStopResult>(nativeJsonObject(encoded)).also {
        require(when (it.method) {
            AgentMethod.Native -> it.outcome in setOf("interrupted", "stopped", "finished", "unknown")
            AgentMethod.Terminal -> it.outcome in setOf("written", "unknown")
            AgentMethod.Unavailable -> false
        })
    }
}
internal fun decodeAgentCloseResult(encoded: String): AgentCloseResult = decodeProtocol {
    productJson.decodeFromJsonElement<AgentCloseResult>(nativeJsonObject(encoded)).also {
        require(it.agent in setOf("stopped", "interrupted", "finished", "unconfirmed"))
        require(it.terminal in setOf("closed", "unconfirmed"))
        require(it.reason == null || it.reason in setOf("stale", "unavailable"))
    }
}
internal fun agentStopMessage(result: AgentStopResult): String = when {
    result.outcome == "unknown" -> "could not confirm the request. check the terminal before trying again."
    result.method == AgentMethod.Terminal -> "keys sent; agent state not confirmed."
    result.outcome == "interrupted" -> "current work interrupted. pending input may remain."
    result.outcome == "stopped" -> "current work stopped. pending input may remain."
    result.outcome == "finished" -> "no active work observed. pending input may remain."
    else -> error("unrecognized stop outcome") // justify-defect: the decoder owns the closed outcome set.
}
internal fun agentCloseMessage(result: AgentCloseResult): String = when {
    result.reason == "stale" -> "terminal left open because the session changed."
    result.terminal == "closed" && result.agent == "unconfirmed" -> "terminal closed; agent stop unconfirmed."
    result.terminal == "unconfirmed" -> "could not confirm the request. check the terminal before trying again."
    else -> when (result.agent) {
        "finished" -> "no active work observed"
        "interrupted" -> "current work interrupted"
        "stopped" -> "current work stopped"
        else -> error("unrecognized close outcome") // justify-defect: the decoder owns the closed outcome set.
    } + "; terminal closed. pending input may remain. saved provider history is retained."
}
