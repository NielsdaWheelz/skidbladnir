package dev.niels.skidbladnir

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.decodeFromJsonElement

@Serializable internal enum class TerminalState {
    @SerialName("working") Working, @SerialName("blocked") Blocked,
    @SerialName("idle") Idle, @SerialName("unknown") Unknown,
}
@Serializable internal enum class TerminalStatusSource {
    @SerialName("terminal") Terminal, @SerialName("unavailable") Unavailable,
}
@Serializable internal data class TerminalStatus(val state: TerminalState, val source: TerminalStatusSource) {
    init { require(source == TerminalStatusSource.Terminal || state == TerminalState.Unknown) }
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
