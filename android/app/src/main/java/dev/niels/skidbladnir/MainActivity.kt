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
import androidx.compose.material3.Shapes
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.Typography
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp

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
                shapes = NidavellirMaterialShapes,
                typography = NidavellirTypography,
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
        is SkidbladnirUiState.Pairing -> PairingScreen(state, controller)
        is SkidbladnirUiState.Dashboard -> DashboardScreen(state, controller)
        is SkidbladnirUiState.Terminal -> TerminalScreen(state, controller)
    }
}

@Composable
private fun PairingScreen(
    state: SkidbladnirUiState.Pairing,
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
            text = "Pair with the devbox",
            style = MaterialTheme.typography.titleLarge,
            fontWeight = FontWeight.Medium,
        )
        Text(
            text = "Enter the bearer minted on the devbox. It stays encrypted on this phone.",
            color = Muted,
            modifier = Modifier.padding(top = 8.dp, bottom = 16.dp),
        )
        OutlinedTextField(
            value = state.draft,
            onValueChange = controller::updatePairingDraft,
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
            onClick = controller::pair,
            enabled = state.draft.isNotEmpty() && !state.pending,
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
            Text("Connect")
        }
        Text(
            text = "Tailnet only · dev-server-cpx11",
            color = Muted,
            style = MaterialTheme.typography.labelMedium,
            modifier = Modifier.padding(top = 12.dp),
        )
    }
}
