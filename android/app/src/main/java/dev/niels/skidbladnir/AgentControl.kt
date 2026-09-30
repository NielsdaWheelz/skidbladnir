package dev.niels.skidbladnir

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonNull
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.decodeFromJsonElement

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
