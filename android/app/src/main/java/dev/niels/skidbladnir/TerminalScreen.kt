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
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.key
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.AndroidView

@Composable
internal fun TerminalScreen(
    state: SkidbladnirUiState.Terminal,
    controller: SkidbladnirController,
) {
    var controlState by remember(state.attempt) { mutableStateOf(TerminalControlState.Off) }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(Ink)
            .systemBarsPadding()
            .imePadding(),
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
                modifier = Modifier.semantics {
                    contentDescription = RETURN_TO_AGENTS_DESCRIPTION
                },
            ) {
                Text("Agents")
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = state.session.tmuxName,
                    fontWeight = FontWeight.SemiBold,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
                Text(
                    text = terminalPresence(state),
                    color = terminalPresenceColor(state.connection),
                    style = MaterialTheme.typography.labelSmall,
                    maxLines = 1,
                )
            }
            TextButton(
                onClick = { controller.requestKill(state.session) },
                colors = ButtonDefaults.textButtonColors(contentColor = Ember),
            ) {
                Text("Kill")
            }
        }

        Box(
            modifier = Modifier
                .fillMaxWidth()
                .weight(1f),
        ) {
            key(state.attempt) {
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

                                override fun onControlStateChanged(newState: TerminalControlState) {
                                    controlState = newState
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
                TerminalUiStatus.Preparing -> TerminalWaiting("Preparing terminal…")
                TerminalUiStatus.Connecting -> TerminalWaiting("Connecting…")
                is TerminalUiStatus.Connected -> Unit
                is TerminalUiStatus.ReconnectRequired -> ReconnectPanel(
                    message = connection.message,
                    onReattach = controller::reattachTerminal,
                    onAgents = controller::detachToAgents,
                )
            }
        }

        TerminalKeyDeck(
            controlState = controlState,
            enabled = state.connection is TerminalUiStatus.Connected,
            onAccessory = { controller.sendTerminalAccessory(state.attempt, it) },
        )
    }

    state.kill?.let { kill -> KillConfirmation(kill, controller::dismissKill, controller::confirmKill) }
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
    message: String,
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
            Text("Reconnect required", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.SemiBold)
            Text(message, color = Ember, modifier = Modifier.padding(top = 8.dp))
            Text(
                "The terminal is frozen. No input will be replayed.",
                color = Muted,
                modifier = Modifier.padding(top = 8.dp, bottom = 20.dp),
            )
            Button(onClick = onReattach, modifier = Modifier.fillMaxWidth()) { Text("Reattach fresh") }
            OutlinedButton(
                onClick = onAgents,
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(top = 8.dp)
                    .semantics {
                        contentDescription = RETURN_TO_AGENTS_DESCRIPTION
                    },
            ) {
                Text("Agents")
            }
        }
    }
}

private fun terminalPresence(state: SkidbladnirUiState.Terminal): String = when (val connection = state.connection) {
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
    TerminalUiStatus.Preparing, TerminalUiStatus.Connecting -> Gold
}

private const val RETURN_TO_AGENTS_DESCRIPTION = "Return to Agents; session keeps running"
