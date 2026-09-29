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
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch
import kotlinx.serialization.Serializable
import kotlinx.serialization.SerializationException
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.decodeFromJsonElement
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject

@Serializable internal data class ConversationKey(
    val machine: String, val provider: AgentProvider, val historyScope: String, val conversationId: String,
) {
    init {
        require(MachineHandle.parse(machine) != null)
        require(historyScope.matches(Regex("[0-9a-f]{64}")))
        require(validNativeId(conversationId))
    }
    constructor(machine: MachineHandle, conversation: Conversation) : this(
        machine.encoded, conversation.provider, conversation.historyScope, conversation.conversationId,
    )
}
@Serializable internal data class UnreadRecord(
    val key: ConversationKey, val acknowledgedIds: Set<String>, val unreadIds: Set<String>,
) {
    init {
        require(acknowledgedIds.all(::validNativeId) && unreadIds.all(::validNativeId))
        require(acknowledgedIds.intersect(unreadIds).isEmpty())
    }
}
@Serializable internal data class UnreadAssociation(
    val machine: String, val tmuxId: String, val identityToken: String, val paneId: String,
    val conversation: Conversation,
) {
    init {
        require(MachineHandle.parse(machine) != null && tmuxId.isNotEmpty() && identityToken.isNotEmpty())
        require(paneId.matches(Regex("%[0-9]+")))
    }
}
@Serializable internal data class UnreadSnapshot(
    val schema: Int, val conversations: List<UnreadRecord>, val associations: List<UnreadAssociation>,
) {
    init {
        require(schema == 1)
        require(conversations.distinctBy { it.key }.size == conversations.size)
        require(associations.distinctBy { Triple(it.machine, it.tmuxId, it.identityToken) }.size == associations.size)
    }
    fun record(key: ConversationKey): UnreadRecord? = conversations.singleOrNull { it.key == key }
    fun conversation(target: SessionTarget): Conversation? {
        val session = target.session
        session.conversation?.let { return it.binding.conversation }
        if (session.agent != null) return null
        if (session.connection != null) return null
        return associations.singleOrNull {
            it.machine == target.machineHandle.encoded && it.tmuxId == session.tmuxId &&
                it.identityToken == session.identityToken && it.paneId == session.activePaneId &&
                it.conversation.provider == AgentProvider.Claude
        }?.conversation
    }
}
internal data class UnreadCapture(val key: ConversationKey, val ids: Set<String>)
internal data class ReplyPresentation(
    val unread: Boolean = false, val previousAgent: Boolean = false,
    val repliesUnavailable: Boolean = false, val storeUnavailable: Boolean = false,
)

private object UnreadSerializer : Serializer<UnreadSnapshot> {
    override val defaultValue = UnreadSnapshot(1, emptyList(), emptyList())
    override suspend fun readFrom(input: InputStream): UnreadSnapshot = try {
        val value = nativeJsonObject(input.readBytes().decodeToString(throwOnInvalidSequence = true))
        val schema = value["schema"]
        require(schema is JsonPrimitive && !schema.isString && schema.content == "1")
        value.getValue("conversations").jsonArray.forEach { entry ->
            for (field in listOf("acknowledgedIds", "unreadIds")) {
                val ids = entry.jsonObject.getValue(field).jsonArray
                require(ids.distinct().size == ids.size)
            }
        }
        productJson.decodeFromJsonElement<UnreadSnapshot>(value)
    } catch (failure: SerializationException) {
        throw CorruptionException("invalid unread store", failure)
    } catch (failure: NoSuchElementException) {
        throw CorruptionException("invalid unread store", failure)
    } catch (failure: IllegalArgumentException) {
        throw CorruptionException("invalid unread store", failure)
    }
    override suspend fun writeTo(value: UnreadSnapshot, output: OutputStream) {
        output.write(productJson.encodeToString(value).encodeToByteArray())
    }
}
private val Context.unreadDataStore: DataStore<UnreadSnapshot> by dataStore(
    fileName = "unread.json", serializer = UnreadSerializer,
)

/** Serialized identity-only merges; acknowledgement wins over every concurrent observation. */
internal class UnreadStore(context: Context) {
    private val dataStore = context.applicationContext.unreadDataStore
    private val scope = CoroutineScope(Dispatchers.Main.immediate + SupervisorJob())

    fun read(onReady: (UnreadSnapshot) -> Unit, onUnavailable: () -> Unit) {
        scope.launch {
            val value = try { dataStore.data.first() } catch (_: IOException) {
                // justify-ignore-error: corruption and I/O are explicitly exposed without resetting the store.
                onUnavailable()
                return@launch
            }
            onReady(value)
        }
    }

    fun reconcile(machine: MachineHandle, sessions: List<TmuxSession>, onReady: (UnreadSnapshot) -> Unit, onUnavailable: () -> Unit) =
        update(onReady, onUnavailable) { value ->
            val associations = value.associations.filter { association ->
                association.machine != machine.encoded || sessions.any {
                    it.tmuxId == association.tmuxId && it.identityToken == association.identityToken
                }
            }.toMutableList()
            for (session in sessions) {
                val conversation = session.conversation?.binding?.conversation?.takeIf { it.provider == AgentProvider.Claude } ?: continue
                associations.removeAll { it.machine == machine.encoded && it.tmuxId == session.tmuxId && it.identityToken == session.identityToken }
                associations.add(UnreadAssociation(machine.encoded, session.tmuxId, session.identityToken, session.activePaneId, conversation))
            }
            value.copy(associations = associations)
        }

    fun observe(key: ConversationKey, ids: Set<String>, completeBaseline: Boolean, onReady: (UnreadSnapshot) -> Unit, onUnavailable: () -> Unit) =
        update(onReady, onUnavailable) { value ->
            val record = value.record(key)
            if (record == null && !completeBaseline) return@update value
            val merged = if (record == null) UnreadRecord(key, ids, emptySet()) else {
                record.copy(unreadIds = (record.unreadIds + ids) - record.acknowledgedIds)
            }
            value.copy(conversations = value.conversations.filterNot { it.key == key } + merged)
        }

    fun acknowledge(capture: UnreadCapture, onReady: (UnreadSnapshot) -> Unit, onUnavailable: () -> Unit) =
        update(onReady, onUnavailable) { value ->
            val record = value.record(capture.key) ?: return@update value
            val merged = record.copy(acknowledgedIds = record.acknowledgedIds + capture.ids, unreadIds = record.unreadIds - capture.ids)
            value.copy(conversations = value.conversations.filterNot { it.key == capture.key } + merged)
        }

    private fun update(onReady: (UnreadSnapshot) -> Unit, onUnavailable: () -> Unit, merge: (UnreadSnapshot) -> UnreadSnapshot) {
        scope.launch {
            val value = try { dataStore.updateData { merge(it) } } catch (_: IOException) {
                // justify-ignore-error: prior metadata remains visible; failure never claims persistence.
                onUnavailable()
                return@launch
            }
            onReady(value)
        }
    }
    fun close() = scope.cancel()
}
