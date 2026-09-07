package dev.niels.skidbladnir

import android.content.ClipData
import android.content.ClipDescription
import android.content.ClipboardManager
import android.graphics.Rect
import android.os.Looper
import android.os.PersistableBundle
import android.view.ActionMode
import android.view.HapticFeedbackConstants
import android.view.Menu
import android.view.MenuItem
import android.view.View
import android.window.OnBackInvokedCallback
import android.window.OnBackInvokedDispatcher
import android.widget.Toast
import kotlin.math.roundToInt

private const val MAXIMUM_SAFE_JAVASCRIPT_INTEGER = 9_007_199_254_740_991L

private data class TerminalSelectionAnchor(
    val x: Double,
    val y: Double,
)

private sealed interface TerminalSelectionState {
    data object Idle : TerminalSelectionState
    data class Selecting(val generation: String) : TerminalSelectionState
    data class Selected(
        val generation: String,
        val snapshot: String,
        val anchor: TerminalSelectionAnchor,
        val actionMode: ActionMode,
    ) : TerminalSelectionState
    data class Clearing(val generation: String) : TerminalSelectionState
}

internal class TerminalSelectionController(
    private val view: View,
    private val sendClearSelection: (String) -> Unit,
    private val isPageLiveAuthoritatively: () -> Boolean,
    private val isInteractionAuthorized: () -> Boolean,
) {
    private var state: TerminalSelectionState = TerminalSelectionState.Idle
    private var pageIsLive = true
    private var lastSelectionGeneration = 0L
    private var pendingAnchor: TerminalSelectionAnchor? = null
    private var suppressedDestroy: ActionMode? = null
    private var openingActionMode = false
    private var openingActionModeWasDestroyed = false
    private var registeredBackDispatcher: OnBackInvokedDispatcher? = null
    private val backInvokedCallback = OnBackInvokedCallback {
        val selected = state as? TerminalSelectionState.Selected ?: return@OnBackInvokedCallback
        enterClearing(selected.generation, selected.actionMode)
    }
    private val clipboard by lazy {
        view.context.getSystemService(ClipboardManager::class.java)
    }

    private val actionModeCallback = object : ActionMode.Callback2() {
        override fun onCreateActionMode(mode: ActionMode, menu: Menu): Boolean {
            if (!pageIsLive || state !is TerminalSelectionState.Selecting ||
                !isInteractionAuthorized()
            ) {
                return false
            }
            menu.add(
                Menu.NONE,
                R.id.terminal_selection_copy_action,
                Menu.NONE,
                R.string.terminal_selection_copy,
            ).setShowAsAction(MenuItem.SHOW_AS_ACTION_ALWAYS)
            return true
        }

        override fun onPrepareActionMode(mode: ActionMode, menu: Menu): Boolean = false

        override fun onActionItemClicked(mode: ActionMode, item: MenuItem): Boolean {
            if (item.itemId != R.id.terminal_selection_copy_action) return false
            val selected = state as? TerminalSelectionState.Selected ?: return true
            if (selected.actionMode !== mode) return true
            if (!pageIsLive || !isInteractionAuthorized()) return true
            copy(selected)
            return true
        }

        override fun onDestroyActionMode(mode: ActionMode) {
            if (suppressedDestroy === mode) {
                suppressedDestroy = null
                return
            }
            if (openingActionMode) {
                openingActionModeWasDestroyed = true
                return
            }
            val selected = state as? TerminalSelectionState.Selected ?: return
            if (selected.actionMode === mode) enterClearing(selected.generation, null)
        }

        override fun onGetContentRect(mode: ActionMode, view: View, outRect: Rect) {
            val anchor = (state as? TerminalSelectionState.Selected)?.anchor ?: pendingAnchor
            val width = maxOf(1, view.width)
            val height = maxOf(1, view.height)
            val x = ((anchor?.x ?: 0.5) * (width - 1)).roundToInt().coerceIn(0, width - 1)
            val y = ((anchor?.y ?: 0.5) * (height - 1)).roundToInt().coerceIn(0, height - 1)
            outRect.set(x, y, minOf(width, x + 1), minOf(height, y + 1))
        }
    }

    fun selectionStarted(generation: String): Boolean {
        requireMainThread()
        if (!pageIsLive || state != TerminalSelectionState.Idle || !isInteractionAuthorized()) {
            return false
        }
        val numericGeneration = generation.toLongOrNull() ?: return false
        if (lastSelectionGeneration == MAXIMUM_SAFE_JAVASCRIPT_INTEGER ||
            numericGeneration != lastSelectionGeneration + 1
        ) {
            return false
        }
        lastSelectionGeneration = numericGeneration
        state = TerminalSelectionState.Selecting(generation)
        view.performHapticFeedback(HapticFeedbackConstants.LONG_PRESS)
        return true
    }

    fun selectionAvailable(
        generation: String,
        text: String,
        anchorX: Double,
        anchorY: Double,
    ): Boolean {
        requireMainThread()
        val selecting = state as? TerminalSelectionState.Selecting ?: return false
        if (!pageIsLive || selecting.generation != generation || !isInteractionAuthorized()) {
            return false
        }
        val anchor = TerminalSelectionAnchor(anchorX, anchorY)
        pendingAnchor = anchor
        openingActionMode = true
        openingActionModeWasDestroyed = false
        val actionMode = try {
            view.startActionMode(actionModeCallback, ActionMode.TYPE_FLOATING)
        } finally {
            openingActionMode = false
        }
        val openingWasDestroyed = openingActionModeWasDestroyed
        openingActionModeWasDestroyed = false
        pendingAnchor = null
        val stillSelecting = (state as? TerminalSelectionState.Selecting)?.generation == generation
        if (actionMode == null || openingWasDestroyed || !stillSelecting ||
            !pageIsLive || !isInteractionAuthorized()
        ) {
            actionMode?.let(::finishSuppressed)
            if (stillSelecting) {
                if (isPageLiveAuthoritatively()) {
                    enterClearing(generation, null)
                } else {
                    state = TerminalSelectionState.Idle
                }
            }
        } else {
            state = TerminalSelectionState.Selected(generation, text, anchor, actionMode)
            if (registerBackCallback()) {
                actionMode.invalidateContentRect()
            } else {
                enterClearing(generation, actionMode)
            }
        }
        return true
    }

    fun selectionCopyRejected(generation: String): Boolean {
        requireMainThread()
        val selecting = state as? TerminalSelectionState.Selecting ?: return false
        if (!pageIsLive || selecting.generation != generation) return false
        Toast.makeText(
            view.context,
            R.string.terminal_selection_too_large,
            Toast.LENGTH_SHORT,
        ).show()
        enterClearing(generation, null)
        return true
    }

    fun selectionCleared(generation: String): Boolean {
        requireMainThread()
        if (!pageIsLive) return false
        val stateGeneration = when (val current = state) {
            TerminalSelectionState.Idle -> return false
            is TerminalSelectionState.Selecting -> current.generation
            is TerminalSelectionState.Selected -> current.generation
            is TerminalSelectionState.Clearing -> current.generation
        }
        if (stateGeneration != generation) return false
        val actionMode = (state as? TerminalSelectionState.Selected)?.actionMode
        unregisterBackCallback()
        state = TerminalSelectionState.Idle
        pendingAnchor = null
        actionMode?.let(::finishSuppressed)
        return true
    }

    fun resetForLiveLifecycle() {
        requireMainThread()
        if (!pageIsLive) return
        when (val current = state) {
            TerminalSelectionState.Idle,
            is TerminalSelectionState.Clearing,
            -> Unit
            is TerminalSelectionState.Selecting -> enterClearing(current.generation, null)
            is TerminalSelectionState.Selected -> enterClearing(current.generation, current.actionMode)
        }
    }

    fun contentRectChanged() {
        requireMainThread()
        (state as? TerminalSelectionState.Selected)?.actionMode?.invalidateContentRect()
    }

    fun permitsTerminalTapIme(): Boolean {
        requireMainThread()
        return pageIsLive && state == TerminalSelectionState.Idle
    }

    fun pageFailedOrDisposed() {
        requireMainThread()
        if (!pageIsLive) return
        pageIsLive = false
        val actionMode = (state as? TerminalSelectionState.Selected)?.actionMode
        unregisterBackCallback()
        state = TerminalSelectionState.Idle
        pendingAnchor = null
        actionMode?.let(::finishSuppressed)
    }

    private fun copy(selected: TerminalSelectionState.Selected) {
        requireMainThread()
        // This lock-backed authorization is the write linearization point. A
        // revocation committed afterward is defined as later; ClipboardManager
        // IPC must never run while the WebView output lock is held.
        if (!pageIsLive || !isInteractionAuthorized()) return
        val clip = ClipData.newPlainText(
            view.context.getString(R.string.terminal_selection_clip_label),
            selected.snapshot,
        )
        clip.description.extras = PersistableBundle().apply {
            putBoolean(ClipDescription.EXTRA_IS_REMOTE_DEVICE, true)
        }
        try {
            clipboard.setPrimaryClip(clip)
        } catch (_: SecurityException) {
            Toast.makeText(
                view.context,
                R.string.terminal_selection_copy_failed,
                Toast.LENGTH_SHORT,
            ).show()
            return
        }
        enterClearing(selected.generation, selected.actionMode)
    }

    private fun enterClearing(generation: String, actionMode: ActionMode?) {
        unregisterBackCallback()
        state = TerminalSelectionState.Clearing(generation)
        pendingAnchor = null
        sendClearSelection(generation)
        actionMode?.let(::finishSuppressed)
    }

    private fun registerBackCallback(): Boolean {
        check(registeredBackDispatcher == null)
        val dispatcher = view.findOnBackInvokedDispatcher() ?: return false
        return try {
            dispatcher.registerOnBackInvokedCallback(
                OnBackInvokedDispatcher.PRIORITY_OVERLAY,
                backInvokedCallback,
            )
            registeredBackDispatcher = dispatcher
            true
        } catch (_: RuntimeException) {
            false
        }
    }

    private fun unregisterBackCallback() {
        val dispatcher = registeredBackDispatcher ?: return
        registeredBackDispatcher = null
        try {
            dispatcher.unregisterOnBackInvokedCallback(backInvokedCallback)
        } catch (_: RuntimeException) {
            // State still exits Selected, so a stale framework callback is inert.
        }
    }

    private fun finishSuppressed(actionMode: ActionMode) {
        suppressedDestroy = actionMode
        actionMode.finish()
        if (suppressedDestroy === actionMode) suppressedDestroy = null
    }

    private fun requireMainThread() {
        check(Looper.myLooper() == Looper.getMainLooper())
    }
}
