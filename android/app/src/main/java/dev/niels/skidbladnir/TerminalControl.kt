package dev.niels.skidbladnir

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

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

/** One screen observation, as the host sent it; sessions.TerminalStatus owns the legal combinations. */
@Serializable internal data class TerminalStatus(
    val activity: TerminalActivity,
    val interaction: TerminalInteraction,
    val notice: TerminalNotice,
    val source: TerminalStatusSource,
)
@Serializable internal enum class TerminalWriteOutcome {
    @SerialName("written") Written, @SerialName("unknown") Unknown,
}
@Serializable internal data class TerminalWriteResult(val outcome: TerminalWriteOutcome)
@Serializable internal enum class TerminalInterrupt {
    @SerialName("written") Written, @SerialName("not_sent") NotSent, @SerialName("unknown") Unknown,
}
@Serializable internal enum class TerminalClosure {
    @SerialName("closed") Closed, @SerialName("not_closed") NotClosed, @SerialName("unknown") Unknown,
}
@Serializable internal data class TerminalCloseResult(val interrupt: TerminalInterrupt, val terminal: TerminalClosure)
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
