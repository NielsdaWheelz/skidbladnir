package dev.niels.skidbladnir

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.systemBarsPadding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.key
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.AndroidView

@Composable
internal fun TerminalScreen(
    state: SkidbladnirUiState.Terminal,
    controller: SkidbladnirController,
) {
    var modifiers by remember(state.attempt) {
        mutableStateOf(
            TerminalModifiers(
                control = TerminalModifierPhase.Off,
                alt = TerminalModifierPhase.Off,
            ),
        )
    }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(Ink)
            .systemBarsPadding()
            .imePadding()
            .testTag("terminal-screen-${state.machine.machine.handle.encoded}"),
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 12.dp, vertical = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            DetachButton(
                onClick = controller::detachToSessions,
            )
            TerminalRenameControl(
                machine = state.machine.machine,
                target = state.target,
                presence = terminalPresence(state),
                presenceColor = terminalPresenceColor(state.connection),
                enabled = terminalActionAdmissible(state.machine.canMutate, state.connection),
                onClick = controller::openRename,
                modifier = Modifier.weight(1f).testTag(terminalStatusTag(state.connection)),
            )
            KillButton(
                machineLabel = state.machine.machine.label,
                target = state.target,
                enabled = terminalActionAdmissible(state.machine.canMutate, state.connection),
                onClick = { controller.requestKill(state.target) },
                modifier = Modifier.testTag("terminal-kill"),
            )
        }

        Box(
            modifier = Modifier
                .fillMaxWidth()
                .weight(1f),
        ) {
            if (state.connection != TerminalUiStatus.Verifying) key(state.attempt) {
                AndroidView(
                    modifier = Modifier.fillMaxSize(),
                    factory = { context ->
                        LockedTerminalWebView(
                            context = context,
                            listener = object : TerminalPageListener {
                                override fun onReady(page: TerminalPage) {
                                    controller.terminalPageReady(state.attempt, page)
                                }

                                override fun onInput(bytes: ByteArray) {
                                    controller.sendTerminal(state.attempt, bytes)
                                }

                                override fun onResize(columns: Int, rows: Int) {
                                    controller.resizeTerminal(state.attempt, columns, rows)
                                }

                                override fun onModifiersChanged(newModifiers: TerminalModifiers) {
                                    modifiers = newModifiers
                                }

                                override fun onUnavailable() {
                                    controller.terminalPageFailed(state.attempt)
                                }
                            },
                        )
                    },
                    update = { view ->
                        view.isEnabled = state.connection is TerminalUiStatus.Connected
                        if (!view.isEnabled) view.clearFocus()
                    },
                    onRelease = LockedTerminalWebView::dispose,
                )
            }

            when (val connection = state.connection) {
                TerminalUiStatus.Verifying -> TerminalWaiting("Verifying ${state.machine.machine.label.text} and session lifetime…")
                TerminalUiStatus.Preparing -> TerminalWaiting("Preparing terminal…")
                TerminalUiStatus.Connecting -> TerminalWaiting("Connecting…")
                is TerminalUiStatus.Connected -> Unit
                is TerminalUiStatus.ReconnectRequired -> ReconnectPanel(
                    machineLabel = state.machine.machine.label,
                    message = connection.message,
                    actionAdmissible = terminalActionAdmissible(state.machine.canMutate, state.connection),
                    onReattach = controller::reattachTerminal,
                    onSessions = controller::detachToSessions,
                )
            }
        }

        TerminalKeyDeck(
            modifiers = modifiers,
            enabled = state.connection is TerminalUiStatus.Connected,
            onAccessory = { controller.sendTerminalAccessory(state.attempt, it) },
        )
    }

    state.kill?.let { kill ->
        KillConfirmation(
            state = kill,
            actionAdmissible = terminalActionAdmissible(state.machine.canMutate, state.connection),
            onDismiss = controller::dismissKill,
            onConfirm = controller::confirmKill,
        )
    }
    state.rename?.let { rename ->
        SessionRenameSheet(
            machine = state.machine.machine,
            terminalTarget = state.target,
            state = rename,
            terminalActionsAdmissible = terminalActionAdmissible(state.machine.canMutate, state.connection),
            onDraftChange = controller::updateRenameDraft,
            onDismiss = controller::dismissRename,
            onSubmit = controller::submitRename,
        )
    }
}

@Composable
private fun TerminalWaiting(message: String) {
    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(Ink.copy(alpha = 0.88f)),
        contentAlignment = Alignment.Center,
    ) {
        Column(horizontalAlignment = Alignment.CenterHorizontally) {
            CircularProgressIndicator()
            Text(message, color = Muted, modifier = Modifier.padding(top = 12.dp))
            Text(
                "Input stays locked until the fresh attachment is ready.",
                color = Muted,
                style = MaterialTheme.typography.labelSmall,
                modifier = Modifier.padding(top = 4.dp),
            )
        }
    }
}

@Composable
private fun ReconnectPanel(
    machineLabel: MachineLabel,
    message: String,
    actionAdmissible: Boolean,
    onReattach: () -> Unit,
    onSessions: () -> Unit,
) {
    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(Ink.copy(alpha = 0.96f)),
        contentAlignment = Alignment.Center,
    ) {
        Column(
            horizontalAlignment = Alignment.CenterHorizontally,
            modifier = Modifier
                .widthIn(max = 320.dp)
                .padding(24.dp),
        ) {
            Text(
                "Reconnect to ${machineLabel.text}",
                style = MaterialTheme.typography.titleLarge,
                fontWeight = FontWeight.SemiBold,
            )
            Text(message, color = noticeToneColor(NoticeTone.Failure), modifier = Modifier.padding(top = 8.dp))
            Text(
                terminalReconnectSafetyCopy(machineLabel),
                color = Muted,
                modifier = Modifier.padding(top = 8.dp, bottom = 20.dp),
            )
            Button(
                onClick = onReattach,
                enabled = actionAdmissible,
                modifier = Modifier.fillMaxWidth().testTag("terminal-reattach"),
            ) {
                Text("Reattach to ${machineLabel.text}")
            }
            OutlinedButton(
                onClick = onSessions,
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(top = 8.dp),
            ) {
                BackToDwarvesContent(tag = "terminal-dwarves-mark")
            }
        }
    }
}

private fun terminalPresence(state: SkidbladnirUiState.Terminal): String = when (val connection = state.connection) {
    TerminalUiStatus.Verifying -> "Verifying machine and session"
    TerminalUiStatus.Preparing -> "Preparing a fresh attachment"
    TerminalUiStatus.Connecting -> "Connecting"
    is TerminalUiStatus.ReconnectRequired -> "Input frozen"
    is TerminalUiStatus.Connected -> {
        val clients = connection.attachedClients
        "$clients ${if (clients == 1) "client" else "clients"} · ${connection.geometry.name.uppercase()}"
    }
}

private fun terminalPresenceColor(connection: TerminalUiStatus): Color = when (connection) {
    is TerminalUiStatus.Connected -> Moss
    is TerminalUiStatus.ReconnectRequired -> noticeToneColor(NoticeTone.Failure)
    TerminalUiStatus.Preparing, TerminalUiStatus.Verifying, TerminalUiStatus.Connecting -> Gold
}

private fun terminalStatusTag(connection: TerminalUiStatus): String = when (connection) {
    is TerminalUiStatus.Connected -> "terminal-status-connected"
    is TerminalUiStatus.ReconnectRequired -> "terminal-status-reconnect"
    TerminalUiStatus.Preparing -> "terminal-status-preparing"
    TerminalUiStatus.Verifying -> "terminal-status-verifying"
    TerminalUiStatus.Connecting -> "terminal-status-connecting"
}

internal fun terminalReconnectSafetyCopy(machineLabel: MachineLabel): String =
    "${machineLabel.text} terminal is frozen. No input will be replayed."
