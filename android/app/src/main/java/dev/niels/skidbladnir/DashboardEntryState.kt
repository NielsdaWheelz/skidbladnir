package dev.niels.skidbladnir

import android.os.Bundle
import androidx.compose.foundation.lazy.grid.LazyGridState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.savedstate.SavedStateRegistry

internal sealed interface DashboardScope {
    data object All : DashboardScope
    data class Machine(val handle: MachineHandle) : DashboardScope
}

internal sealed interface DashboardItemKey {
    val encoded: String
    data class Space(val fingerprint: String) : DashboardItemKey {
        init { require(DashboardCardKey.isFingerprint(fingerprint)) }
        override val encoded: String get() = "space:$fingerprint"
    }
    data object Unassigned : DashboardItemKey { override val encoded = "space:unassigned" }

    companion object {
        fun parse(encoded: String): DashboardItemKey? = when {
            DashboardCardKey.isFingerprint(encoded) -> DashboardCardKey(encoded)
            encoded == "space:unassigned" -> Unassigned
            encoded.startsWith("space:") && DashboardCardKey.isFingerprint(encoded.substringAfter(':')) ->
                Space(encoded.substringAfter(':'))
            else -> null
        }
    }
}

@JvmInline
internal value class DashboardCardKey(
    val lifetimeFingerprint: String,
) : DashboardItemKey {
    override val encoded: String get() = lifetimeFingerprint
    init {
        // justify-service-invariant-check: Kotlin cannot encode a lowercase SHA-256
        // string's alphabet and length in this value-class type.
        require(isFingerprint(lifetimeFingerprint))
    }

    companion object {
        private val FINGERPRINT = Regex("[0-9a-f]{64}")

        fun isFingerprint(value: String): Boolean = FINGERPRINT.matches(value)
    }
}

internal data class DashboardViewport(
    val anchor: DashboardItemKey?,
    val fallbackIndex: Int,
    val offsetPx: Int,
) {
    init {
        // justify-service-invariant-check: Kotlin cannot encode the nonnegative ranges
        // or the anchor-dependent sibling-field tuple in this data-class type.
        require(fallbackIndex >= 0)
        require(offsetPx >= 0)
        require(anchor != null || fallbackIndex == 0 && offsetPx == 0)
    }
}

internal data class DashboardEntrySnapshot(
    val schemaVersion: Int,
    val scope: DashboardScope,
    val viewport: DashboardViewport,
    val space: DashboardSpaceKey = DashboardSpaceKey.All,
) {
    init {
        // justify-service-invariant-check: the task capsule's exact wire version is a
        // runtime integer boundary and cannot be encoded by this shared snapshot type.
        require(schemaVersion == 2)
    }
}

internal class DashboardEntryState(
    restoredSnapshot: DashboardEntrySnapshot? = null,
) {
    private var currentScope by mutableStateOf(restoredSnapshot?.scope ?: DashboardScope.All)
    private var currentSpace by mutableStateOf(restoredSnapshot?.space?.selection() ?: DashboardSpaceSelection.All)
    private var pendingSnapshot by mutableStateOf(restoredSnapshot)
    private var ownedGridState by mutableStateOf(LazyGridState())
    private var acceptedHandles: Set<MachineHandle>? = null
    private var installed = false

    val scope: DashboardScope get() = currentScope
    val space: DashboardSpaceSelection get() = currentSpace
    val gridState: LazyGridState get() = ownedGridState
    val restorationPending: Boolean get() = pendingSnapshot != null

    fun acceptFleet(handles: Set<MachineHandle>) {
        acceptedHandles = handles.toSet()
        val scopeAccepted = when (val scope = currentScope) {
            DashboardScope.All -> true
            is DashboardScope.Machine -> scope.handle in handles
        }
        if (handles.isEmpty() || !scopeAccepted) resetAll()
    }

    fun selectScope(scope: DashboardScope) {
        if (scope == currentScope) return
        // justify-service-invariant-check: accepted fleet membership is dynamic task
        // state and therefore cannot be encoded in DashboardScope.Machine's type.
        when (scope) {
            DashboardScope.All -> Unit
            is DashboardScope.Machine -> require(scope.handle in checkNotNull(acceptedHandles))
        }
        currentScope = scope
        pendingSnapshot = null
    }

    fun selectSpace(space: DashboardSpaceSelection) {
        if (space.key == currentSpace.key) return
        currentSpace = space
        pendingSnapshot = null
    }

    fun resolveSpace(labels: List<SpaceLabel>) {
        val selected = currentSpace as? DashboardSpaceSelection.Named ?: return
        if (selected.label != null) return
        val label = labels.firstOrNull { spaceFingerprint(it) == selected.fingerprint } ?: return
        currentSpace = selected.copy(label = label)
    }

    fun followCreatedMembership(label: SpaceLabel?) {
        if (currentSpace.matches(label)) return
        selectSpace(label?.let { DashboardSpaceSelection.Named(spaceFingerprint(it), it) }
            ?: DashboardSpaceSelection.Unassigned)
        ownedGridState = LazyGridState()
    }

    fun selectTerminalAccessLoss(handle: MachineHandle) {
        // justify-service-invariant-check: access loss arrives from a dynamic controller
        // fleet; its current accepted membership cannot be represented by MachineHandle.
        require(handle in checkNotNull(acceptedHandles))
        currentScope = DashboardScope.Machine(handle)
        pendingSnapshot = null
        ownedGridState = LazyGridState()
    }

    fun resetAll() {
        currentScope = DashboardScope.All
        currentSpace = DashboardSpaceSelection.All
        pendingSnapshot = null
        ownedGridState = LazyGridState()
    }

    fun restoreOnce(keys: List<DashboardItemKey>) {
        val restored = pendingSnapshot ?: return
        if (keys.isNotEmpty()) {
            val resolvedIndex = restored.viewport.anchor?.let(keys::indexOf)
                ?.takeIf { it >= 0 }
                ?: restored.viewport.fallbackIndex.coerceAtMost(keys.lastIndex)
            gridState.requestScrollToItem(resolvedIndex, restored.viewport.offsetPx)
        }
        pendingSnapshot = null
    }

    fun snapshot(): DashboardEntrySnapshot {
        pendingSnapshot?.let { return it }
        val index = gridState.firstVisibleItemIndex
        val key = gridState.layoutInfo.visibleItemsInfo
            .singleOrNull { it.index == index }
            ?.key as? String
        val anchor = key?.let(DashboardItemKey::parse)
        return DashboardEntrySnapshot(
            schemaVersion = SCHEMA_VERSION,
            scope = currentScope,
            space = currentSpace.key,
            viewport = if (anchor == null) {
                TOP_VIEWPORT
            } else {
                DashboardViewport(anchor, index, gridState.firstVisibleItemScrollOffset)
            },
        )
    }

    fun install(savedStateRegistry: SavedStateRegistry) {
        // justify-service-invariant-check: provider ownership is a lifecycle fact; the
        // registry API has no type that can express one installation per state holder.
        check(!installed)
        installed = true
        savedStateRegistry.consumeRestoredStateForKey(REGISTRY_KEY)?.let { encoded ->
            val restored = decodeSnapshot(encoded)
            if (restored == null) {
                resetAll()
            } else {
                currentScope = restored.scope
                currentSpace = restored.space.selection()
                pendingSnapshot = restored
            }
        }
        savedStateRegistry.registerSavedStateProvider(REGISTRY_KEY) { encodeSnapshot(snapshot()) }
    }

    private companion object {
        const val SCHEMA_VERSION = 2
        const val REGISTRY_KEY = "dev.niels.skidbladnir.dashboard-entry"
        val TOP_VIEWPORT = DashboardViewport(anchor = null, fallbackIndex = 0, offsetPx = 0)

        fun encodeSnapshot(snapshot: DashboardEntrySnapshot): Bundle = Bundle().apply {
            putInt("version", SCHEMA_VERSION)
            when (val scope = snapshot.scope) {
                DashboardScope.All -> putString("scopeKind", "all")
                is DashboardScope.Machine -> {
                    putString("scopeKind", "machine")
                    putString("scopeMachine", scope.handle.encoded)
                }
            }
            when (val space = snapshot.space) {
                DashboardSpaceKey.All -> putString("spaceKind", "all")
                DashboardSpaceKey.Unassigned -> putString("spaceKind", "unassigned")
                is DashboardSpaceKey.Named -> {
                    putString("spaceKind", "named")
                    putString("spaceLabelSha256", space.fingerprint)
                }
            }
            when (val anchor = snapshot.viewport.anchor) {
                null -> putString("anchorKind", "none")
                is DashboardCardKey -> {
                    putString("anchorKind", "session")
                    putString("anchorSha256", anchor.lifetimeFingerprint)
                }
                is DashboardItemKey.Space -> {
                    putString("anchorKind", "space")
                    putString("anchorSha256", anchor.fingerprint)
                }
                DashboardItemKey.Unassigned -> putString("anchorKind", "unassigned")
            }
            putInt("fallbackIndex", snapshot.viewport.fallbackIndex)
            putInt("offsetPx", snapshot.viewport.offsetPx)
        }

        // justify-defect: malformed current-version task state is a trusted-state contract defect.
        fun decodeSnapshot(encoded: Bundle): DashboardEntrySnapshot? {
            val version = encoded.requiredInt("version")
            if (version != SCHEMA_VERSION) return null
            val requiredKeys = mutableSetOf("version", "scopeKind", "spaceKind", "anchorKind", "fallbackIndex", "offsetPx")
            val scope = when (encoded.requiredString("scopeKind")) {
                "all" -> DashboardScope.All
                "machine" -> {
                    requiredKeys += "scopeMachine"
                    DashboardScope.Machine(checkNotNull(MachineHandle.parse(encoded.requiredString("scopeMachine"))))
                }
                else -> error("invalid dashboard machine scope")
            }
            val space = when (encoded.requiredString("spaceKind")) {
                "all" -> DashboardSpaceKey.All
                "unassigned" -> DashboardSpaceKey.Unassigned
                "named" -> {
                    requiredKeys += "spaceLabelSha256"
                    val fingerprint = encoded.requiredString("spaceLabelSha256")
                    check(DashboardCardKey.isFingerprint(fingerprint))
                    DashboardSpaceKey.Named(fingerprint)
                }
                else -> error("invalid dashboard space scope")
            }
            val anchor = when (encoded.requiredString("anchorKind")) {
                "none" -> null
                "unassigned" -> DashboardItemKey.Unassigned
                "session", "space" -> {
                    requiredKeys += "anchorSha256"
                    val fingerprint = encoded.requiredString("anchorSha256")
                    check(DashboardCardKey.isFingerprint(fingerprint))
                    if (encoded.requiredString("anchorKind") == "session") DashboardCardKey(fingerprint)
                    else DashboardItemKey.Space(fingerprint)
                }
                else -> error("invalid dashboard anchor kind")
            }
            check(encoded.keySet() == requiredKeys)
            val index = encoded.requiredInt("fallbackIndex")
            val offset = encoded.requiredInt("offsetPx")
            check(index >= 0 && offset >= 0 && (anchor != null || index == 0 && offset == 0))
            return DashboardEntrySnapshot(version, scope, DashboardViewport(anchor, index, offset), space)
        }

        fun Bundle.requiredInt(key: String): Int {
            check(containsKey(key))
            val lowerDefault = getInt(key, Int.MIN_VALUE)
            val upperDefault = getInt(key, Int.MAX_VALUE)
            check(lowerDefault == upperDefault)
            return lowerDefault
        }

        fun Bundle.requiredString(key: String): String {
            check(containsKey(key))
            return checkNotNull(getString(key))
        }
    }
}
