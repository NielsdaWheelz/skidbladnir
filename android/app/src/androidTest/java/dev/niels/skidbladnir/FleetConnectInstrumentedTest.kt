package dev.niels.skidbladnir

import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.test.assertIsEnabled
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onNodeWithText
import org.junit.Rule
import org.junit.Test

class FleetConnectInstrumentedTest {
    @get:Rule val compose = createComposeRule()

    @Test
    fun freshStoreShowsTheFrozenConnectBoundaryWithoutManualCredentials() {
        compose.setContent {
            MaterialTheme {
                FleetConnectScreen(
                    state = SkidbladnirUiState.FleetConnect(FleetConnectMode.Install, FleetConnectPhase.Ready),
                    tailscaleInstalled = true,
                    onConnect = {},
                    onTailscale = {},
                )
            }
        }

        compose.onNodeWithText("Connect your fleet").assertIsDisplayed()
        compose.onNodeWithText("Sign in to Tailscale, then scan a fresh fleet invite from your MacBook.").assertIsDisplayed()
        compose.onNodeWithText("Connect").assertIsDisplayed().assertIsEnabled()
        compose.onNodeWithText("Open Tailscale").assertIsDisplayed().assertIsEnabled()
        compose.onNodeWithText(
            "Skíðblaðnir opens Tailscale but cannot sign in or control the VPN for you.",
        ).assertIsDisplayed()
        compose.onNodeWithText("Bearer").assertDoesNotExist()
        compose.onNodeWithText("Update bearer").assertDoesNotExist()
    }

    @Test
    fun connectingAndFailureStatesNameProgressAndOneRecoveryAction() {
        var phase by mutableStateOf(FleetConnectPhase.Connecting)
        compose.setContent {
            MaterialTheme {
                FleetConnectScreen(
                    state = SkidbladnirUiState.FleetConnect(FleetConnectMode.Install, phase),
                    tailscaleInstalled = false,
                    onConnect = {},
                    onTailscale = {},
                )
            }
        }
        compose.onNodeWithText("Connecting to 3 machines…").assertIsDisplayed()
        compose.onNodeWithText("Connect").assertIsNotEnabled()
        compose.onNodeWithText("Install Tailscale").assertIsNotEnabled()

        compose.runOnIdle { phase = FleetConnectPhase.Failed }
        compose.onNodeWithText(
            "Couldn’t connect the whole fleet. Nothing was saved. Create and scan a new fleet invite.",
        ).assertIsDisplayed()
        compose.onNodeWithText("Connect").assertIsEnabled()
        compose.onNodeWithText("Install Tailscale").assertIsEnabled()
    }
}
