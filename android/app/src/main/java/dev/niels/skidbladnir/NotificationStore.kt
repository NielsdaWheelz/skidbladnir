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
@Serializable internal data class NotificationRecord(
    val key: NotificationKey, val revision: Long, val foreground: NotificationForeground? = null,
    val pending: Boolean, val baselinePending: Boolean,
) {
    init {
        require(revision >= 0)
        require(!pending || foreground != null && !baselinePending)
    }
}
@Serializable internal data class NotificationSnapshot(val schema: Int, val terminals: List<NotificationRecord>) {
    init {
        require(schema == 1)
        require(terminals.distinctBy { it.key }.size == terminals.size)
    }
    fun record(key: NotificationKey): NotificationRecord? = terminals.singleOrNull { it.key == key }

    /** The caller supplies pre-dispatch revisions; all effects are committed in one DataStore update. */
    fun observe(
        machine: MachineHandle, sessions: List<TmuxSession>, expected: NotificationSnapshot,
        predecessors: Map<NotificationKey, NotificationPredecessor>, visiting: NotificationKey?,
    ): NotificationUpdate {
        val records = terminals.associateBy { it.key }.toMutableMap()
        val next = mutableMapOf<NotificationKey, NotificationPredecessor>()
        for (old in terminals.filter { it.key.machine == machine.encoded }) {
            val session = sessions.singleOrNull { it.tmuxId == old.key.tmuxId && it.identityToken == old.key.identityToken }
            if (session == null) {
                if (expected.record(old.key)?.revision == old.revision) records.remove(old.key)
            }
        }
        for (session in sessions) {
            val key = NotificationKey(SessionTarget(machine, session))
            val old = records[key]
            if (old?.revision != expected.record(key)?.revision) continue
            for (unselected in records.values.toList().filter {
                it.key.machine == key.machine && it.key.tmuxId == key.tmuxId && it.key.identityToken == key.identityToken && it.key.paneId != key.paneId
            }) {
                if (expected.record(unselected.key)?.revision != unselected.revision) continue
                // A rejected selected sample has no selection authority. An admitted one proves
                // discontinuity, not deletion; retain revisions to reject late pane observations.
                records[unselected.key] = unselected.copy(
                    revision = notificationRevision(unselected), foreground = null, pending = false, baselinePending = false,
                )
            }
            val foreground = session.agent?.takeIf { session.connection == null }?.let(::NotificationForeground)
            val positiveExit = foreground == null && session.terminalStatus.source == TerminalStatusSource.Terminal
            val replaced = foreground != null && old?.foreground != foreground || positiveExit
            val qualified = foreground != null && session.terminalStatus.source == TerminalStatusSource.Terminal &&
                session.terminalStatus.state != TerminalState.Unknown
            val revision = notificationRevision(old)
            val prior = predecessors[key]?.takeIf { it.foreground == foreground && it.revision == old?.revision }
            var pending = old?.pending == true
            var baselinePending = old?.baselinePending == true
            if (replaced || key == visiting) pending = false
            if (positiveExit) baselinePending = false
            if (qualified) {
                if (baselinePending) {
                    pending = false
                    baselinePending = false
                } else if (key != visiting && !replaced && session.terminalStatus.state == TerminalState.Idle && prior != null) {
                    pending = true
                }
                when (session.terminalStatus.state) {
                    TerminalState.Working -> {
                        pending = false
                        if (key != visiting) next[key] = NotificationPredecessor(checkNotNull(foreground), revision)
                    }
                    TerminalState.Blocked -> pending = false
                    TerminalState.Idle -> Unit
                    TerminalState.Unknown -> error("qualified unknown notification sample") // justify-defect: qualified excludes unknown.
                }
            }
            records[key] = NotificationRecord(
                key, revision, foreground ?: old?.foreground?.takeUnless { positiveExit }, pending, baselinePending,
            )
        }
        return NotificationUpdate(copy(terminals = records.values.toList()), next)
    }

    fun consume(key: NotificationKey, closing: Boolean): NotificationSnapshot {
        val old = record(key)
        val record = NotificationRecord(key, notificationRevision(old), old?.foreground, false, closing || old?.baselinePending == true)
        return copy(terminals = terminals.filterNot { it.key == key } + record)
    }
}
internal data class NotificationPredecessor(val foreground: NotificationForeground, val revision: Long)
internal data class NotificationUpdate(val snapshot: NotificationSnapshot, val predecessors: Map<NotificationKey, NotificationPredecessor>)
internal data class NotificationPresentation(val ready: Boolean = false, val unavailable: Boolean = false)

private fun notificationRevision(record: NotificationRecord?): Long {
    if (record?.revision == Long.MAX_VALUE) throw IOException("notification revision exhausted")
    return record?.revision?.plus(1) ?: 0
}

private object NotificationSerializer : Serializer<NotificationSnapshot> {
    override val defaultValue = NotificationSnapshot(1, emptyList())
    override suspend fun readFrom(input: InputStream): NotificationSnapshot = try {
        val value = nativeJsonObject(input.readBytes().decodeToString(throwOnInvalidSequence = true))
        val schema = value["schema"]
        require(schema is JsonPrimitive && !schema.isString && schema.content == "1")
        value.getValue("terminals").jsonArray.forEach { entry ->
            val revision = entry.jsonObject["revision"]
            require(revision is JsonPrimitive && !revision.isString && revision.content.matches(Regex("[0-9]+")))
            for (field in listOf("pending", "baselinePending")) {
                val flag = entry.jsonObject[field]
                require(flag is JsonPrimitive && !flag.isString && flag.content in setOf("true", "false"))
            }
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
    fileName = "notifications.json", serializer = NotificationSerializer,
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
        predecessors: () -> Map<NotificationKey, NotificationPredecessor>, visiting: NotificationKey?,
        valid: () -> Boolean, onReady: (NotificationUpdate) -> Unit, onUnavailable: () -> Unit,
    ) {
        scope.launch {
            var next = emptyMap<NotificationKey, NotificationPredecessor>()
            val committed = try {
                dataStore.updateData { value ->
                    if (!valid()) value else value.observe(machine, sessions, expected, predecessors(), visiting).also {
                        next = it.predecessors
                    }.snapshot
                }
            } catch (_: IOException) {
                // justify-ignore-error: failure is visible and never resets notification data or terminal transport.
                onUnavailable()
                return@launch
            }
            onReady(NotificationUpdate(committed, next))
        }
    }

    fun presented(key: NotificationKey, onReady: (NotificationSnapshot) -> Unit, onUnavailable: () -> Unit) =
        update(onReady, onUnavailable) { it.consume(key, closing = false) }

    fun endVisit(key: NotificationKey, onReady: (NotificationSnapshot) -> Unit, onUnavailable: () -> Unit) =
        update(onReady, onUnavailable) { it.consume(key, closing = true) }

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
