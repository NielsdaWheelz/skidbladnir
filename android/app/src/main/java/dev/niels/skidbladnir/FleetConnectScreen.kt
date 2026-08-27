package dev.niels.skidbladnir

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.systemBarsPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp

internal enum class FleetConnectMode { Install, Reconnect }
internal enum class FleetConnectPhase { Ready, Scanning, Connecting, Failed }

internal data class FleetConnectContent(
    val title: String,
    val body: String,
    val primaryAction: String,
    val externalBoundary: String,
    val progress: String?,
    val failure: String?,
)

internal fun fleetConnectContent(mode: FleetConnectMode, phase: FleetConnectPhase): FleetConnectContent =
    FleetConnectContent(
        title = when (mode) {
            FleetConnectMode.Install -> "Connect your fleet"
            FleetConnectMode.Reconnect -> "Reconnect fleet"
        },
        body = when (mode) {
            FleetConnectMode.Install ->
                "Sign in to Tailscale, then scan a fresh fleet invite from your MacBook."
            FleetConnectMode.Reconnect ->
                "Scan a fresh fleet invite from your MacBook to reconnect the exact installed machines."
        },
        primaryAction = when (mode) {
            FleetConnectMode.Install -> "Connect"
            FleetConnectMode.Reconnect -> "Reconnect fleet"
        },
        externalBoundary = "Skíðblaðnir opens Tailscale but cannot sign in or control the VPN for you.",
        progress = when (phase) {
            FleetConnectPhase.Scanning -> "Scanning a fresh fleet invite…"
            FleetConnectPhase.Connecting -> "Connecting to 3 machines…"
            FleetConnectPhase.Ready, FleetConnectPhase.Failed -> null
        },
        failure = when (phase) {
            FleetConnectPhase.Failed ->
                "Couldn’t connect the whole fleet. Nothing was saved. Create and scan a new fleet invite."
            FleetConnectPhase.Ready, FleetConnectPhase.Scanning, FleetConnectPhase.Connecting -> null
        },
    )

@Composable
internal fun FleetConnectScreen(
    state: SkidbladnirUiState.FleetConnect,
    tailscaleInstalled: Boolean,
    onConnect: () -> Unit,
    onTailscale: () -> Unit,
) {
    val content = fleetConnectContent(state.mode, state.phase)
    val pending = state.phase == FleetConnectPhase.Scanning || state.phase == FleetConnectPhase.Connecting
    Column(
        modifier = Modifier
            .fillMaxSize()
            .systemBarsPadding()
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 24.dp, vertical = 32.dp),
        verticalArrangement = Arrangement.Center,
    ) {
        Text(
            text = "SKÍÐBLAÐNIR",
            style = MaterialTheme.typography.headlineLarge,
            fontWeight = FontWeight.SemiBold,
        )
        Text(
            text = content.title,
            style = MaterialTheme.typography.titleLarge,
            fontWeight = FontWeight.Medium,
            modifier = Modifier.padding(top = 32.dp),
        )
        Text(
            text = content.body,
            color = Muted,
            modifier = Modifier.padding(top = 8.dp),
        )
        if (content.progress != null) {
            CircularProgressIndicator(
                modifier = Modifier
                    .padding(top = 24.dp)
                    .semantics { contentDescription = content.progress.removeSuffix("…") },
                strokeWidth = 2.dp,
            )
            Text(content.progress, modifier = Modifier.padding(top = 12.dp))
        }
        if (content.failure != null) {
            Spacer(Modifier.height(16.dp))
            NoticePanel(tone = NoticeTone.Failure, body = content.failure)
        }
        Spacer(Modifier.height(20.dp))
        Button(
            onClick = onConnect,
            enabled = !pending,
            modifier = Modifier.fillMaxWidth(),
        ) { Text(content.primaryAction) }
        OutlinedButton(
            onClick = onTailscale,
            enabled = !pending,
            modifier = Modifier
                .fillMaxWidth()
                .padding(top = 8.dp),
        ) { Text(if (tailscaleInstalled) "Open Tailscale" else "Install Tailscale") }
        Text(
            text = content.externalBoundary,
            color = Muted,
            style = MaterialTheme.typography.labelMedium,
            modifier = Modifier.padding(top = 12.dp),
        )
    }
}
