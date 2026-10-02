package dev.niels.skidbladnir

import android.view.WindowInsets as PlatformWindowInsets
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.WindowInsetsSides
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.ime
import androidx.compose.foundation.layout.only
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawing
import androidx.compose.foundation.layout.windowInsetsBottomHeight
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.minimumInteractiveComponentSize
import androidx.compose.material3.rememberModalBottomSheetState
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
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.text.style.TextOverflow
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
    var sessionSheet by remember(state.attempt) { mutableStateOf(false) }
    val inputAdmissible = terminalInputAdmissible(state.connection, state.viewport)
    val recovering = terminalPageLive(state.connection) && state.viewport == TerminalViewport.TooSmall
    val execution = state.machine.executionContext(state.target.session)
    val owner = state.machine.machine.label.text
    // Where typed input runs, which is the rail's one safety fact: an ssh/mosh
    // pane executes on its destination, never on the machine owning the pane.
    val place = when (execution) {
        is ExecutionContext.Local -> owner
        is ExecutionContext.Remote -> "${execution.machine.label.text} via $owner"
        ExecutionContext.RemoteUnknown -> "remote via $owner"
    }
    val presence = terminalPresence(state.connection)

    // The screen is the sole inset owner: the rail pads the status bar and a
    // band under the deck fills the navigation bar or keyboard inset, so both
    // strata reach the top and bottom edges and the terminal keeps the space
    // between. Side insets stay Ink.
    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(Ink)
            .windowInsetsPadding(WindowInsets.safeDrawing.only(WindowInsetsSides.Horizontal)),
    ) {
        TerminalRail(
            name = state.target.session.tmuxName,
            place = place,
            presence = presence,
            presenceColor = terminalPresenceColor(state.connection),
            spokenName = "${state.target.session.tmuxName} on $place, $presence",
            onDetach = onDetach,
            onSession = { sessionSheet = true },
        )

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
                                modifier = Modifier.fillMaxSize(),
                                factory = { context ->
                                    LockedTerminalWebView(
                                        context = context,
                                        nominalTextSizeSp = textSize.nominalSp,
                                        listener = object : TerminalPageListener {
                                            override fun onReady(page: TerminalPage) {
                                                controller.terminalPageReady(state.attempt, page)
                                            }

                                            override fun onOutputApplied(page: TerminalPage) {
                                                controller.terminalOutputPresented(state.attempt, page)
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
                                onRelease = { view ->
                                    controller.terminalPageDisposed(state.attempt, view)
                                    view.dispose()
                                },
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
        // The deck's ground continues under the navigation bar or keyboard.
        // A separate band, not deck padding, so the recovery overlay above
        // stays bounded by the visible area and its actions remain reachable.
        Spacer(
            Modifier
                .fillMaxWidth()
                .background(RaisedSurface)
                .windowInsetsBottomHeight(WindowInsets.safeDrawing),
        )
    }

    if (sessionSheet) {
        TerminalSessionSheet(
            state = state,
            execution = execution,
            place = place,
            presence = presence,
            controller = controller,
            onDismiss = { sessionSheet = false },
        )
    }
    state.close?.let { close ->
        CloseConfirmation(
            state = close,
            actionAdmissible = terminalActionAdmissible(state.machine.canMutate, state.connection),
            onDismiss = controller::dismissClose,
            onConfirm = controller::confirmClose,
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
            onAutomatic = controller::useAutomaticTitle,
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

// The whole top chrome: one stratum, one row. Detach leads; the rest of the
// row is the session's identity and opens the session sheet, which holds
// every rarer fact and action. Nothing here may grow a second row: the
// terminal takes every pixel the rail and the deck leave (terminal-chrome.md).
@Composable
private fun TerminalRail(
    name: String,
    place: String,
    presence: String,
    presenceColor: Color,
    spokenName: String,
    onDetach: () -> Unit,
    onSession: () -> Unit,
) {
    Surface(color = DeepSurface, modifier = Modifier.fillMaxWidth()) {
        Row(
            modifier = Modifier
                .windowInsetsPadding(WindowInsets.safeDrawing.only(WindowInsetsSides.Top))
                .heightIn(min = 48.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Box(
                modifier = Modifier
                    .clickable(
                        interactionSource = remember { MutableInteractionSource() },
                        indication = AngularIndication(NidavellirShapes.Chip),
                        role = Role.Button,
                        onClick = onDetach,
                    )
                    .minimumInteractiveComponentSize()
                    .padding(horizontal = 12.dp),
                contentAlignment = Alignment.Center,
            ) {
                Text("Detach", color = Gold, style = MaterialTheme.typography.labelLarge)
            }
            Row(
                modifier = Modifier
                    .weight(1f)
                    .clickable(
                        interactionSource = remember { MutableInteractionSource() },
                        indication = AngularIndication(NidavellirShapes.Chip),
                        role = Role.Button,
                        onClickLabel = "open session actions",
                        onClick = onSession,
                    )
                    .clearAndSetSemantics {
                        contentDescription = spokenName
                    }
                    .heightIn(min = 48.dp)
                    .padding(start = 4.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Column(Modifier.weight(1f)) {
                    Text(
                        text = name,
                        color = Bone,
                        style = MaterialTheme.typography.bodyMedium,
                        fontFamily = NidavellirType.Data,
                        fontWeight = FontWeight.SemiBold,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                    // The host truncates before presence does, and only an
                    // anomaly spends colour: the resting state stays Muted.
                    Row {
                        Text(
                            text = place,
                            color = Muted,
                            style = MaterialTheme.typography.labelSmall,
                            fontFamily = NidavellirType.Data,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                            modifier = Modifier.weight(1f, fill = false),
                        )
                        Text(
                            text = " · $presence",
                            color = presenceColor,
                            style = MaterialTheme.typography.labelSmall,
                            fontFamily = NidavellirType.Data,
                            maxLines = 1,
                        )
                    }
                }
                Text(
                    text = "⋯",
                    color = Gold,
                    style = MaterialTheme.typography.headlineSmall,
                    modifier = Modifier.padding(horizontal = 16.dp),
                )
            }
        }
    }
}

// Everything the rail does not show. Rows reuse the existing sheets and
// confirmations rather than re-rendering them, so each action keeps exactly
// one owner; this sheet only routes to it.
@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun TerminalSessionSheet(
    state: SkidbladnirUiState.Terminal,
    execution: ExecutionContext,
    place: String,
    presence: String,
    controller: SkidbladnirController,
    onDismiss: () -> Unit,
) {
    val owner = state.machine.machine.label.text
    val session = state.target.session
    val notification = state.machine.notifications[NotificationKey(state.target)] ?: NotificationPresentation()
    val status = sessionStatusContent(session, state.machine.canMutate, notification)
    val actionAdmissible = terminalActionAdmissible(state.machine.canMutate, state.connection)
    // A reconciling rename keeps `rename` set after its sheet hides; every row
    // whose controller entry would then refuse is disabled, never silently inert.
    val sheetFree = state.close == null && state.rename == null
    val renameEnabled = sheetFree && actionAdmissible
    val actionsEnabled = !state.terminalControlPending && sheetFree && actionAdmissible
    val shellEnabled = !state.shellPending && sheetFree && actionAdmissible
    val textSize = state.textSize
    val context = when (execution) {
        is ExecutionContext.Local -> null
        is ExecutionContext.Remote -> listOfNotNull(
            "running on ${execution.machine.label.text}",
            execution.agent?.let { remoteAgentLabel(execution, state.machines) },
            "terminal on $owner",
        ).joinToString(" · ")
        ExecutionContext.RemoteUnknown -> "remote context unknown · terminal on $owner"
    }
    // An unknown remote context has no directory claim to make, not a missing one.
    val directory = when (execution) {
        is ExecutionContext.Local -> "directory ${execution.cwd ?: "unavailable"}"
        is ExecutionContext.Remote -> "directory ${execution.cwd ?: "unavailable"}"
        ExecutionContext.RemoteUnknown -> null
    }
    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true),
        shape = NidavellirShapes.Sheet,
        containerColor = DeepSurface,
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .verticalScroll(rememberScrollState())
                // 4dp here plus each row's own 16dp puts every label, fact and
                // action on the sheets' common 20dp text edge.
                .padding(horizontal = 4.dp)
                .padding(bottom = 28.dp),
            verticalArrangement = Arrangement.spacedBy(4.dp),
        ) {
            Text(
                text = session.tmuxName,
                style = MaterialTheme.typography.titleMedium,
                fontFamily = NidavellirType.Data,
                fontWeight = FontWeight.SemiBold,
                modifier = Modifier.padding(horizontal = 16.dp).semantics { heading() },
            )
            Text(
                text = buildAnnotatedString {
                    append("$place · ")
                    withStyle(SpanStyle(color = terminalPresenceColor(state.connection))) { append(presence) }
                },
                color = Muted,
                style = MaterialTheme.typography.labelMedium,
                fontFamily = NidavellirType.Data,
                modifier = Modifier.padding(horizontal = 16.dp),
            )
            Text(
                text = listOfNotNull(status.label, status.detail, status.evidence).joinToString(" · "),
                color = sessionStatusColor(status.tone),
                style = MaterialTheme.typography.labelMedium,
                fontFamily = NidavellirType.Data,
                modifier = Modifier
                    .padding(horizontal = 16.dp)
                    .padding(top = 8.dp)
                    .clearAndSetSemantics { contentDescription = status.accessibilityLabel },
            )
            listOfNotNull(
                context,
                directory,
                "recorded native conversation; may differ from terminal".takeIf { session.conversation != null },
                status.secondary,
            ).forEach { line ->
                Text(line, color = Muted, style = MaterialTheme.typography.labelMedium, fontFamily = NidavellirType.Data,
                    modifier = Modifier.padding(horizontal = 16.dp))
            }
            Column(Modifier.padding(top = 12.dp)) {
                // The card's lifetime verbs follow the session's own, through
                // the card's rows (SessionActions.kt), so order, labels and
                // tone cannot drift between the two surfaces.
                SessionActionRows(
                    listOf(
                        SessionAction("rename", renameEnabled, destructive = false, controller::openRename),
                        SessionAction("new terminal on $owner", shellEnabled, destructive = false,
                            if (session.connection == null) controller::newTerminalHere else controller::openSourceTerminalForge),
                        SessionAction(
                            if (textSize is TerminalTextSizeState.Ready) "text size · ${textSize.nominalSp}" else "text size",
                            sheetFree && textSize is TerminalTextSizeState.Ready && terminalPageLive(state.connection),
                            destructive = false,
                            controller::openTextSize,
                        ),
                    ) + terminalLifetimeActions(
                        actionsEnabled,
                        onStop = { controller.stopTerminal(state.target) },
                        onTerminalClose = { controller.requestTerminalClose(state.target) },
                        onClose = { controller.requestClose(state.target) },
                    ),
                    dismiss = onDismiss,
                )
            }
        }
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
                modifier = Modifier.fillMaxWidth(),
            ) {
                Text("Reattach to ${machineLabel.text}")
            }
            OutlinedButton(
                onClick = onSessions,
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(top = 8.dp),
            ) {
                BackToDwarvesContent()
            }
        }
    }
}

// Terse because the terminal area's own overlays carry the full sentences.
private fun terminalPresence(connection: TerminalUiStatus): String = when (connection) {
    TerminalUiStatus.Verifying -> "verifying"
    TerminalUiStatus.Preparing -> "preparing"
    TerminalUiStatus.Connecting -> "connecting"
    is TerminalUiStatus.ReconnectRequired -> "input frozen"
    is TerminalUiStatus.Connected -> {
        val clients = connection.attachedClients
        "$clients ${if (clients == 1) "client" else "clients"}"
    }
}

// Transit is absence, not an armed recovery, so only frozen input is coloured.
private fun terminalPresenceColor(connection: TerminalUiStatus): Color = when (connection) {
    is TerminalUiStatus.ReconnectRequired -> noticeToneColor(NoticeTone.Failure)
    is TerminalUiStatus.Connected, TerminalUiStatus.Preparing, TerminalUiStatus.Verifying,
    TerminalUiStatus.Connecting -> Muted
}

internal fun terminalReconnectSafetyCopy(machineLabel: MachineLabel): String =
    "${machineLabel.text} terminal is frozen. No input will be replayed."
