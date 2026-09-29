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
    @SerialName("native") Native,
    @SerialName("unavailable") Unavailable,
}
@Serializable internal data class AgentStatus(val state: AgentState, val source: AgentMethod) {
    init { require(source == AgentMethod.Native || source == AgentMethod.Unavailable && state == AgentState.Unknown) }
}
@Serializable internal data class AgentMethods(
    val read: AgentMethod, val sendPeer: AgentMethod, val sendUser: AgentMethod,
    val queueUser: AgentMethod, val stop: AgentMethod,
) {
    init { require(queueUser == AgentMethod.Unavailable) }
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
@Serializable internal data class AgentBinding(val conversation: Conversation)
@Serializable internal data class ConversationRuntime(
    val binding: AgentBinding, val status: AgentStatus, val methods: AgentMethods, val turn: AgentTurn? = null,
)
@Serializable internal data class ConversationObservation(
    val binding: AgentBinding, val status: AgentStatus, val turn: AgentTurn? = null,
)
@Serializable internal data class ConversationOutput(
    val text: String, val source: AgentMethod, val scope: String, val truncated: Boolean,
    val observation: ConversationObservation, val outputState: String,
    val outputId: String? = null, val outputTurnId: String? = null,
)
internal fun decodeConversationOutput(encoded: String): ConversationOutput = decodeProtocol {
    productJson.decodeFromJsonElement<ConversationOutput>(nativeJsonObject(encoded)).also {
        require(it.source == AgentMethod.Native && it.scope in setOf("latest", "history"))
        require(it.text.utf8ByteCountWithin(32 * 1024) != null)
        require(it.outputState in setOf("partial", "finalized", "unknown", "none"))
        require(it.outputId == null || validNativeId(it.outputId))
        require(it.outputTurnId == null || validNativeId(it.outputTurnId))
        require(it.outputState != "finalized" || it.outputId != null)
        require(it.outputState != "none" || (it.scope != "latest" || it.text.isEmpty()) && it.outputId == null && it.outputTurnId == null)
    }
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
            for (field in listOf("pid")) {
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

@Serializable private data class ConversationStopRequest(val conversation: Conversation, val turn: AgentTurn? = null)
@Serializable private data class ConversationCloseRequest(
    val identityToken: String, val method: AgentMethod, val conversation: Conversation, val turn: AgentTurn? = null,
)
@Serializable private data class ConversationReadRequest(val conversation: Conversation, val scope: String, val maxBytes: Int)
@Serializable private data class ConversationAssociationRequest(val identityToken: String, val conversation: Conversation? = null)
internal fun encodeConversationStopRequest(runtime: ConversationRuntime): String =
    productJson.encodeToString(ConversationStopRequest.serializer(), ConversationStopRequest(runtime.binding.conversation, runtime.turn?.takeIf { it.state == "inProgress" }))
internal fun encodeAgentControlRequest(target: SessionTarget): String {
    val runtime = requireNotNull(target.session.conversation)
    require(runtime.methods.stop == AgentMethod.Native)
    return productJson.encodeToString(ConversationCloseRequest.serializer(), ConversationCloseRequest(
        target.session.identityToken, AgentMethod.Native, runtime.binding.conversation, runtime.turn?.takeIf { it.state == "inProgress" },
    ))
}
internal fun encodeConversationReadRequest(conversation: Conversation): String =
    productJson.encodeToString(ConversationReadRequest.serializer(), ConversationReadRequest(conversation, "latest", 16 * 1024))
internal fun encodeConversationAssociationRequest(target: SessionTarget, conversation: Conversation?): String =
    productJson.encodeToString(ConversationAssociationRequest.serializer(), ConversationAssociationRequest(target.session.identityToken, conversation))

@Serializable internal data class AgentStopResult(val method: AgentMethod, val outcome: String)
@Serializable internal data class AgentCloseResult(val agent: String, val terminal: String, val reason: String? = null)
@Serializable internal data class AgentResultPage(val conversation: Conversation, val resultIds: List<String>, val nextCursor: String? = null)
@Serializable private data class AgentResultsRequest(val conversation: Conversation, val cursor: String? = null)
internal fun encodeAgentResultsRequest(conversation: Conversation, cursor: String?): String =
    productJson.encodeToString(AgentResultsRequest.serializer(), AgentResultsRequest(conversation, cursor))

internal fun decodeAgentResultPage(encoded: String): AgentResultPage = decodeProtocol {
    productJson.decodeFromJsonElement<AgentResultPage>(nativeJsonObject(encoded)).also {
        require(it.resultIds.size <= 128 && it.resultIds.distinct().size == it.resultIds.size)
        require(it.resultIds.all(::validNativeId))
        require(it.nextCursor == null || it.nextCursor.isNotEmpty() && it.nextCursor.utf8ByteCountWithin(4096) != null)
    }
}
internal fun decodeAgentStopResult(encoded: String): AgentStopResult = decodeProtocol {
    productJson.decodeFromJsonElement<AgentStopResult>(nativeJsonObject(encoded)).also {
        require(it.method == AgentMethod.Native)
        require(it.outcome in setOf("interrupted", "stopped", "finished", "unknown"))
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
    result.outcome == "unknown" -> "could not confirm the request. inspect the conversation before trying again."
    result.outcome == "interrupted" -> "current work interrupted. pending input may remain."
    result.outcome == "stopped" -> "current work stopped. pending input may remain."
    result.outcome == "finished" -> "no active work observed. pending input may remain."
    else -> error("unrecognized stop outcome") // justify-defect: the decoder owns the closed outcome set.
}
internal fun agentCloseMessage(result: AgentCloseResult): String = when {
    result.reason == "stale" -> "terminal left open because the session changed."
    result.terminal == "closed" && result.agent == "unconfirmed" -> "terminal closed; conversation stop unconfirmed."
    result.terminal == "unconfirmed" -> "could not confirm the request. inspect the conversation before trying again."
    else -> when (result.agent) {
        "finished" -> "no active work observed"
        "interrupted" -> "current work interrupted"
        "stopped" -> "current work stopped"
        else -> error("unrecognized close outcome") // justify-defect: the decoder owns the closed outcome set.
    } + "; terminal closed. pending input may remain. saved provider history is retained."
}
