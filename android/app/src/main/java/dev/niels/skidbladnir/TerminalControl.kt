package dev.niels.skidbladnir

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.decodeFromJsonElement

@Serializable internal enum class TerminalActivity {
    @SerialName("starting") Starting, @SerialName("working") Working, @SerialName("idle") Idle, @SerialName("unknown") Unknown,
}
@Serializable internal enum class TerminalInteraction {
    @SerialName("none") None, @SerialName("permission") Permission, @SerialName("question") Question,
    @SerialName("confirmation") Confirmation, @SerialName("setup") Setup, @SerialName("input") Input,
    @SerialName("menu") Menu, @SerialName("unknown") Unknown,
}
@Serializable internal enum class TerminalNotice {
    @SerialName("none") None, @SerialName("interrupted") Interrupted, @SerialName("error") Error,
}
@Serializable internal enum class TerminalStatusSource {
    @SerialName("terminal") Terminal, @SerialName("unavailable") Unavailable,
}
@Serializable internal enum class TerminalStatusReason {
    @SerialName("recognized") Recognized, @SerialName("partial") Partial, @SerialName("layout_unknown") LayoutUnknown,
    @SerialName("evidence_clipped") EvidenceClipped, @SerialName("evidence_conflict") EvidenceConflict,
    @SerialName("provider_unrecognized") ProviderUnrecognized, @SerialName("remote_context") RemoteContext,
    @SerialName("foreground_changed") ForegroundChanged, @SerialName("observation_timeout") ObservationTimeout,
    @SerialName("capture_failed") CaptureFailed, @SerialName("process_failed") ProcessFailed,
}

/** One screen observation. The host's sessions.TerminalStatus owns the legal combinations; init mirrors its Valid. */
@Serializable internal data class TerminalStatus(
    val activity: TerminalActivity,
    val interaction: TerminalInteraction,
    val notice: TerminalNotice,
    val source: TerminalStatusSource,
    val reason: TerminalStatusReason,
) {
    init {
        // justify-service-invariant-check: this flat record mirrors the host's sessions.TerminalStatus and its
        // five-member wire. A sealed split would need custom decoding and still leave reason's dimension counts here.
        val activityKnown = activity != TerminalActivity.Unknown
        val interactionKnown = interaction != TerminalInteraction.Unknown
        val terminal = source == TerminalStatusSource.Terminal
        require(when (reason) {
            TerminalStatusReason.ObservationTimeout, TerminalStatusReason.CaptureFailed, TerminalStatusReason.ProcessFailed ->
                !terminal && !activityKnown && !interactionKnown && notice == TerminalNotice.None
            TerminalStatusReason.ProviderUnrecognized, TerminalStatusReason.RemoteContext, TerminalStatusReason.ForegroundChanged ->
                terminal && !activityKnown && !interactionKnown && notice == TerminalNotice.None
            TerminalStatusReason.Recognized -> terminal && activityKnown && interactionKnown
            TerminalStatusReason.Partial -> terminal && activityKnown != interactionKnown
            TerminalStatusReason.LayoutUnknown -> terminal && !activityKnown && !interactionKnown
            TerminalStatusReason.EvidenceClipped, TerminalStatusReason.EvidenceConflict -> terminal && (!activityKnown || !interactionKnown)
        })
    }
}
@Serializable internal enum class TerminalWriteOutcome {
    @SerialName("written") Written, @SerialName("unknown") Unknown,
}
@Serializable internal data class TerminalWriteResult(val method: String, val outcome: TerminalWriteOutcome) {
    init { require(method == "terminal") }
}
@Serializable internal enum class TerminalInterrupt {
    @SerialName("written") Written, @SerialName("not_sent") NotSent, @SerialName("unknown") Unknown,
}
@Serializable internal enum class TerminalClosure {
    @SerialName("closed") Closed, @SerialName("not_closed") NotClosed, @SerialName("unknown") Unknown,
}
@Serializable internal data class TerminalCloseResult(val interrupt: TerminalInterrupt, val terminal: TerminalClosure)
@Serializable private data class TerminalControlRequest(val identityToken: String, val paneId: String)

internal fun encodeTerminalControlRequest(target: SessionTarget): String = productJson.encodeToString(
    TerminalControlRequest.serializer(), TerminalControlRequest(target.session.identityToken, target.session.activePaneId),
)
internal fun decodeTerminalWriteResult(encoded: String): TerminalWriteResult = decodeProtocol {
    productJson.decodeFromJsonElement<TerminalWriteResult>(strictJsonObject(encoded))
}
internal fun decodeTerminalCloseResult(encoded: String): TerminalCloseResult = decodeProtocol {
    productJson.decodeFromJsonElement<TerminalCloseResult>(strictJsonObject(encoded))
}
internal fun terminalStopMessage(result: TerminalWriteResult): String = when (result.outcome) {
    TerminalWriteOutcome.Written -> "interrupt key sent; stopping is unconfirmed."
    TerminalWriteOutcome.Unknown -> TERMINAL_INPUT_UNKNOWN
}
internal const val TERMINAL_INPUT_UNKNOWN = "could not confirm terminal input. inspect the terminal before trying again."
internal const val TERMINAL_CLOSE_UNKNOWN = "could not confirm terminal closure. refresh before trying again."
internal const val TERMINAL_SHARED_WORK = "work shared elsewhere or running remotely may continue."
internal const val TERMINAL_STOP_ACTION = "send interrupt"
internal const val TERMINAL_CLOSE_ACTION = "interrupt and close terminal"
internal const val TERMINAL_ONLY_CLOSE_ACTION = "close terminal only"

internal fun terminalCloseMessage(result: TerminalCloseResult): String = when (result.terminal) {
    TerminalClosure.Closed -> when (result.interrupt) {
        TerminalInterrupt.Written -> "interrupt key sent; stopping is unconfirmed. terminal closed. $TERMINAL_SHARED_WORK"
        TerminalInterrupt.NotSent, TerminalInterrupt.Unknown -> "terminal closed; interruption unconfirmed."
    }
    TerminalClosure.NotClosed -> when (result.interrupt) {
        TerminalInterrupt.Written -> "interrupt key sent; stopping is unconfirmed. terminal left open."
        TerminalInterrupt.NotSent -> "interrupt not sent; terminal left open. refresh before trying again."
        TerminalInterrupt.Unknown -> "interruption unconfirmed; terminal left open. inspect the terminal before trying again."
    }
    TerminalClosure.Unknown -> when (result.interrupt) {
        TerminalInterrupt.Written -> "interrupt key sent; stopping is unconfirmed. $TERMINAL_CLOSE_UNKNOWN"
        TerminalInterrupt.NotSent -> "interrupt not sent. $TERMINAL_CLOSE_UNKNOWN"
        TerminalInterrupt.Unknown -> "interruption unconfirmed. $TERMINAL_CLOSE_UNKNOWN"
    } + " $TERMINAL_SHARED_WORK"
}

internal fun closeConfirmationBody(label: MachineLabel, target: SessionTarget, terminalOnly: Boolean): String {
    val effect = if (terminalOnly) "close this entire session without sending an interrupt."
    else "send one interrupt to the selected pane, then close this entire session. closure proceeds even if interruption fails."
    return "${target.session.tmuxName} on ${label.text}: $effect $TERMINAL_SHARED_WORK"
}
