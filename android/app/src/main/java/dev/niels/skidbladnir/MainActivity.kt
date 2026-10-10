package dev.niels.skidbladnir

import android.os.Bundle
import android.content.Intent
import android.Manifest
import androidx.activity.ComponentActivity
import androidx.activity.SystemBarStyle
import androidx.activity.result.contract.ActivityResultContracts
import androidx.activity.compose.BackHandler
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
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
import kotlinx.coroutines.launch
import org.unifiedpush.android.connector.UnifiedPush

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
    private lateinit var dashboardEntry: DashboardEntryState
    private lateinit var controller: SkidbladnirController
    private lateinit var scanner: FleetScanner
    private var resumed = false
    private val notificationPermission = registerForActivityResult(ActivityResultContracts.RequestPermission()) {
        applicationContext.notificationOwner().refresh()
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        dashboardEntry = DashboardEntryState()
        dashboardEntry.install(savedStateRegistry)
        controller = SkidbladnirController(applicationContext, dashboardEntry)
        scanner = FleetScanner(this)
        if (savedInstanceState == null) acceptNotificationClick(intent)
        // The app is dark whatever the system theme, so the system bars always
        // draw light content over the strata painted beneath them.
        enableEdgeToEdge(
            statusBarStyle = SystemBarStyle.dark(android.graphics.Color.TRANSPARENT),
            navigationBarStyle = SystemBarStyle.dark(android.graphics.Color.TRANSPARENT),
        )
        setContent {
            NidavellirTheme {
                Surface(
                    modifier = Modifier.fillMaxSize(),
                    color = MaterialTheme.colorScheme.background,
                    contentColor = MaterialTheme.colorScheme.onBackground,
                ) {
                    SkidbladnirApp(controller, dashboardEntry, scanner,
                        onTailscale = { openOrInstallTailscale(this) },
                        onNotificationSetup = ::notificationSetup,
                    )
                }
            }
        }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        acceptNotificationClick(intent)
    }

    private fun acceptNotificationClick(intent: Intent) {
        val captured = notificationClick(intent) ?: return
        val owner = applicationContext.notificationOwner()
        owner.scope.launch { owner.dismiss(captured.slot, captured.episode, captured.epoch) }
        controller.openNotificationReference(captured.ref)
        // Intent consumption prevents task recreation from replaying a prior click.
        intent.action = Intent.ACTION_MAIN
        intent.removeExtra("skid.reference")
    }

    override fun onResume() {
        super.onResume()
        resumed = true
        val owner = applicationContext.notificationOwner()
        owner.activityFocus(true, hasWindowFocus())
        owner.refresh()
        if (owner.view.value.device.config != null && UnifiedPush.getSavedDistributor(this) == "io.heckel.ntfy") {
            NotificationEnrollment.schedule(this, androidx.work.ExistingWorkPolicy.KEEP)
        }
    }

    override fun onPause() {
        resumed = false
        applicationContext.notificationOwner().activityFocus(false, false)
        super.onPause()
    }

    override fun onWindowFocusChanged(hasFocus: Boolean) {
        super.onWindowFocusChanged(hasFocus)
        applicationContext.notificationOwner().activityFocus(resumed, hasFocus)
    }

    internal fun notificationSetup() {
        val owner = applicationContext.notificationOwner()
        val view = owner.view.value
        val instruction = when {
            view.device.config == null -> "reconnect the fleet to load its private notification configuration."
            view.health == NotificationHealth.ResetRequired ->
                "reset notification memory to adopt the current observer. fleet pairings stay installed."
            view.health == NotificationHealth.SetupRequired || view.health == NotificationHealth.DistributorMissing ->
                "configure ntfy with the fleet’s private server. keep ntfy and tailscale running in the background."
            else -> "keep ntfy and tailscale configured and running in the background."
        }
        val dialog = android.app.AlertDialog.Builder(this)
            .setTitle("notifications")
            .setMessage("${notificationHealthMessage(view.health)}\n\n$instruction")
            .setNeutralButton("open notification settings") { _, _ -> startActivity(owner.settingsIntent()) }
            .setNegativeButton("close", null)
        when {
            view.device.config == null -> dialog.setPositiveButton("reconnect fleet") { _, _ ->
                controller.requestFleetReconnect()
            }
            view.health == NotificationHealth.ResetRequired -> dialog.setPositiveButton("reset notification memory") { _, _ ->
                owner.scope.launch { owner.reset() }
            }
            else -> dialog.setPositiveButton("set up notifications") { _, _ ->
                if (owner.view.value.device.config == null) {
                    controller.requestFleetReconnect()
                } else if (UnifiedPush.getDistributors(this).contains("io.heckel.ntfy")) {
                    UnifiedPush.saveDistributor(this, "io.heckel.ntfy")
                    NotificationEnrollment.schedule(this, androidx.work.ExistingWorkPolicy.REPLACE)
                    notificationPermission.launch(Manifest.permission.POST_NOTIFICATIONS)
                } else {
                    startActivity(Intent(Intent.ACTION_VIEW, android.net.Uri.parse("https://ntfy.sh/docs/subscribe/phone/")))
                }
            }
        }
        dialog.show()
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
internal fun DashboardTerminalHost(
    state: SkidbladnirUiState.Workspace,
    entry: DashboardEntryState,
    controller: SkidbladnirController,
    onOpenTerminal: (SessionTarget) -> Unit,
    onDetach: () -> Unit,
    onNotificationSetup: () -> Unit,
) {
    val terminalVisible = when (state) {
        is SkidbladnirUiState.Dashboard -> false
        is SkidbladnirUiState.Terminal -> true
    }
    BackHandler(enabled = terminalVisible, onBack = onDetach)
    when (state) {
        is SkidbladnirUiState.Dashboard -> DashboardScreen(
            state = state,
            entry = entry,
            controller = controller,
            onOpenTerminal = onOpenTerminal,
            onNotificationSetup = onNotificationSetup,
        )
        is SkidbladnirUiState.Terminal -> TerminalScreen(
            state = state,
            controller = controller,
            onDetach = onDetach,
        )
    }
}

@Composable
internal fun NidavellirTheme(content: @Composable () -> Unit) {
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
        content = content,
    )
}

@Composable
internal fun SkidbladnirApp(
    controller: SkidbladnirController,
    dashboardEntry: DashboardEntryState,
    scanner: FleetScanner,
    onTailscale: () -> Unit,
    onNotificationSetup: () -> Unit,
) {
    val state = controller.state
    val context = LocalContext.current
    BackHandler(enabled = state is SkidbladnirUiState.Dashboard && state.forge != null) {
        controller.dismissForge()
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
        is SkidbladnirUiState.Workspace -> DashboardTerminalHost(
            state = state,
            entry = dashboardEntry,
            controller = controller,
            onOpenTerminal = controller::openTerminal,
            onDetach = controller::detachToSessions,
            onNotificationSetup = onNotificationSetup,
        )
    }
}
