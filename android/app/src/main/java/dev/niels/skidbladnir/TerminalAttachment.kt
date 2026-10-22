package dev.niels.skidbladnir

import kotlinx.coroutines.Deferred

/**
 * One terminal attempt's live resources, from page Ready until leave: the page, the connection opened on it
 * and the notification visit that connection began. Its identity is the attempt's owner token.
 */
internal class TerminalAttachment(val page: TerminalPage, private val notifications: NotificationOwner) {
    var connection: TerminalConnection? = null
        private set
    private var visit: Deferred<NotificationRow?>? = null
    /** The connection's first output reached the screen. */
    var presented = false
        private set

    /** The connection carries [viewport], so the fitted grid is the first client frame the gateway sees. */
    fun connect(credential: MachineCredential, target: SessionTarget, viewport: TerminalViewport.Fitted, observer: TerminalConnectionObserver) {
        visit = notifications.beginVisit(NotificationKey(target))
        val opened = TerminalConnection(GatewayClient, credential, target, viewport, page, observer)
        connection = opened
        opened.start()
    }

    /** The first presented output acknowledges what the visit's capture saw. */
    fun present() {
        val begun = visit ?: return
        if (presented) return
        presented = true
        notifications.presented(begun)
    }

    fun detach() {
        if (!presented) visit?.cancel()
        page.resetInputState()
        connection?.detach()
    }
}
