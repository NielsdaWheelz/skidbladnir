package dev.niels.skidbladnir

import kotlinx.serialization.Serializable
import kotlinx.serialization.decodeFromString

internal const val MAXIMUM_WORKING_DIRECTORY_BYTES = 4_096

internal class WorkingDirectoryPath private constructor(val encoded: String) {
    companion object {
        fun parse(candidate: String): WorkingDirectoryPath? {
            if (candidate.utf8ByteCountWithin(MAXIMUM_WORKING_DIRECTORY_BYTES) == null) return null
            if (candidate.hasDisplayUnsafeCodePoint()) return null
            if (candidate != "~" && !candidate.startsWith("~/") && !candidate.startsWith('/')) return null
            return WorkingDirectoryPath(candidate)
        }
    }

    override fun equals(other: Any?): Boolean = other is WorkingDirectoryPath && encoded == other.encoded
    override fun hashCode(): Int = encoded.hashCode()
    override fun toString(): String = encoded
}

/** A directory under the host's home as its listing names it: `~` or `~/a/b`. */
@JvmInline @Serializable internal value class HomeDirectory(val encoded: String) {
    val basename: String get() = if (this == Home) "~" else encoded.substringAfterLast('/')
    val hidden: Boolean get() = this != Home && basename.startsWith('.')

    companion object {
        val Home = HomeDirectory("~")
    }
}

internal sealed interface ParentDirectory {
    data object Absent : ParentDirectory
    data class Available(val directory: HomeDirectory) : ParentDirectory
}
internal enum class DirectoryOmissions { None, Present }
@Serializable internal data class DirectoryEntry(val directory: HomeDirectory)
internal data class DirectoryListing(
    val directory: HomeDirectory,
    val parent: ParentDirectory,
    val children: List<DirectoryEntry>,
    val omissions: DirectoryOmissions,
)

internal data class DirectorySearchResult(val directories: List<WorkingDirectoryPath>, val omitted: Boolean)

@Serializable private data class WireDirectoryListing(
    val directory: HomeDirectory,
    val parentDirectory: HomeDirectory? = null,
    val children: List<DirectoryEntry>,
    val omitted: Boolean,
)

@Serializable private data class WireDirectorySearch(val directories: List<String>, val omitted: Boolean)

internal fun decodeDirectoryListing(encoded: String): DirectoryListing {
    val wire = productJson.decodeFromString<WireDirectoryListing>(encoded)
    return DirectoryListing(
        wire.directory,
        wire.parentDirectory?.let(ParentDirectory::Available) ?: ParentDirectory.Absent,
        wire.children,
        if (wire.omitted) DirectoryOmissions.Present else DirectoryOmissions.None,
    )
}

/** A result the forge could not submit as typed is dropped, not shown. */
internal fun decodeDirectorySearch(encoded: String): DirectorySearchResult {
    val wire = productJson.decodeFromString<WireDirectorySearch>(encoded)
    return DirectorySearchResult(wire.directories.mapNotNull(WorkingDirectoryPath::parse), wire.omitted)
}

internal sealed interface WorkingDirectoryInput {
    data object Home : WorkingDirectoryInput
    data class Literal(val path: WorkingDirectoryPath) : WorkingDirectoryInput
    data class Query(val terms: List<String>) : WorkingDirectoryInput
    data object Invalid : WorkingDirectoryInput
}

internal fun classifyWorkingDirectory(draft: String): WorkingDirectoryInput {
    if (draft.isEmpty()) return WorkingDirectoryInput.Home
    if (draft.startsWith('/') || draft.startsWith('~')) {
        return WorkingDirectoryPath.parse(draft)?.let(WorkingDirectoryInput::Literal)
            ?: WorkingDirectoryInput.Invalid
    }
    val trimmed = draft.trimStart()
    if (trimmed.startsWith('/') || trimmed.startsWith('~') || draft.hasDisplayUnsafeCodePoint()) {
        return WorkingDirectoryInput.Invalid
    }
    val terms = mutableListOf<String>()
    var bytes = 0
    var index = 0
    while (index < draft.length) {
        while (index < draft.length && draft[index].isWhitespace()) index++
        if (index == draft.length) break
        val start = index
        while (index < draft.length && !draft[index].isWhitespace()) index++
        val term = draft.substring(start, index)
        bytes += term.utf8ByteCountWithin(256) ?: return WorkingDirectoryInput.Invalid
        if (terms.size == 8 || bytes > 256) return WorkingDirectoryInput.Invalid
        terms += term
    }
    return if (terms.isEmpty()) WorkingDirectoryInput.Invalid else WorkingDirectoryInput.Query(terms)
}

internal sealed interface DirectorySearchState {
    data object Idle : DirectorySearchState
    // Each edit gets a distinct snapshot; reference equality binds debounce and replies to it.
    class Loading : DirectorySearchState
    data class Ready(val result: DirectorySearchResult) : DirectorySearchState
    data class Failed(val failure: GatewayFailure) : DirectorySearchState
}

internal fun workingDirectoryChoices(forge: ForgeState, machine: MachineState?): List<WorkingDirectoryPath> {
    if (forge.pending || forge.surface != ForgeSurface.Form || machine?.canMutate != true ||
        machine.machine.handle != forge.form.machineHandle) return emptyList()
    return when (classifyWorkingDirectory(forge.form.cwd)) {
        WorkingDirectoryInput.Home -> {
            val home = checkNotNull(WorkingDirectoryPath.parse("~"))
            // canMutate guarantees Fresh; Kotlin cannot narrow through the derived property.
            val inventory = (machine.inventory as InventoryState.Fresh).snapshot.inventory
            listOf(home) + inventory.sessions.asSequence()
                .filter { it.connection == null }
                .mapNotNull(TmuxSession::cwd)
                .mapNotNull(WorkingDirectoryPath::parse)
                .filter { it != home }
                .distinct()
                .sortedWith { first, second -> compareCaseInsensitiveUtf8(first.encoded, second.encoded) }
                .toList()
        }
        is WorkingDirectoryInput.Query -> (forge.directorySearch as? DirectorySearchState.Ready)?.result?.directories.orEmpty()
        is WorkingDirectoryInput.Literal, WorkingDirectoryInput.Invalid -> emptyList()
    }
}
