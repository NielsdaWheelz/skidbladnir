package dev.niels.skidbladnir

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.BackHandler
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.systemBarsPadding
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Shapes
import androidx.compose.material3.Surface
import androidx.compose.material3.Typography
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext

private val NidavellirTypography = Typography().let { base ->
    base.copy(
        displayLarge = base.displayLarge.copy(fontFamily = NidavellirType.Display),
        headlineLarge = base.headlineLarge.copy(fontFamily = NidavellirType.Display),
        titleLarge = base.titleLarge.copy(fontFamily = NidavellirType.Display),
    )
}

private val NidavellirMaterialShapes = Shapes(
    small = NidavellirShapes.Chip,
    medium = NidavellirShapes.Card,
    large = NidavellirShapes.Sheet,
)

class MainActivity : ComponentActivity() {
    private lateinit var controller: SkidbladnirController
    private lateinit var scanner: FleetScanner

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        controller = SkidbladnirController(applicationContext)
        scanner = FleetScanner(this)
        enableEdgeToEdge()
        setContent {
            MaterialTheme(
                colorScheme = darkColorScheme(
                    primary = Gold,
                    onPrimary = Ink,
                    secondary = Frost,
                    background = Ink,
                    onBackground = Bone,
                    surface = DeepSurface,
                    onSurface = Bone,
                    surfaceVariant = RaisedSurface,
                    onSurfaceVariant = Muted,
                    // Unread by app code; the slot stays for M3-internal error
                    // state such as text fields (destructive-chrome.md).
                    error = noticeToneColor(NoticeTone.Failure),
                ),
                shapes = NidavellirMaterialShapes,
                typography = NidavellirTypography,
            ) {
                Surface(
                    modifier = Modifier.fillMaxSize(),
                    color = MaterialTheme.colorScheme.background,
                    contentColor = MaterialTheme.colorScheme.onBackground,
                ) {
                    SkidbladnirApp(controller, scanner) { openOrInstallTailscale(this) }
                }
            }
        }
    }

    override fun onStart() {
        super.onStart()
        controller.start()
    }

    override fun onStop() {
        controller.stopForBackground()
        super.onStop()
    }

    override fun onDestroy() {
        controller.close()
        super.onDestroy()
    }
}

@Composable
private fun SkidbladnirApp(
    controller: SkidbladnirController,
    scanner: FleetScanner,
    onTailscale: () -> Unit,
) {
    val state = controller.state
    val context = LocalContext.current
    BackHandler(enabled = state is SkidbladnirUiState.Dashboard && state.forge != null) {
        controller.dismissForge()
    }
    BackHandler(enabled = state is SkidbladnirUiState.Terminal) {
        controller.detachToSessions()
    }
    BackHandler(
        enabled = state is SkidbladnirUiState.FleetConnect && fleetReconnectCanCancel(state),
    ) { controller.cancelFleetReconnect() }
    if (state is SkidbladnirUiState.FleetConnect && state.phase == FleetConnectPhase.Scanning) {
        LaunchedEffect(state) {
            scanner.scan(
                onResult = controller::acceptFleetScan,
                onCancelled = controller::cancelFleetScan,
                onFailure = controller::failFleetScan,
            )
        }
    }
    when (state) {
        SkidbladnirUiState.Booting -> Box(
            modifier = Modifier
                .fillMaxSize()
                .systemBarsPadding(),
            contentAlignment = Alignment.Center,
        ) {
            CircularProgressIndicator()
        }
        is SkidbladnirUiState.FleetConnect -> FleetConnectScreen(
            state = state,
            tailscaleInstalled = tailscaleInstalled(context),
            onConnect = controller::requestFleetScan,
            onTailscale = onTailscale,
        )
        is SkidbladnirUiState.Dashboard -> DashboardScreen(state, controller)
        is SkidbladnirUiState.Terminal -> TerminalScreen(state, controller)
    }
}
