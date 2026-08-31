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

@JvmInline
internal value class DashboardCardKey(
    val lifetimeFingerprint: String,
) {
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
    val anchor: DashboardCardKey?,
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
) {
    init {
        // justify-service-invariant-check: the task capsule's exact wire version is a
        // runtime integer boundary and cannot be encoded by this shared snapshot type.
        require(schemaVersion == 1)
    }
}

internal class DashboardEntryState(
    restoredSnapshot: DashboardEntrySnapshot? = null,
) {
    private var currentScope by mutableStateOf(restoredSnapshot?.scope ?: DashboardScope.All)
    private var pendingSnapshot by mutableStateOf(restoredSnapshot)
    private var ownedGridState by mutableStateOf(LazyGridState())
    private var acceptedHandles: Set<MachineHandle>? = null
    private var installed = false

    val scope: DashboardScope get() = currentScope
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
        pendingSnapshot = null
        ownedGridState = LazyGridState()
    }

    fun restoreOnce(keys: List<DashboardCardKey>) {
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
        val anchor = key
            ?.takeIf(DashboardCardKey::isFingerprint)
            ?.let(::DashboardCardKey)
        return DashboardEntrySnapshot(
            schemaVersion = SCHEMA_VERSION,
            scope = currentScope,
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
                pendingSnapshot = restored
            }
        }
        savedStateRegistry.registerSavedStateProvider(REGISTRY_KEY) { encodeSnapshot(snapshot()) }
    }

    private companion object {
        const val SCHEMA_VERSION = 1
        const val REGISTRY_KEY = "dev.niels.skidbladnir.dashboard-entry"
        const val VERSION = "version"
        const val SCOPE_KIND = "scopeKind"
        const val SCOPE_MACHINE = "scopeMachine"
        const val ANCHOR = "anchorLifetimeSha256"
        const val FALLBACK_INDEX = "fallbackIndex"
        const val OFFSET_PX = "offsetPx"
        const val ALL = "all"
        const val MACHINE = "machine"

        val TOP_VIEWPORT = DashboardViewport(anchor = null, fallbackIndex = 0, offsetPx = 0)

        fun encodeSnapshot(snapshot: DashboardEntrySnapshot): Bundle = Bundle().apply {
            putInt(VERSION, snapshot.schemaVersion)
            when (val scope = snapshot.scope) {
                DashboardScope.All -> putString(SCOPE_KIND, ALL)
                is DashboardScope.Machine -> {
                    putString(SCOPE_KIND, MACHINE)
                    putString(SCOPE_MACHINE, scope.handle.encoded)
                }
            }
            snapshot.viewport.anchor?.let { putString(ANCHOR, it.lifetimeFingerprint) }
            putInt(FALLBACK_INDEX, snapshot.viewport.fallbackIndex)
            putInt(OFFSET_PX, snapshot.viewport.offsetPx)
        }

        // justify-defect: a current-v1 Bundle is trusted process-owned state; malformed
        // keys, primitive types, or values mean a schema/code mismatch and must not fall back.
        fun decodeSnapshot(encoded: Bundle): DashboardEntrySnapshot? {
            val version = encoded.requiredInt(VERSION)
            if (version != SCHEMA_VERSION) return null
            val scopeKind = encoded.requiredString(SCOPE_KIND)
            val anchor = if (encoded.containsKey(ANCHOR)) {
                encoded.requiredString(ANCHOR).also {
                    check(DashboardCardKey.isFingerprint(it))
                }.let(::DashboardCardKey)
            } else {
                null
            }
            val requiredKeys = mutableSetOf(VERSION, SCOPE_KIND, FALLBACK_INDEX, OFFSET_PX)
            if (anchor != null) requiredKeys += ANCHOR
            val scope = when (scopeKind) {
                ALL -> DashboardScope.All
                MACHINE -> {
                    requiredKeys += SCOPE_MACHINE
                    DashboardScope.Machine(
                        checkNotNull(MachineHandle.parse(encoded.requiredString(SCOPE_MACHINE))),
                    )
                }
                else -> error("invalid Dashboard scope kind")
            }
            check(encoded.keySet() == requiredKeys)
            val fallbackIndex = encoded.requiredInt(FALLBACK_INDEX)
            val offsetPx = encoded.requiredInt(OFFSET_PX)
            check(fallbackIndex >= 0)
            check(offsetPx >= 0)
            check(anchor != null || fallbackIndex == 0 && offsetPx == 0)
            return DashboardEntrySnapshot(
                schemaVersion = version,
                scope = scope,
                viewport = DashboardViewport(anchor, fallbackIndex, offsetPx),
            )
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
