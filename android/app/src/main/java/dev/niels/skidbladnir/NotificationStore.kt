package dev.niels.skidbladnir

import android.content.Context
import androidx.datastore.core.CorruptionException
import androidx.datastore.core.Serializer
import androidx.datastore.dataStore
import java.io.InputStream
import java.io.OutputStream
import java.net.URI
import java.util.Base64
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.SerializationException
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.decodeFromJsonElement

@Serializable internal data class NotificationKey(
    val machine: String, val tmuxId: String, val identityToken: String, val paneId: String,
) {
    init {
        require(MachineHandle.parse(machine) != null)
        require(tmuxId.matches(Regex("\\$[0-9]+")) && identityToken.isNotEmpty())
        require(identityToken.utf8ByteCountWithin(Int.MAX_VALUE) != null)
        require(paneId.matches(Regex("%[0-9]+")))
    }
    constructor(target: SessionTarget) : this(
        target.machineHandle.encoded, target.session.tmuxId, target.session.identityToken, target.session.activePaneId,
    )
    val slot: String get() = Base64.getUrlEncoder().withoutPadding().encodeToString(
        productJson.encodeToString(NotificationSlot(machine, tmuxId, identityToken)).encodeToByteArray(),
    )
}
@Serializable private data class NotificationSlot(val machine: String, val tmuxId: String, val identityToken: String)

@Serializable internal data class NotificationForeground(val provider: AgentProvider, val pid: Long, val startIdentity: String) {
    init { require(pid > 0 && startIdentity.isNotEmpty() && startIdentity.utf8ByteCountWithin(Int.MAX_VALUE) != null) }
    constructor(agent: AgentRuntime) : this(agent.provider, agent.pid, agent.startIdentity)
}
@Serializable internal enum class NotificationPhase {
    @SerialName("quiet") Quiet, @SerialName("armed") Armed, @SerialName("ready") Ready,
}
@Serializable internal enum class NotificationCauseKind {
    @SerialName("ready") Ready, @SerialName("action") Action,
}
@Serializable internal data class NotificationCause(
    val kind: NotificationCauseKind, val request: TerminalInteraction? = null, val notice: TerminalNotice? = null,
) {
    init {
        require(request == null || request in setOf(TerminalInteraction.Permission, TerminalInteraction.Question,
            TerminalInteraction.Setup, TerminalInteraction.Confirmation, TerminalInteraction.Input))
        require(notice == null || notice != TerminalNotice.None)
    }
    init { require(when (kind) {
        NotificationCauseKind.Ready -> request == null && notice == null
        NotificationCauseKind.Action -> request != null || notice != null
    }) }
    val text: String get() = when (kind) {
        NotificationCauseKind.Ready -> "ready"
        NotificationCauseKind.Action -> listOfNotNull(
            request?.let(::terminalRequestText), notice?.let(::terminalNoticeText),
        ).joinToString(" · ")
    }
}
@Serializable internal data class NotificationRecord(
    val key: NotificationKey, val foreground: NotificationForeground? = null, val phase: NotificationPhase,
    val readyGeneration: Long, val attentionEpisode: Long, val cause: NotificationCause? = null,
) {
    init {
        require(readyGeneration >= 0 && attentionEpisode >= 0)
        require(phase == NotificationPhase.Quiet || foreground != null)
        require(phase != NotificationPhase.Quiet || readyGeneration == 0L)
        require(phase != NotificationPhase.Ready || readyGeneration > 0)
        require(cause == null || attentionEpisode > 0 && foreground != null)
        require(cause?.kind != NotificationCauseKind.Ready || phase == NotificationPhase.Ready && attentionEpisode >= readyGeneration)
    }
}
@Serializable internal enum class NotificationAvailability {
    @SerialName("fresh") Fresh, @SerialName("gap") Gap,
}
@Serializable internal data class NotificationRow(
    val ref: String, val name: String, val terminalStatus: TerminalStatus,
    val attentionQualified: Boolean, val record: NotificationRecord,
) {
    init {
        require(name.utf8ByteCountWithin(Int.MAX_VALUE) != null)
        require(notificationReference(ref) == record.key)
        require(!attentionQualified || record.cause == null || terminalStatus.source == TerminalStatusSource.Terminal)
        if (attentionQualified && record.cause?.kind == NotificationCauseKind.Action) {
            val request = when (terminalStatus.interaction) {
                TerminalInteraction.Permission, TerminalInteraction.Question, TerminalInteraction.Confirmation,
                TerminalInteraction.Setup, TerminalInteraction.Input -> terminalStatus.interaction
                TerminalInteraction.None, TerminalInteraction.Menu, TerminalInteraction.Unknown -> null
            }
            val notice = when (terminalStatus.notice) {
                TerminalNotice.None -> null
                TerminalNotice.Error, TerminalNotice.Interrupted -> terminalStatus.notice
            }
            require(record.cause.request == request && record.cause.notice == notice)
        }
        require(!attentionQualified || record.cause?.kind != NotificationCauseKind.Ready ||
            terminalStatus.activity == TerminalActivity.Idle && terminalStatus.interaction == TerminalInteraction.None && terminalStatus.notice == TerminalNotice.None)
    }
    val readyQualified: Boolean get() = attentionQualified && record.phase == NotificationPhase.Ready &&
        record.foreground != null && terminalStatus.activity == TerminalActivity.Idle &&
        terminalStatus.interaction == TerminalInteraction.None && terminalStatus.source == TerminalStatusSource.Terminal
}
@Serializable internal data class NotificationMachine(
    val machine: String, val availability: NotificationAvailability,
    val observedAt: String? = null, val sessions: List<NotificationRow>? = null,
) {
    init {
        require(MachineHandle.parse(machine) != null)
        require(when (availability) {
            NotificationAvailability.Fresh -> observedAt != null && sessions != null
            NotificationAvailability.Gap -> observedAt == null && sessions == null
        })
        observedAt?.let(::acceptWireInstant)
        sessions?.let { rows ->
            require(rows.all { it.record.key.machine == machine })
            require(rows.map { it.record.key.slot }.distinct().size == rows.size)
        }
    }
}
@Serializable internal data class ObserverSnapshot(
    val schema: Int, val epoch: String, val revision: Long, val receiverTag: String,
    val machines: List<NotificationMachine>,
) {
    init {
        require(schema == 1 && epoch.matches(Regex("[0-9a-f]{32}")) && revision in 0 until Long.MAX_VALUE)
        require(receiverTag.matches(Regex("$epoch:(0|[1-9][0-9]*)")))
        require(receiverTag.substringAfter(':').toLong() in 0..revision)
        require(machines.size == FLEET_LABELS.size && machines.distinctBy { it.machine }.size == machines.size)
        require(machines.flatMap { it.sessions.orEmpty() }.all {
            it.record.readyGeneration <= revision && it.record.attentionEpisode <= revision
        })
    }
}
@Serializable internal data class NotificationHint(val schema: Int, val epoch: String, val revision: Long) {
    init { require(schema == 1 && epoch.matches(Regex("[0-9a-f]{32}")) && revision in 0 until Long.MAX_VALUE) }
}
@Serializable internal data class NotificationConfig(val observerMachine: String, val ntfyOrigin: String) {
    init {
        require(MachineHandle.parse(observerMachine) != null)
        val uri = try { URI(ntfyOrigin) } catch (_: java.net.URISyntaxException) {
            throw IllegalArgumentException("invalid notification origin")
        }
        require(uri.scheme == "https" && uri.host != null && uri.port == 8444 && uri.rawUserInfo == null &&
            uri.rawPath.isEmpty() && uri.rawQuery == null && uri.rawFragment == null &&
            ntfyOrigin == "https://${uri.host.lowercase(java.util.Locale.ROOT)}:8444")
    }
    fun credential(fleet: List<MachineCredential>): MachineCredential? {
        if (fleet.size != FLEET_LABELS.size) return null
        val selected = fleet.singleOrNull { it.machine.handle.encoded == observerMachine } ?: return null
        return selected.takeIf { URI(it.machine.origin.encoded).host == URI(ntfyOrigin).host }
    }
}
@Serializable internal data class NotificationSubscriptionKeys(val p256dh: String, val auth: String) {
    init {
        val point = canonicalNotificationBase64(p256dh, 65)
        require(point[0] == 4.toByte())
        val parameters = java.security.AlgorithmParameters.getInstance("EC").apply {
            init(java.security.spec.ECGenParameterSpec("secp256r1"))
        }.getParameterSpec(java.security.spec.ECParameterSpec::class.java)
        val x = java.math.BigInteger(1, point.copyOfRange(1, 33))
        val y = java.math.BigInteger(1, point.copyOfRange(33, 65))
        val p = (parameters.curve.field as java.security.spec.ECFieldFp).p
        require(x < p && y < p && y.modPow(java.math.BigInteger.TWO, p) ==
            x.modPow(java.math.BigInteger.valueOf(3), p).add(parameters.curve.a.multiply(x)).add(parameters.curve.b).mod(p))
        canonicalNotificationBase64(auth, 16)
    }
}
@Serializable internal data class NotificationSubscription(val endpoint: String, val keys: NotificationSubscriptionKeys) {
    init {
        try { URI(endpoint) } catch (_: java.net.URISyntaxException) {
            throw IllegalArgumentException("invalid notification endpoint")
        }
    }
    fun validFor(config: NotificationConfig): Boolean {
        val uri = URI(endpoint)
        return uri.scheme == "https" && uri.host == URI(config.ntfyOrigin).host &&
            uri.port == 8444 && uri.rawUserInfo == null && uri.rawQuery == "up=1" && uri.rawFragment == null &&
            uri.rawPath.matches(Regex("/up[A-Za-z0-9_-]{1,62}")) && endpoint == config.ntfyOrigin + uri.rawPath + "?up=1"
    }
}
@Serializable internal data class NotificationRegistration(val generation: Long, val subscription: NotificationSubscription) {
    init { require(generation in 1 until Long.MAX_VALUE) }
}
@Serializable internal enum class NotificationDelivery {
    @SerialName("claimed") Claimed, @SerialName("posted") Posted, @SerialName("closed") Closed,
}
@Serializable internal data class DeviceNotificationRecord(
    val key: NotificationKey, val foreground: NotificationForeground? = null,
    val receivedReadyGeneration: Long, val acknowledgedReadyGeneration: Long,
    val handledAttentionEpisode: Long, val presentation: NotificationDelivery,
) {
    init {
        require(receivedReadyGeneration >= 0 && acknowledgedReadyGeneration in 0..receivedReadyGeneration && handledAttentionEpisode >= 0)
        require(foreground != null || receivedReadyGeneration == 0L && handledAttentionEpisode == 0L)
        require(presentation == NotificationDelivery.Closed || handledAttentionEpisode > 0)
    }
}
@Serializable internal data class DeviceNotifications(
    val schema: Int = 3, val observerEpoch: String? = null,
    val admittedRevision: Long = 0, val localRevision: Long = 0,
    val config: NotificationConfig? = null, val desiredRegistration: NotificationRegistration? = null,
    val records: List<DeviceNotificationRecord> = emptyList(),
) {
    init {
        require(schema == 3 && admittedRevision in 0 until Long.MAX_VALUE && localRevision in 0 until Long.MAX_VALUE)
        require(observerEpoch == null || observerEpoch.matches(Regex("[0-9a-f]{32}")))
        require(observerEpoch != null || admittedRevision == 0L && records.isEmpty())
        require(records.distinctBy { it.key.slot }.size == records.size)
        require(records.all { it.receivedReadyGeneration <= admittedRevision && it.handledAttentionEpisode <= admittedRevision })
        require(desiredRegistration == null || config != null && desiredRegistration.subscription.validFor(config))
    }
}
internal data class NotificationPresentation(val ready: Boolean = false, val unavailable: Boolean = false)

internal fun notificationReference(encoded: String): NotificationKey {
    require(encoded.length in 1..4_096)
    val decoded = Base64.getUrlDecoder().decode(encoded)
    require(Base64.getUrlEncoder().withoutPadding().encodeToString(decoded) == encoded)
    return productJson.decodeFromJsonElement<NotificationKey>(nativeJsonObject(decoded.decodeToString(throwOnInvalidSequence = true)))
}
private fun canonicalNotificationBase64(encoded: String, size: Int): ByteArray {
    val decoded = Base64.getUrlDecoder().decode(encoded)
    require(decoded.size == size && Base64.getUrlEncoder().withoutPadding().encodeToString(decoded) == encoded)
    return decoded
}

/** Counter spelling is checked before kotlinx can accept floating-point coercion. */
internal fun notificationJson(encoded: String): JsonObject = nativeJsonObject(encoded).also { root ->
    fun inspect(value: JsonElement) {
        when (value) {
            is JsonObject -> {
                value.forEach { (key, item) ->
                    if (key in setOf("schema", "revision", "admittedRevision", "localRevision", "generation", "pid",
                            "readyGeneration", "attentionEpisode", "receivedReadyGeneration", "acknowledgedReadyGeneration", "handledAttentionEpisode")) {
                        require(item is JsonPrimitive && !item.isString && item.content.matches(Regex("0|[1-9][0-9]*")))
                    }
                    inspect(item)
                }
            }
            is JsonArray -> value.forEach(::inspect)
            is JsonPrimitive -> Unit
            kotlinx.serialization.json.JsonNull -> error("nativeJsonObject accepted null")
        }
    }
    inspect(root)
}
private object NotificationSerializer : Serializer<DeviceNotifications> {
    private val codec = Json(productJson) { encodeDefaults = true }
    override val defaultValue = DeviceNotifications()
    override suspend fun readFrom(input: InputStream): DeviceNotifications = try {
        val bytes = input.readNBytes(1_048_577)
        require(bytes.size <= 1_048_576)
        val value = notificationJson(bytes.decodeToString(throwOnInvalidSequence = true))
        require(value.keys.containsAll(setOf("schema", "admittedRevision", "localRevision", "records")))
        productJson.decodeFromJsonElement(value)
    } catch (failure: SerializationException) {
        throw CorruptionException("invalid notification store", failure)
    } catch (failure: IllegalArgumentException) {
        throw CorruptionException("invalid notification store", failure)
    }
    override suspend fun writeTo(t: DeviceNotifications, output: OutputStream) {
        output.write(codec.encodeToString(t).encodeToByteArray())
    }
}
private val Context.notificationDataStore by dataStore(fileName = "notifications-v3.json", serializer = NotificationSerializer)

/** Application owner is the only caller; DataStore owns durable atomic replacement. */
internal class NotificationStore(context: Context) {
    private val dataStore = context.applicationContext.notificationDataStore
    suspend fun read(): DeviceNotifications = dataStore.updateData { it }
    suspend fun write(value: DeviceNotifications): DeviceNotifications = dataStore.updateData { previous ->
        check(value.localRevision > previous.localRevision)
        value
    }
}
