package dev.niels.skidbladnir

import androidx.compose.foundation.background
import androidx.compose.foundation.horizontalScroll
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
import androidx.compose.foundation.rememberScrollState
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.key
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.AndroidView

@Composable
internal fun TerminalScreen(
    state: SkidbladnirUiState.Terminal,
    controller: SkidbladnirController,
) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(Ink)
            .systemBarsPadding()
            .imePadding()
            .testTag("terminal-screen-${state.machine.handle.encoded}"),
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 12.dp, vertical = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            TextButton(
                onClick = controller::detachToAgents,
                modifier = Modifier.testTag("terminal-detach"),
            ) { Text(terminalDetachActionLabel()) }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = "${state.machine.label.text} · ${state.target.session.name}",
                    fontWeight = FontWeight.SemiBold,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
                Text(
                    text = "${state.machine.label.text} · ${terminalPresence(state)}",
                    color = terminalPresenceColor(state.connection),
                    style = MaterialTheme.typography.labelSmall,
                    maxLines = 1,
                    modifier = Modifier.testTag(terminalStatusTag(state.connection)),
                )
            }
            TextButton(
                onClick = { controller.requestKill(state.target) },
                enabled = terminalActionAdmissible(state.machineCanMutate, state.connection),
                colors = ButtonDefaults.textButtonColors(contentColor = Ember),
                modifier = Modifier.testTag("terminal-kill"),
            ) {
                Text("Kill")
            }
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
                TerminalUiStatus.Verifying -> TerminalWaiting("Verifying ${state.machine.label.text} and session lifetime…")
                TerminalUiStatus.Preparing -> TerminalWaiting("Preparing terminal…")
                TerminalUiStatus.Connecting -> TerminalWaiting("Connecting…")
                is TerminalUiStatus.Connected -> Unit
                is TerminalUiStatus.ReconnectRequired -> ReconnectPanel(
                    machineLabel = state.machine.label,
                    message = connection.message,
                    actionAdmissible = terminalActionAdmissible(state.machineCanMutate, state.connection),
                    onReattach = controller::reattachTerminal,
                    onAgents = controller::detachToAgents,
                )
            }
        }

        TerminalAccessoryRow(
            enabled = state.connection is TerminalUiStatus.Connected,
            onBytes = { controller.sendTerminal(state.attempt, it) },
            onAccessory = { controller.sendTerminalAccessory(state.attempt, it) },
            onDetach = controller::detachToAgents,
        )
    }

    state.kill?.let { kill ->
        KillConfirmation(
            state = kill,
            actionAdmissible = terminalActionAdmissible(state.machineCanMutate, state.connection),
            onDismiss = controller::dismissKill,
            onConfirm = controller::confirmKill,
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
    onAgents: () -> Unit,
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
            Text(message, color = Ember, modifier = Modifier.padding(top = 8.dp))
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
                onClick = onAgents,
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(top = 8.dp),
            ) {
                Text("Back to Agents")
            }
        }
    }
}

@Composable
private fun TerminalAccessoryRow(
    enabled: Boolean,
    onBytes: (ByteArray) -> Unit,
    onAccessory: (TerminalAccessory) -> Unit,
    onDetach: () -> Unit,
) {
    Surface(color = RaisedSurface, shadowElevation = 6.dp) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .horizontalScroll(rememberScrollState())
                .padding(horizontal = 6.dp, vertical = 6.dp),
            horizontalArrangement = Arrangement.spacedBy(5.dp),
        ) {
            AccessoryButton("Esc", enabled) { onBytes(byteArrayOf(0x1b)) }
            AccessoryButton("Ctrl-C", enabled) { onBytes(byteArrayOf(0x03)) }
            AccessoryButton("Tab", enabled) { onBytes(byteArrayOf(0x09)) }
            AccessoryButton("←", enabled, "Left arrow") { onAccessory(TerminalAccessory.Left) }
            AccessoryButton("↑", enabled, "Up arrow") { onAccessory(TerminalAccessory.Up) }
            AccessoryButton("↓", enabled, "Down arrow") { onAccessory(TerminalAccessory.Down) }
            AccessoryButton("→", enabled, "Right arrow") { onAccessory(TerminalAccessory.Right) }
            AccessoryButton("Home", enabled) { onAccessory(TerminalAccessory.Home) }
            AccessoryButton("End", enabled) { onAccessory(TerminalAccessory.End) }
            AccessoryButton("Newline", enabled, "Insert newline without submitting") { onBytes(byteArrayOf(0x0a)) }
            AccessoryButton(
                terminalDetachActionLabel(),
                enabled = true,
                description = "Detach phone; agent keeps running",
                onClick = onDetach,
            )
        }
    }
}

@Composable
private fun AccessoryButton(
    label: String,
    enabled: Boolean,
    description: String = label,
    onClick: () -> Unit,
) {
    OutlinedButton(
        onClick = onClick,
        enabled = enabled,
        modifier = Modifier.semantics { contentDescription = description },
    ) {
        Text(label, maxLines = 1)
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
    is TerminalUiStatus.ReconnectRequired -> Ember
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

internal fun terminalDetachActionLabel(): String = "Detach · agent keeps running"
