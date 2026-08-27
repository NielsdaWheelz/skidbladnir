package dev.niels.skidbladnir

import kotlinx.serialization.SerializationException
import kotlinx.serialization.decodeFromString
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.jsonObject

private const val MAXIMUM_PROTOCOL_JSON_DEPTH = 12

/** Rejects duplicate semantic keys and trailing documents before kotlinx decodes last-wins JSON. */
internal fun strictJsonObject(encoded: String): JsonObject {
    if (!UniqueJsonObjectKeyScanner(encoded).accepts()) {
        throw SerializationException("JSON must be one bounded document with unique object keys")
    }
    return productJson.parseToJsonElement(encoded).jsonObject
}

private class UniqueJsonObjectKeyScanner(private val encoded: String) {
    private var offset = 0

    fun accepts(): Boolean {
        skipWhitespace()
        if (!scanValue(depth = 0)) return false
        skipWhitespace()
        return offset == encoded.length
    }

    private fun scanValue(depth: Int): Boolean {
        if (depth > MAXIMUM_PROTOCOL_JSON_DEPTH || offset >= encoded.length) return false
        return when (encoded[offset]) {
            '{' -> scanObject(depth)
            '[' -> scanArray(depth)
            '"' -> scanString() != null
            't' -> scanLiteral("true")
            'f' -> scanLiteral("false")
            'n' -> scanLiteral("null")
            else -> scanNumber()
        }
    }

    private fun scanObject(depth: Int): Boolean {
        offset++
        skipWhitespace()
        if (consume('}')) return true
        val keys = mutableSetOf<String>()
        while (true) {
            skipWhitespace()
            val encodedKey = scanString() ?: return false
            val key = productJson.decodeFromString<String>(encodedKey)
            if (!keys.add(key)) return false
            skipWhitespace()
            if (!consume(':')) return false
            skipWhitespace()
            if (!scanValue(depth + 1)) return false
            skipWhitespace()
            if (consume('}')) return true
            if (!consume(',')) return false
        }
    }

    private fun scanArray(depth: Int): Boolean {
        offset++
        skipWhitespace()
        if (consume(']')) return true
        while (true) {
            if (!scanValue(depth + 1)) return false
            skipWhitespace()
            if (consume(']')) return true
            if (!consume(',')) return false
            skipWhitespace()
        }
    }

    private fun scanString(): String? {
        if (!consume('"')) return null
        val start = offset - 1
        while (offset < encoded.length) {
            when (val character = encoded[offset++]) {
                '"' -> return encoded.substring(start, offset)
                '\\' -> if (!scanEscape()) return null
                else -> if (character < ' ') return null
            }
        }
        return null
    }

    private fun scanEscape(): Boolean {
        if (offset >= encoded.length) return false
        return when (encoded[offset++]) {
            '"', '\\', '/', 'b', 'f', 'n', 'r', 't' -> true
            'u' -> scanUnicodeEscape()
            else -> false
        }
    }

    private fun scanUnicodeEscape(): Boolean {
        if (offset + 4 > encoded.length) return false
        repeat(4) {
            val digit = encoded[offset++]
            if (digit !in '0'..'9' && digit !in 'a'..'f' && digit !in 'A'..'F') return false
        }
        return true
    }

    private fun scanLiteral(literal: String): Boolean {
        if (!encoded.startsWith(literal, offset)) return false
        offset += literal.length
        return true
    }

    private fun scanNumber(): Boolean {
        val start = offset
        consume('-')
        if (consume('0')) {
            if (offset < encoded.length && encoded[offset].isDigit()) return false
        } else {
            if (offset >= encoded.length || encoded[offset] !in '1'..'9') return false
            while (offset < encoded.length && encoded[offset].isDigit()) offset++
        }
        if (consume('.')) {
            if (offset >= encoded.length || !encoded[offset].isDigit()) return false
            while (offset < encoded.length && encoded[offset].isDigit()) offset++
        }
        if (offset < encoded.length && encoded[offset] in charArrayOf('e', 'E')) {
            offset++
            if (offset < encoded.length && encoded[offset] in charArrayOf('+', '-')) offset++
            if (offset >= encoded.length || !encoded[offset].isDigit()) return false
            while (offset < encoded.length && encoded[offset].isDigit()) offset++
        }
        return offset > start
    }

    private fun skipWhitespace() {
        while (offset < encoded.length && encoded[offset] in charArrayOf(' ', '\t', '\r', '\n')) offset++
    }

    private fun consume(expected: Char): Boolean {
        if (offset >= encoded.length || encoded[offset] != expected) return false
        offset++
        return true
    }
}
