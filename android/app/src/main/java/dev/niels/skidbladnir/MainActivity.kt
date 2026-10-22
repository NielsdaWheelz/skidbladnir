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
import androidx.compose.material3.Surface
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import kotlinx.coroutines.launch

class MainActivity : ComponentActivity() {
    private lateinit var dashboardEntry: DashboardEntryState
    private lateinit var controller: SkidbladnirController
    private lateinit var scanner: FleetScanner
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
        owner.scope.launch { owner.handled(captured.slot, captured.episode) }
        controller.openNotificationReference(captured.ref)
        // Intent consumption prevents task recreation from replaying a prior click.
        intent.action = Intent.ACTION_MAIN
        intent.removeExtra("skid.reference")
    }

    override fun onResume() {
        super.onResume()
        applicationContext.notificationOwner().refresh()
        NotificationEnrollment.schedule(this, androidx.work.ExistingWorkPolicy.KEEP)
    }

    internal fun notificationSetup() {
        val owner = applicationContext.notificationOwner()
        val view = owner.view.value
        val unpaired = MachineStore(this).read().credentials.isEmpty()
        val instruction = when {
            view.health == NotificationHealth.Disabled -> "background push is optional. session management works without firebase."
            view.health == NotificationHealth.RestartRequired -> "stop skid in android app settings, then reopen it. your sessions stay installed."
            view.health == NotificationHealth.Unconfigured -> "install the matching private android-push-credentials.json on the hub, then apply."
            unpaired -> "connect this phone to a fleet to receive notifications."
            view.health == NotificationHealth.PermissionBlocked -> "open notification settings to allow needs-input alerts."
            else -> "keep tailscale connected, then retry notification setup."
        }
        val dialog = android.app.AlertDialog.Builder(this)
            .setTitle("notifications")
            .setMessage("${notificationHealthMessage(view.health)}\n\n$instruction")
            .setNeutralButton("open notification settings") { _, _ -> startActivity(owner.settingsIntent()) }
            .setNegativeButton("close", null)
        when {
            view.health in setOf(NotificationHealth.Disabled, NotificationHealth.RestartRequired, NotificationHealth.Unconfigured) -> Unit
            unpaired -> dialog.setPositiveButton("reconnect fleet") { _, _ ->
                controller.requestFleetReconnect()
            }
            else -> dialog.setPositiveButton("set up notifications") { _, _ ->
                if (MachineStore(this).read().credentials.isEmpty()) {
                    controller.requestFleetReconnect()
                } else {
                    NotificationEnrollment.schedule(this, androidx.work.ExistingWorkPolicy.REPLACE)
                    notificationPermission.launch(Manifest.permission.POST_NOTIFICATIONS)
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
    BackHandler(enabled = state is SkidbladnirUiState.Terminal, onBack = controller::detachToSessions)
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
        is SkidbladnirUiState.Dashboard -> DashboardScreen(
            state = state,
            entry = dashboardEntry,
            controller = controller,
            onOpenTerminal = controller::openTerminal,
            onNotificationSetup = onNotificationSetup,
        )
        is SkidbladnirUiState.Terminal -> TerminalScreen(
            state = state,
            controller = controller,
            onDetach = controller::detachToSessions,
        )
    }
}
