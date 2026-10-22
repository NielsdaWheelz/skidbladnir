package dev.niels.skidbladnir

import java.time.DateTimeException
import kotlinx.serialization.SerializationException
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive

/** The one codec. Tolerant of keys it does not read (rules.md trust boundaries); null optionals encode as omitted. */
internal val productJson = Json { ignoreUnknownKeys = true; explicitNulls = false }

// Residue for TerminalConnection.kt only; deleted when the terminal-page cleanup replaces its decoder.
internal class ProtocolDecodeException(reason: String) :
    RuntimeException("Protocol payload could not be decoded: $reason.")

internal inline fun <Value> decodeProtocol(block: () -> Value): Value = try {
    block()
} catch (failure: IllegalArgumentException) { // includes SerializationException
    throw ProtocolDecodeException(failure.javaClass.simpleName)
} catch (failure: DateTimeException) {
    throw ProtocolDecodeException(failure.javaClass.simpleName)
} catch (failure: NoSuchElementException) {
    throw ProtocolDecodeException(failure.javaClass.simpleName)
}

internal fun strictJsonObject(encoded: String): JsonObject =
    productJson.parseToJsonElement(encoded) as? JsonObject
        ?: throw SerializationException("protocol payload is not a JSON object")

internal fun JsonObject.requiredObject(key: String): JsonObject =
    this[key] as? JsonObject ?: throw SerializationException("missing or non-object $key")

internal fun JsonObject.requiredString(key: String): String {
    val member = this[key]
    if (member !is JsonPrimitive || !member.isString) throw SerializationException("missing or non-string $key")
    return member.content
}

internal fun JsonObject.requireExactKeys(expected: Set<String>) {
    if (keys != expected) throw SerializationException("unexpected protocol fields")
}
