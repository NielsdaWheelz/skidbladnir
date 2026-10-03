package dev.niels.skidbladnir

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
    if (forge.pending || forge.surface != ForgeSurface.Form || machine?.canForge != true ||
        machine.machine.handle != forge.form.machineHandle) return emptyList()
    return when (classifyWorkingDirectory(forge.form.cwd)) {
        WorkingDirectoryInput.Home -> {
            val home = checkNotNull(WorkingDirectoryPath.parse("~"))
            // canForge guarantees Fresh; Kotlin cannot narrow through the derived property.
            val inventory = (machine.inventory as InventoryState.Fresh).snapshot.inventory
            listOf(home) + inventory.sessions.asSequence()
                .filter { it.connection == null }
                .mapNotNull(TmuxSession::cwd)
                .map { cwd ->
                    WorkingDirectoryPath.parse(cwd)
                        ?: throw ProtocolDecodeException("inventory working-directory value")
                }
                .filter { it != home }
                .distinct()
                .sortedWith { first, second -> compareCaseInsensitiveUtf8(first.encoded, second.encoded) }
                .toList()
        }
        is WorkingDirectoryInput.Query -> (forge.directorySearch as? DirectorySearchState.Ready)?.result?.directories.orEmpty()
        is WorkingDirectoryInput.Literal, WorkingDirectoryInput.Invalid -> emptyList()
    }
}
