package dev.niels.skidbladnir

import kotlinx.serialization.json.Json

/** The one codec. Tolerant of keys it does not read (rules.md trust boundaries); null optionals encode as omitted. */
internal val productJson = Json { ignoreUnknownKeys = true; explicitNulls = false }
