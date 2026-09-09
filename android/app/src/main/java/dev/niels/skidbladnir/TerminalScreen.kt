package dev.niels.skidbladnir

import android.view.WindowInsets as PlatformWindowInsets
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.ime
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.systemBarsPadding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
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
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.LocalView
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.AndroidView

@Composable
internal fun TerminalScreen(
    state: SkidbladnirUiState.Terminal,
    controller: SkidbladnirController,
    onDetach: () -> Unit,
) {
    var modifiers by remember(state.attempt) {
        mutableStateOf(
            TerminalModifiers(
                control = TerminalModifierPhase.Off,
                alt = TerminalModifierPhase.Off,
            ),
        )
    }
    val inputAdmissible = terminalInputAdmissible(state.connection, state.viewport)
    val recovering = terminalPageLive(state.connection) && state.viewport == TerminalViewport.TooSmall

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
            HeaderChip(
                label = "Detach",
                spokenName = null,
                enabled = true,
                onClick = onDetach,
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
            HeaderChip(
                label = "Aa",
                spokenName = "Terminal text size",
                enabled = state.textSize is TerminalTextSizeState.Ready &&
                    terminalPageLive(state.connection),
                onClick = controller::openTextSize,
                modifier = Modifier.testTag("terminal-text-size"),
            )
            KillButton(
                machineLabel = state.machine.machine.label,
                target = state.target,
                enabled = terminalActionAdmissible(state.machine.canMutate, state.connection),
                onClick = { controller.requestKill(state.target) },
                modifier = Modifier.testTag("terminal-kill"),
            )
        }

        // The recovery overlay sits over the terminal area and the key deck
        // together, so it never changes the WebView's measured bounds and its
        // controls stay reachable even at zero terminal height.
        Box(
            modifier = Modifier
                .fillMaxWidth()
                .weight(1f),
        ) {
            Column(modifier = Modifier.fillMaxSize()) {
                Box(
                    modifier = Modifier
                        .fillMaxWidth()
                        .weight(1f),
                ) {
                    val textSize = state.textSize
                    if (state.connection != TerminalUiStatus.Verifying && textSize is TerminalTextSizeState.Ready) {
                        key(state.attempt) {
                            AndroidView(
                                modifier = Modifier.fillMaxSize().testTag("terminal-page"),
                                factory = { context ->
                                    LockedTerminalWebView(
                                        context = context,
                                        nominalTextSizeSp = textSize.nominalSp,
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

                                            override fun onViewportTooSmall() {
                                                controller.viewportTooSmall(state.attempt)
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
                                    view.isEnabled = inputAdmissible
                                    if (!view.isEnabled) view.clearFocus()
                                },
                                onRelease = LockedTerminalWebView::dispose,
                            )
                        }
                    }

                    // A failed preference read blocks initialization until Retry succeeds.
                    if (textSize == TerminalTextSizeState.Unavailable &&
                        state.connection !is TerminalUiStatus.ReconnectRequired
                    ) {
                        TextSizeUnavailable(onRetry = controller::retryTextSizeRead)
                    } else when (val connection = state.connection) {
                        TerminalUiStatus.Verifying -> TerminalWaiting("Verifying ${state.machine.machine.label.text} and session lifetime…")
                        TerminalUiStatus.Preparing -> TerminalWaiting("Preparing terminal…")
                        TerminalUiStatus.Connecting -> if (!recovering) TerminalWaiting("Connecting…")
                        is TerminalUiStatus.Connected -> Unit
                        is TerminalUiStatus.ReconnectRequired -> ReconnectPanel(
                            machineLabel = state.machine.machine.label,
                            message = connection.message,
                            actionAdmissible = terminalActionAdmissible(state.machine.canMutate, state.connection),
                            onReattach = controller::reattachTerminal,
                            onSessions = onDetach,
                        )
                    }
                }

                TerminalKeyDeck(
                    modifiers = modifiers,
                    enabled = inputAdmissible,
                    onAccessory = { controller.sendTerminalAccessory(state.attempt, it) },
                )
            }

            if (recovering) {
                TerminalTooSmall(onTextSize = controller::openTextSize)
            }
        }
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
    terminalTextSizeSheet(state.textSize, state.connection)?.let { textSize ->
        TerminalTextSizeSheet(
            textSize = textSize,
            connection = state.connection,
            onDecrease = controller::decreaseTextSize,
            onIncrease = controller::increaseTextSize,
            onReset = controller::resetTextSize,
            onDismiss = controller::dismissTextSize,
        )
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun TerminalTextSizeSheet(
    textSize: TerminalTextSizeState.Ready,
    connection: TerminalUiStatus,
    onDecrease: () -> Unit,
    onIncrease: () -> Unit,
    onReset: () -> Unit,
    onDismiss: () -> Unit,
) {
    ModalBottomSheet(
        onDismissRequest = onDismiss,
        shape = NidavellirShapes.Sheet,
        containerColor = DeepSurface,
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 20.dp)
                .padding(bottom = 28.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Text(
                text = "Text size",
                style = MaterialTheme.typography.headlineSmall,
                fontFamily = NidavellirType.Display,
                fontWeight = FontWeight.SemiBold,
            )
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(16.dp),
            ) {
                OutlinedButton(
                    onClick = onDecrease,
                    enabled = terminalTextSizeStepAdmissible(textSize, connection, textSize.nominalSp - 1),
                    shape = NidavellirShapes.Chip,
                    modifier = Modifier.semantics { contentDescription = "Decrease terminal text size" },
                ) {
                    Text("−")
                }
                Text(
                    text = textSize.nominalSp.toString(),
                    style = MaterialTheme.typography.headlineMedium,
                    fontFamily = NidavellirType.Data,
                    modifier = Modifier.semantics { contentDescription = "Terminal text size, ${textSize.nominalSp}" },
                )
                OutlinedButton(
                    onClick = onIncrease,
                    enabled = terminalTextSizeStepAdmissible(textSize, connection, textSize.nominalSp + 1),
                    shape = NidavellirShapes.Chip,
                    modifier = Modifier.semantics { contentDescription = "Increase terminal text size" },
                ) {
                    Text("+")
                }
            }
            OutlinedButton(
                onClick = onReset,
                enabled = terminalTextSizeStepAdmissible(textSize, connection, TERMINAL_TEXT_SIZE_DEFAULT_SP),
                shape = NidavellirShapes.Chip,
            ) {
                Text("Reset to $TERMINAL_TEXT_SIZE_DEFAULT_SP")
            }
            if (textSize.write == TerminalTextSizeWrite.Failed) {
                Text("Text size could not be saved.", color = noticeToneColor(NoticeTone.Failure))
            }
            Text("Saved on this phone. Android’s text-size setting also applies.", color = Muted)
            Text("Screen size is shared with other attached terminals.", color = Muted)
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.End,
            ) {
                Button(onClick = onDismiss, shape = NidavellirShapes.Chip) {
                    Text("Done")
                }
            }
        }
    }
}

@Composable
private fun TextSizeUnavailable(onRetry: () -> Unit) {
    Box(
        modifier = Modifier.fillMaxSize(),
        contentAlignment = Alignment.Center,
    ) {
        NoticePanel(tone = NoticeTone.Failure, body = "Text size unavailable.") {
            TextButton(onClick = onRetry) { Text("Retry") }
        }
    }
}

@Composable
private fun TerminalTooSmall(onTextSize: () -> Unit) {
    val imeVisible = WindowInsets.ime.getBottom(LocalDensity.current) > 0
    // The terminal IME is raised by the WebView through the window's insets
    // controller, never by a Compose text-input session, so it is hidden there.
    val view = LocalView.current
    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(Ink.copy(alpha = 0.96f))
            .verticalScroll(rememberScrollState())
            .padding(vertical = 12.dp),
    ) {
        NoticePanel(
            tone = NoticeTone.Armed,
            title = "Terminal needs more room",
            body = "Hide the keyboard, reduce text size, or make the app window larger.",
        ) {
            if (imeVisible) {
                TextButton(
                    onClick = { view.windowInsetsController?.hide(PlatformWindowInsets.Type.ime()) },
                ) { Text("Hide keyboard") }
            }
            TextButton(onClick = onTextSize) { Text("Text size") }
        }
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
        "$clients ${if (clients == 1) "client" else "clients"}"
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
