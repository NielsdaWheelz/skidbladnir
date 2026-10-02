package dev.niels.skidbladnir

import android.content.Context
import androidx.datastore.core.CorruptionException
import androidx.datastore.core.DataStore
import androidx.datastore.core.Serializer
import androidx.datastore.dataStore
import java.io.IOException
import java.io.InputStream
import java.io.OutputStream
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.joinAll
import kotlinx.coroutines.launch
import kotlinx.serialization.Serializable
import kotlinx.serialization.SerializationException
import kotlinx.serialization.SerialName
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.decodeFromJsonElement
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject

@Serializable internal data class NotificationKey(
    val machine: String, val tmuxId: String, val identityToken: String, val paneId: String,
) {
    init {
        require(MachineHandle.parse(machine) != null)
        require(tmuxId.matches(Regex("\\$[0-9]+")) && identityToken.isNotEmpty())
        require(paneId.matches(Regex("%[0-9]+")))
    }
    constructor(target: SessionTarget) : this(
        target.machineHandle.encoded, target.session.tmuxId, target.session.identityToken, target.session.activePaneId,
    )
}
@Serializable internal data class NotificationForeground(val provider: AgentProvider, val pid: Long, val startIdentity: String) {
    init { require(pid > 0 && startIdentity.isNotEmpty()) }
    constructor(agent: AgentRuntime) : this(agent.provider, agent.pid, agent.startIdentity)
}
@Serializable internal enum class NotificationState {
    @SerialName("quiet") Quiet, @SerialName("armed") Armed, @SerialName("ready") Ready,
}
@Serializable internal data class NotificationRecord(
    val key: NotificationKey, val revision: Long, val foreground: NotificationForeground? = null,
    val state: NotificationState,
) {
    init {
        require(revision >= 0)
        require(state == NotificationState.Quiet || foreground != null)
    }
}
@Serializable internal data class NotificationSnapshot(val schema: Int, val terminals: List<NotificationRecord>) {
    init {
        require(schema == 2)
        require(terminals.distinctBy { it.key }.size == terminals.size)
    }
    fun record(key: NotificationKey): NotificationRecord? = terminals.singleOrNull { it.key == key }

    /** Committed attention is visible only on a qualified idle of its exact local foreground. */
    fun presentsReady(key: NotificationKey, session: TmuxSession): Boolean {
        val saved = record(key) ?: return false
        return saved.state == NotificationState.Ready && session.connection == null &&
            session.terminalStatus.source == TerminalStatusSource.Terminal &&
            saved.foreground == session.agent?.let(::NotificationForeground) &&
            readyObservation(session.terminalStatus) == ReadyObservation.Idle
    }
}

/** The caller supplies pre-dispatch revisions; all effects are committed in one DataStore update. */
private fun NotificationSnapshot.reduce(
    machine: MachineHandle, sessions: List<TmuxSession>, expected: NotificationSnapshot,
    visiting: NotificationKey?, authoritative: Boolean,
): NotificationSnapshot {
    val records = terminals.associateBy { it.key }.toMutableMap()
    if (authoritative) for (old in terminals.filter { it.key.machine == machine.encoded }) {
        val session = sessions.singleOrNull { it.tmuxId == old.key.tmuxId && it.identityToken == old.key.identityToken }
        if (session == null) {
            if (expected.record(old.key)?.revision == old.revision) records.remove(old.key)
        }
    }
    for (session in sessions) {
        val key = NotificationKey(SessionTarget(machine, session))
        val old = records[key]
        if (old?.revision != expected.record(key)?.revision) continue
        val unselectedPanes = (records.keys + expected.terminals.map { it.key }).filter {
            it.machine == key.machine && it.tmuxId == key.tmuxId && it.identityToken == key.identityToken && it.paneId != key.paneId
        }
        // Pane selection is one observation: a stale sibling cannot be reset piecemeal.
        if (unselectedPanes.any { expected.record(it)?.revision != records[it]?.revision }) continue
        for (unselectedKey in unselectedPanes) {
            val unselected = records.getValue(unselectedKey)
            records[unselected.key] = unselected.copy(
                revision = notificationRevision(unselected), foreground = null, state = NotificationState.Quiet,
            )
        }
        val foreground = session.agent?.takeIf { session.connection == null }?.let(::NotificationForeground)
        val positiveExit = session.connection != null || foreground == null && session.terminalStatus.source == TerminalStatusSource.Terminal
        val replaced = foreground != null && old?.foreground != foreground || positiveExit
        val revision = notificationRevision(old)
        var state = if (replaced) NotificationState.Quiet else old?.state ?: NotificationState.Quiet
        if (foreground != null && session.terminalStatus.source == TerminalStatusSource.Terminal) when (readyObservation(session.terminalStatus)) {
            ReadyObservation.NonIdle -> state = NotificationState.Armed
            ReadyObservation.Idle -> state = when {
                key == visiting -> NotificationState.Quiet
                state == NotificationState.Armed -> NotificationState.Ready
                else -> state
            }
            ReadyObservation.Gap -> Unit
        }
        records[key] = NotificationRecord(
            key, revision, foreground ?: old?.foreground?.takeUnless { positiveExit }, state,
        )
    }
    return copy(terminals = records.values.toList())
}

private fun NotificationSnapshot.presented(key: NotificationKey): NotificationSnapshot {
    val old = record(key)
    val state = when (old?.state) {
        NotificationState.Armed -> NotificationState.Armed
        NotificationState.Ready, NotificationState.Quiet, null -> NotificationState.Quiet
    }
    val record = NotificationRecord(key, notificationRevision(old), old?.foreground, state)
    return copy(terminals = terminals.filterNot { it.key == key } + record)
}

private fun NotificationSnapshot.endVisit(key: NotificationKey): NotificationSnapshot {
    val old = record(key)
    val record = NotificationRecord(key, notificationRevision(old), old?.foreground, old?.state ?: NotificationState.Quiet)
    return copy(terminals = terminals.filterNot { it.key == key } + record)
}

internal data class NotificationPresentation(val ready: Boolean = false, val unavailable: Boolean = false)

private enum class ReadyObservation { NonIdle, Idle, Gap }

private fun readyObservation(status: TerminalStatus): ReadyObservation {
    val interactionNone = when (status.interaction) {
        TerminalInteraction.None -> true
        TerminalInteraction.Unknown -> false
        TerminalInteraction.Permission, TerminalInteraction.Question, TerminalInteraction.Confirmation,
        TerminalInteraction.Setup, TerminalInteraction.Input, TerminalInteraction.Menu -> return ReadyObservation.NonIdle
    }
    return when (status.activity) {
        TerminalActivity.Starting, TerminalActivity.Working -> ReadyObservation.NonIdle
        TerminalActivity.Idle -> if (interactionNone) ReadyObservation.Idle else ReadyObservation.Gap
        TerminalActivity.Unknown -> ReadyObservation.Gap
    }
}

private fun notificationRevision(record: NotificationRecord?): Long {
    if (record?.revision == Long.MAX_VALUE) throw IOException("notification revision exhausted")
    return record?.revision?.plus(1) ?: 0
}

private object NotificationSerializer : Serializer<NotificationSnapshot> {
    override val defaultValue = NotificationSnapshot(2, emptyList())
    override suspend fun readFrom(input: InputStream): NotificationSnapshot = try {
        val value = nativeJsonObject(input.readBytes().decodeToString(throwOnInvalidSequence = true))
        val schema = value["schema"]
        require(schema is JsonPrimitive && !schema.isString && schema.content == "2")
        value.getValue("terminals").jsonArray.forEach { entry ->
            val revision = entry.jsonObject["revision"]
            require(revision is JsonPrimitive && !revision.isString && revision.content.matches(Regex("[0-9]+")))
        }
        productJson.decodeFromJsonElement<NotificationSnapshot>(value)
    } catch (failure: SerializationException) {
        throw CorruptionException("invalid notification store", failure)
    } catch (failure: NoSuchElementException) {
        throw CorruptionException("invalid notification store", failure)
    } catch (failure: IllegalArgumentException) {
        throw CorruptionException("invalid notification store", failure)
    }
    override suspend fun writeTo(value: NotificationSnapshot, output: OutputStream) {
        output.write(productJson.encodeToString(value).encodeToByteArray())
    }
}
private val Context.notificationDataStore: DataStore<NotificationSnapshot> by dataStore(
    fileName = "notifications-v2.json", serializer = NotificationSerializer,
)

/** DataStore owns serialization; reads join the same queue so dispatch follows prior visit commits. */
internal class NotificationStore(context: Context) {
    private val dataStore = context.applicationContext.notificationDataStore
    private val scope = CoroutineScope(Dispatchers.Main.immediate + SupervisorJob())

    suspend fun read(): NotificationSnapshot = dataStore.updateData { it }

    fun read(onReady: (NotificationSnapshot) -> Unit, onUnavailable: () -> Unit) =
        update(onReady, onUnavailable) { it }

    fun retainMachines(machines: Set<MachineHandle>, onReady: (NotificationSnapshot) -> Unit, onUnavailable: () -> Unit) =
        update(onReady, onUnavailable) { value ->
            value.copy(terminals = value.terminals.filter { record -> machines.any { it.encoded == record.key.machine } })
        }

    fun observe(
        machine: MachineHandle, sessions: List<TmuxSession>, expected: NotificationSnapshot,
        visiting: NotificationKey?, valid: () -> Boolean,
        onReady: (NotificationSnapshot) -> Unit, onUnavailable: () -> Unit,
    ) = update(onReady, onUnavailable) { value ->
        if (valid()) value.reduce(machine, sessions, expected, visiting, authoritative = true) else value
    }

    fun presented(key: NotificationKey, onReady: (NotificationSnapshot) -> Unit, onUnavailable: () -> Unit) =
        update(onReady, onUnavailable) { it.presented(key) }

    fun endVisit(key: NotificationKey, onReady: (NotificationSnapshot) -> Unit, onUnavailable: () -> Unit) =
        update(onReady, onUnavailable) { it.endVisit(key) }

    fun observeSession(
        machine: MachineHandle, session: TmuxSession, expected: NotificationSnapshot,
        visiting: NotificationKey?, valid: () -> Boolean,
        onReady: (NotificationSnapshot) -> Unit, onUnavailable: () -> Unit,
    ) = update(onReady, onUnavailable) { value ->
        if (valid()) value.reduce(machine, listOf(session), expected, visiting, authoritative = false) else value
    }

    private fun update(onReady: (NotificationSnapshot) -> Unit, onUnavailable: () -> Unit, merge: (NotificationSnapshot) -> NotificationSnapshot) {
        scope.launch {
            val value = try { dataStore.updateData { merge(it) } } catch (_: IOException) {
                // justify-ignore-error: corruption and I/O are explicitly exposed without resetting the store.
                onUnavailable()
                return@launch
            }
            onReady(value)
        }
    }

    fun close() {
        val pending = checkNotNull(scope.coroutineContext[Job]).children.toList()
        scope.launch {
            pending.joinAll()
            scope.cancel()
        }
    }
}
