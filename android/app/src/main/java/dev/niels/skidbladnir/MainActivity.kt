package dev.niels.skidbladnir

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.BackHandler
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.systemBarsPadding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp

internal val Ink = Color(0xFF0C0D0F)
internal val DeepSurface = Color(0xFF15171A)
internal val RaisedSurface = Color(0xFF202329)
internal val Bone = Color(0xFFF3F0E8)
internal val Muted = Color(0xFFAAA69D)
internal val Gold = Color(0xFFD6A85F)
internal val Ember = Color(0xFFE46C55)
internal val Moss = Color(0xFF76B082)
internal val Frost = Color(0xFF78A9C6)

class MainActivity : ComponentActivity() {
    private lateinit var controller: SkidbladnirController

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        controller = SkidbladnirController(applicationContext)
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
                    error = Ember,
                ),
            ) {
                Surface(
                    modifier = Modifier.fillMaxSize(),
                    color = MaterialTheme.colorScheme.background,
                    contentColor = MaterialTheme.colorScheme.onBackground,
                ) {
                    SkidbladnirApp(controller)
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
private fun SkidbladnirApp(controller: SkidbladnirController) {
    val state = controller.state
    BackHandler(enabled = state is SkidbladnirUiState.Dashboard && state.forge != null) {
        controller.dismissForge()
    }
    BackHandler(enabled = state is SkidbladnirUiState.Terminal) {
        controller.detachToAgents()
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
        is SkidbladnirUiState.BearerRepair -> BearerRepairScreen(state, controller)
        is SkidbladnirUiState.Dashboard -> DashboardScreen(state, controller)
        is SkidbladnirUiState.Terminal -> TerminalScreen(state, controller)
    }
}

@Composable
private fun BearerRepairScreen(
    state: SkidbladnirUiState.BearerRepair,
    controller: SkidbladnirController,
) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .systemBarsPadding()
            .imePadding()
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 24.dp, vertical = 32.dp),
        verticalArrangement = Arrangement.Center,
    ) {
        Text(
            text = "Skíðblaðnir",
            style = MaterialTheme.typography.headlineLarge,
            fontWeight = FontWeight.SemiBold,
        )
        Text(
            text = "Your agents, aboard.",
            color = Muted,
            style = MaterialTheme.typography.titleMedium,
        )
        Spacer(Modifier.height(32.dp))
        Text(
            text = "Update ${state.machine.label.text} bearer",
            style = MaterialTheme.typography.titleLarge,
            fontWeight = FontWeight.Medium,
        )
        Text(
            text = "Re-authenticate the existing machine at ${state.machine.origin.encoded}. Its identity and destination stay fixed.",
            color = Muted,
            modifier = Modifier.padding(top = 8.dp, bottom = 16.dp),
        )
        OutlinedTextField(
            value = state.bearer,
            onValueChange = controller::updateBearerRepair,
            modifier = Modifier.fillMaxWidth(),
            enabled = !state.pending,
            label = { Text("Bearer") },
            singleLine = true,
            visualTransformation = PasswordVisualTransformation(),
            keyboardOptions = KeyboardOptions(
                keyboardType = KeyboardType.Password,
                autoCorrectEnabled = false,
            ),
        )
        if (state.error != null) {
            Text(
                text = state.error,
                color = MaterialTheme.colorScheme.error,
                modifier = Modifier.padding(top = 12.dp),
            )
        }
        Button(
            onClick = controller::repairBearer,
            enabled = state.bearer.isNotEmpty() && !state.pending,
            modifier = Modifier
                .fillMaxWidth()
                .padding(top = 20.dp),
        ) {
            if (state.pending) {
                CircularProgressIndicator(
                    modifier = Modifier.size(20.dp),
                    strokeWidth = 2.dp,
                )
                Spacer(Modifier.width(8.dp))
            }
            Text("Update bearer")
        }
        TextButton(onClick = controller::cancelBearerRepair, enabled = !state.pending, modifier = Modifier.fillMaxWidth()) {
            Text("Back to agents")
        }
        Text(
            text = "Tailnet only · fixed machine identity",
            color = Muted,
            style = MaterialTheme.typography.labelMedium,
            modifier = Modifier.padding(top = 12.dp),
        )
    }
}
