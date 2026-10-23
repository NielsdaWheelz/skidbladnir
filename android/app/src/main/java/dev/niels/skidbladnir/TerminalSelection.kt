package dev.niels.skidbladnir

import android.content.ClipData
import android.content.ClipDescription
import android.content.ClipboardManager
import android.graphics.Rect
import android.os.PersistableBundle
import android.view.ActionMode
import android.view.Menu
import android.view.MenuItem
import android.view.View
import android.widget.Toast
import android.window.OnBackInvokedCallback
import android.window.OnBackInvokedDispatcher
import kotlin.math.roundToInt

// The native half of the page-owned selection: one floating Copy toolbar over
// the page's released selection. Closing it by Copy, Back or a framework
// dismissal calls onDismissed so the page drops its highlight; hide() closes it
// quietly because the page already dropped the selection or is being reset.
// Main-thread only, like the view it belongs to.
internal class TerminalSelectionToolbar(
    private val view: View,
    private val onDismissed: () -> Unit,
) {
    private var mode: ActionMode? = null
    private var backDispatcher: OnBackInvokedDispatcher? = null
    private var text = ""
    private var anchorX = 0.0
    private var anchorY = 0.0

    // The floating toolbar does not consume system Back, so an overlay-priority
    // callback closes it before the screen's detach handler sees Back.
    private val back = OnBackInvokedCallback { dismiss() }

    private val callback = object : ActionMode.Callback2() {
        override fun onCreateActionMode(mode: ActionMode, menu: Menu): Boolean {
            this@TerminalSelectionToolbar.mode = mode
            backDispatcher = view.findOnBackInvokedDispatcher()?.also {
                it.registerOnBackInvokedCallback(OnBackInvokedDispatcher.PRIORITY_OVERLAY, back)
            }
            menu.add(Menu.NONE, R.id.terminal_selection_copy_action, Menu.NONE, R.string.terminal_selection_copy)
                .setShowAsAction(MenuItem.SHOW_AS_ACTION_ALWAYS)
            return true
        }

        override fun onPrepareActionMode(mode: ActionMode, menu: Menu): Boolean = false

        override fun onActionItemClicked(mode: ActionMode, item: MenuItem): Boolean {
            if (item.itemId != R.id.terminal_selection_copy_action) return false
            copy()
            return true
        }

        // A close this class started has already cleared `mode`, so only the
        // framework's own dismissal reaches onDismissed.
        override fun onDestroyActionMode(mode: ActionMode) {
            if (mode !== this@TerminalSelectionToolbar.mode) return
            release()
            onDismissed()
        }

        override fun onGetContentRect(mode: ActionMode, view: View, outRect: Rect) {
            val x = (anchorX * (maxOf(1, view.width) - 1)).roundToInt()
            val y = (anchorY * (maxOf(1, view.height) - 1)).roundToInt()
            outRect.set(x, y, x + 1, y + 1)
        }
    }

    // The anchors are the release point as fractions of the view, in 0..1.
    fun show(text: String, anchorX: Double, anchorY: Double) {
        this.text = text
        this.anchorX = anchorX
        this.anchorY = anchorY
        if (view.startActionMode(callback, ActionMode.TYPE_FLOATING) == null) {
            release()
            onDismissed()
        }
    }

    fun hide() {
        val shown = mode ?: return
        release()
        shown.finish()
    }

    fun contentRectChanged() {
        mode?.invalidateContentRect()
    }

    private fun dismiss() {
        hide()
        onDismissed()
    }

    private fun release() {
        mode = null
        text = ""
        backDispatcher?.unregisterOnBackInvokedCallback(back)
        backDispatcher = null
    }

    // Copy is reachable only while the toolbar is shown, and the host hides it
    // first on every loss of liveness, enablement, attachment or window focus.
    private fun copy() {
        val clip = ClipData.newPlainText(view.context.getString(R.string.terminal_selection_clip_label), text)
        clip.description.extras = PersistableBundle().apply {
            putBoolean(ClipDescription.EXTRA_IS_REMOTE_DEVICE, true)
        }
        try {
            view.context.getSystemService(ClipboardManager::class.java).setPrimaryClip(clip)
        } catch (_: SecurityException) {
            Toast.makeText(view.context, R.string.terminal_selection_copy_failed, Toast.LENGTH_SHORT).show()
            return
        }
        dismiss()
    }
}
