package dev.niels.skidbladnir

import java.util.Base64
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.decodeFromString
import kotlinx.serialization.json.Json

/** GET /v1/notifications/state: the hub's verdict per session lifetime. The hub owns every invariant. */
@Serializable internal data class HubNotifications(val machines: List<NotificationMachine>, val receiver: ReceiverStatus? = null)
@Serializable internal data class NotificationMachine(
    val machine: String, val availability: NotificationAvailability, val sessions: List<NotificationRow> = emptyList(),
)
@Serializable internal enum class NotificationAvailability { @SerialName("fresh") Fresh, @SerialName("gap") Gap }
/** Title, body and episode are set unless delivery is Clear; readyGeneration and foreground only while ready is pending. */
@Serializable internal data class NotificationRow(
    val slot: String, val ref: String, val title: String = "", val body: String = "", val episode: Long = 0,
    val delivery: NotificationDelivery, val readyGeneration: Long = 0, val foreground: NotificationForeground? = null,
)
@Serializable internal enum class NotificationDelivery { @SerialName("post") Post, @SerialName("keep") Keep, @SerialName("clear") Clear }
/** disabled | unconfigured | active | unavailable; sent only to phones. */
@Serializable internal data class ReceiverStatus(val status: String)

/** The exact pane of one session lifetime; a ref is base64url(json(key)). */
@Serializable internal data class NotificationKey(val machine: String, val tmuxId: String, val identityToken: String, val paneId: String) {
    constructor(target: SessionTarget) : this(
        target.machineHandle.encoded, target.session.tmuxId, target.session.identityToken, target.session.activePaneId,
    )
}
@Serializable internal data class NotificationForeground(val provider: AgentProvider, val pid: Long, val startIdentity: String) {
    constructor(agent: AgentRuntime) : this(agent.provider, agent.pid, agent.startIdentity)
}

private val referenceJson = Json { ignoreUnknownKeys = true }

/** Null for anything that is not a key: a click intent can come from any app. */
internal fun notificationReference(encoded: String): NotificationKey? = try {
    referenceJson.decodeFromString<NotificationKey>(Base64.getUrlDecoder().decode(encoded).decodeToString())
} catch (_: IllegalArgumentException) { null } // includes SerializationException

/** One native notice as the os shows it; slot is its tag. */
internal data class NotificationNotice(
    val slot: String, val machine: String, val ref: String, val title: String, val body: String, val episode: Long,
)
/** cancel removes the slot; otherwise a post, audible or a silent replacement. */
internal data class NotificationEffect(val notice: NotificationNotice, val cancel: Boolean, val audible: Boolean)

/**
 * The hub's verdicts against what the os shows. alerted is this process's audible posts (slot to episode), so an
 * episode alerts once per process and a swiped notice is not re-posted. Effects are applied in order.
 */
internal fun reconcileNotifications(
    state: HubNotifications, delivered: List<NotificationNotice>, alerted: Map<String, Long>, allowed: Boolean,
): List<NotificationEffect> {
    val effects = mutableListOf<NotificationEffect>()
    val copies = delivered.associateBy { it.slot }
    val rows = mutableSetOf<String>()
    for (machine in state.machines) {
        if (machine.availability != NotificationAvailability.Fresh) continue
        for (row in machine.sessions) {
            rows += row.slot
            val want = NotificationNotice(row.slot, machine.machine, row.ref, row.title, row.body, row.episode)
            val copy = copies[row.slot]
            when (row.delivery) {
                // alerted covers a post the os has not surfaced yet.
                NotificationDelivery.Clear -> if (copy != null || row.slot in alerted) effects += NotificationEffect(want, cancel = true, audible = false)
                // Unverified attention: never post, never remove; only retitle the copy it already shows.
                NotificationDelivery.Keep -> if (copy != null && copy.episode == row.episode && copy != want) {
                    effects += NotificationEffect(want, cancel = false, audible = false)
                }
                NotificationDelivery.Post -> when {
                    copy != null && copy.episode == row.episode -> if (copy != want) effects += NotificationEffect(want, cancel = false, audible = false)
                    alerted[row.slot] == row.episode -> Unit
                    allowed -> effects += NotificationEffect(want, cancel = false, audible = true)
                    // Superseded while blocked: the new episode posts once permission allows.
                    copy != null -> effects += NotificationEffect(copy, cancel = true, audible = false)
                }
            }
        }
    }
    // Vanished lifetimes and removed machines; a gap machine keeps its notices.
    val gaps = state.machines.filter { it.availability == NotificationAvailability.Gap }.map { it.machine }.toSet()
    for (copy in delivered) if (copy.slot !in rows && copy.machine !in gaps) effects += NotificationEffect(copy, cancel = true, audible = false)
    return effects
}
