package dev.niels.skidbladnir

import java.time.Instant
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Assert.assertThrows
import org.junit.Test

class ProductContractTest {
    @Test
    fun `stored machine origin must already be canonical`() {
        val canonical = "https://arch.example.ts.net:8443/"
        assertEquals(canonical, requireNotNull(parseStoredMachineOrigin(canonical)).encoded)
        assertNull(parseStoredMachineOrigin("https://ARCH.example.ts.net:8443/"))
    }

    @Test
    fun `pairing bearer accepts only canonical 256-bit raw base64url`() {
        val canonical = "A".repeat(43)
        assertTrue(GatewayBearer.parse(canonical)?.encoded == canonical)

        for (invalid in listOf(
            "",
            "A".repeat(42),
            "A".repeat(44),
            "A".repeat(42) + "+",
            "A".repeat(42) + "\n",
            "A".repeat(42) + "B",
        )) {
            assertNull("accepted invalid bearer ${invalid.length}", GatewayBearer.parse(invalid))
        }
    }

    @Test
    fun `inventory decoder rejects duplicate picker and session identities`() {
        assertThrows(ProtocolDecodeException::class.java) {
            decodeSessionsResponse(
                """{"machine":{"handle":"mh-0123456789abcdef0123456789abcdef","platform":"Linux"},"observedAt":"2026-08-25T12:00:00Z","profiles":[{"key":"personal","label":"Personal","provider":"Codex"},{"key":"personal","label":"Work","provider":"Codex"}],"sessions":[]}""",
            )
        }
        assertThrows(ProtocolDecodeException::class.java) {
            decodeSessionsResponse(
                """{"machine":{"handle":"mh-0123456789abcdef0123456789abcdef","platform":"Linux"},"observedAt":"2026-08-25T12:00:00Z","profiles":[{"key":"personal","label":"Codex","provider":"Codex"},{"key":"work","label":"Codex","provider":"Codex"}],"sessions":[]}""",
            )
        }
        val first = inventorySession("${'$'}1", "one", "token")
        val duplicateId = inventorySession("${'$'}1", "two", "other-token")
        val duplicateToken = inventorySession("${'$'}2", "two", "token")
        assertThrows(ProtocolDecodeException::class.java) {
            decodeSessionsResponse(inventoryWithSessions(first, duplicateId))
        }
        assertThrows(ProtocolDecodeException::class.java) {
            decodeSessionsResponse(inventoryWithSessions(first, duplicateToken))
        }
    }

    @Test
    fun `forge request uses the hard-cut optional tmux name`() {
        val emptyOptional = encodeCreateSessionRequest(
            ForgeDraft(machineHandle, cwd = "~/src", profile = workProfile, optionalTmuxName = "", objective = ""),
        )
        val invalidDraft = encodeCreateSessionRequest(
            ForgeDraft(machineHandle, cwd = "~/src", profile = workProfile, optionalTmuxName = "bad name", objective = "\u001b"),
        )

        assertEquals("{\"cwd\":\"~/src\",\"profile\":\"work\"}", emptyOptional)
        assertEquals(
            "{\"cwd\":\"~/src\",\"profile\":\"work\",\"optionalTmuxName\":\"bad name\",\"objective\":\"\\u001b\"}",
            invalidDraft,
        )
    }

    @Test
    fun `pressure decoder exposes typed measured and missing signals with compact history`() {
        val pressure = decodePressureResponse(linuxPressureResponse())

        assertEquals(PressureLevel.Unknown, pressure.current.level)
        assertEquals(PressurePhase.Steady, pressure.current.phase)
        assertEquals(listOf(PressureReason.Disk, PressureReason.Load), pressure.current.reasons)
        assertEquals(
            listOf(
                PressureMetric.CpuPercent,
                PressureMetric.NormalizedLoad,
                PressureMetric.MemoryAvailablePercent,
                PressureMetric.SwapUsedPercent,
                PressureMetric.DiskAvailablePercent,
                PressureMetric.CpuPsiSomeAvg60Percent,
                PressureMetric.MemoryPsiFullAvg60Percent,
                PressureMetric.IoPsiFullAvg60Percent,
            ),
            pressure.current.signals.map(PressureSignal::metric),
        )
        assertEquals(
            PressureSignal.Measured(PressureValue.CpuPercent(12.5), PressureSignalState.Informational),
            pressure.current.signals[0],
        )
        assertEquals(
            PressureSignal.Measured(PressureValue.NormalizedLoad(1.25), PressureSignalState.Warm),
            pressure.current.signals[1],
        )
        assertEquals(
            PressureSignal.Missing(PressureMetric.MemoryAvailablePercent),
            pressure.current.signals[2],
        )
        assertEquals(
            PressureSignal.Measured(PressureValue.DiskAvailablePercent(4.0), PressureSignalState.Hot),
            pressure.current.signals[4],
        )
        assertEquals(
            listOf(
                PressureHistorySample(
                    sampledAt = Instant.parse("2026-08-25T12:00:00Z"),
                    level = PressureLevel.Unknown,
                ),
            ),
            pressure.history,
        )

        val recovering = decodePressureResponse(
            darwinPressureResponse()
                .replace("\"phase\":\"Steady\"", "\"phase\":\"Recovering\"")
                .replace(
                    "\"value\":\"Warning\",\"state\":\"Warm\"",
                    "\"value\":\"Normal\",\"state\":\"Normal\"",
                ),
        )
        assertEquals(PressureLevel.Warm, recovering.current.level)
        assertEquals(PressurePhase.Recovering, recovering.current.phase)
    }

    @Test
    fun `pressure decoder rejects inconsistent signal membership state or causes`() {
        val valid = linuxPressureResponse()
        val invalid = listOf(
            valid.replace(
                "\"cpuPercent\":{\"value\":12.5,\"state\":\"Informational\"}",
                "\"cpuPercent\":null",
            ),
            valid.replace("\"state\":\"Informational\"", "\"state\":\"Normal\""),
            valid.replace(
                "\"normalizedLoad\":{\"value\":1.25,\"state\":\"Warm\"}",
                "\"normalizedLoad\":{\"value\":1.25,\"state\":\"Informational\"}",
            ),
            valid.replace(
                "\"missing\":[\"memoryAvailablePercent\"]",
                "\"missing\":[\"cpuPercent\",\"memoryAvailablePercent\"]",
            ),
            valid.replace("\"phase\":\"Steady\"", "\"phase\":\"Recovering\""),
            valid.replace("\"ioPsiFullAvg60Percent\"", "\"temperature\""),
            darwinPressureResponse().replace(
                "\"value\":\"Warning\",\"state\":\"Warm\"",
                "\"value\":\"Warning\",\"state\":\"Normal\"",
            ),
            darwinPressureResponse().replace("\"reasons\":[\"Memory\"]", "\"reasons\":[\"CpuPsi\"]"),
            darwinPressureResponse().replace(
                "\"unsupported\":[\"cpuPsiSomeAvg60Percent\",\"ioPsiFullAvg60Percent\",\"memoryAvailablePercent\",\"memoryPsiFullAvg60Percent\"]",
                "\"unsupported\":[\"memoryAvailablePercent\",\"cpuPsiSomeAvg60Percent\",\"ioPsiFullAvg60Percent\",\"memoryPsiFullAvg60Percent\"]",
            ),
            valid.replace("\"reasons\":[\"Disk\",\"Load\"]", "\"reasons\":[\"Disk\"]"),
            valid.replace(
                "\"reasons\":[\"Disk\",\"Load\"]",
                "\"reasons\":[\"Memory\",\"Disk\",\"Load\"]",
            ),
            valid.replace("\"reasons\":[\"Disk\",\"Load\"]", "\"reasons\":[\"Load\",\"Disk\"]"),
        )

        invalid.forEachIndexed { index, payload ->
            assertThrows("accepted invalid pressure payload case $index", ProtocolDecodeException::class.java) {
                decodePressureResponse(payload)
            }
        }
    }

    @Test
    fun `pressure decoder rejects malformed or inconsistent compact history`() {
        val valid = linuxPressureResponse()
        val invalid = listOf(
            linuxPressureResponse(
                history =
                    """[{"sampledAt":"2026-08-25T11:59:55Z","level":"Unknown"},{"sampledAt":"2026-08-25T11:59:50Z","level":"Unknown"},{"sampledAt":"2026-08-25T12:00:00Z","level":"Unknown"}]""",
            ),
            valid.replace("2026-08-25T12:00:00Z", "not-an-instant"),
            valid.replace(
                "\"history\":[{\"sampledAt\":\"2026-08-25T12:00:00Z\",\"level\":\"Unknown\"}]",
                "\"history\":[{\"sampledAt\":\"2026-08-25T11:59:55Z\",\"level\":\"Unknown\"}]",
            ),
            valid.replace(
                "\"history\":[{\"sampledAt\":\"2026-08-25T12:00:00Z\",\"level\":\"Unknown\"}]",
                "\"history\":[{\"sampledAt\":\"2026-08-25T12:00:00Z\",\"level\":\"Unknown\",\"phase\":\"Steady\"}]",
            ),
        )

        invalid.forEachIndexed { index, payload ->
            assertThrows("accepted invalid pressure history case $index", ProtocolDecodeException::class.java) {
                decodePressureResponse(payload)
            }
        }
    }

    @Test
    fun `mutation failures distinguish rejection from unknown outcome`() {
        assertTrue(createFailureIsDefinitive(GatewayFailure.Api(ApiErrorCode.SessionNameConflict)))
        assertFalse(createFailureIsDefinitive(GatewayFailure.Api(ApiErrorCode.InternalError)))
        assertFalse(createFailureIsDefinitive(GatewayFailure.Transport))
        assertTrue(killFailureIsDefinitive(GatewayFailure.Api(ApiErrorCode.SessionIdentityMismatch)))
        assertTrue(killFailureIsDefinitive(GatewayFailure.Api(ApiErrorCode.SessionGroupedConflict)))
        assertFalse(killFailureIsDefinitive(GatewayFailure.Api(ApiErrorCode.InternalError)))
        assertFalse(killFailureIsDefinitive(GatewayFailure.Transport))
    }

    @Test
    fun `session identity mismatch recovery copy is action neutral`() {
        assertEquals(
            "rename and kill share one stale-identity outcome, so its public recovery must not " +
                "claim that either destructive action was attempted",
            "The session changed. Refresh and try again.",
            apiErrorMessage(ApiErrorCode.SessionIdentityMismatch),
        )
    }

    @Test
    fun `dashboard reentry preserves rejected Forge draft but never replays pending Start`() {
        val draft = ForgeDraft(
            machineHandle = machineHandle,
            cwd = "bad relative directory",
            profile = personalProfile,
            optionalTmuxName = "bad name",
            objective = "Inspect the forge",
        )
        val rejected = dashboardWithForge(
            ForgeState(form = ForgeForm(draft), pending = false, error = "Choose a valid working directory."),
        )

        val rejectedCarry = forgeCarry(rejected)
        assertTrue(rejectedCarry.forge?.form?.submission() == draft)
        assertEquals("Choose a valid working directory.", rejectedCarry.forge?.error)
        assertNull(rejectedCarry.recovery)

        val pendingCarry = forgeCarry(
            dashboardWithForge(ForgeState(form = ForgeForm(draft), pending = true, error = null)),
        )
        assertNull(pendingCarry.forge)
        assertTrue((pendingCarry.recovery as ForgeRecovery.RefreshRequired).draft == draft)
    }

    @Test
    fun `terminal actions require both fresh machine state and settled attachment`() {
        val connected = TerminalUiStatus.Connected(1, TerminalGeometry.Owner)
        assertFalse(terminalActionAdmissible(machineCanMutate = false, connected))
        assertFalse(terminalActionAdmissible(machineCanMutate = true, TerminalUiStatus.Preparing))
        assertFalse(terminalActionAdmissible(machineCanMutate = true, TerminalUiStatus.Verifying))
        assertFalse(terminalActionAdmissible(machineCanMutate = true, TerminalUiStatus.Connecting))
        assertTrue(terminalActionAdmissible(machineCanMutate = true, connected))
        assertTrue(
            terminalActionAdmissible(
                machineCanMutate = true,
                TerminalUiStatus.ReconnectRequired("Devbox: reconnect required."),
            ),
        )
    }

    @Test
    fun `terminal protocol accepts only the fixed server variants`() {
        assertEquals(
            TerminalServerEvent.Hello(2, TerminalGeometry.Constrained),
            decodeTerminalServerEvent("""{"kind":"Hello","attachedClients":2,"geometry":"Constrained"}"""),
        )
        assertEquals(
            TerminalServerEvent.Presence(1, TerminalGeometry.Owner),
            decodeTerminalServerEvent("""{"kind":"Presence","attachedClients":1,"geometry":"Owner"}"""),
        )
        assertEquals(
            TerminalServerEvent.Error(ApiErrorCode.ReconnectRequired),
            decodeTerminalServerEvent(
                """{"kind":"Error","error":{"code":"ReconnectRequired","message":"Reconnect required."}}""",
            ),
        )
        assertEquals("{\"kind\":\"Resize\",\"columns\":80,\"rows\":24}", encodeTerminalResize(80, 24))
        assertEquals("{\"kind\":\"Detach\"}", encodeTerminalDetach())
    }

    @Test
    fun `terminal upgrade failure preserves owned authentication and machine identity codes`() {
        assertEquals(ApiErrorCode.ReconnectRequired, terminalUpgradeFailureCode(null, null))
        assertEquals(GatewayFailure.Transport, decodeGatewayHttpFailure(502, "upstream unavailable"))
        assertEquals(ApiErrorCode.ReconnectRequired, terminalUpgradeFailureCode(503, "upstream unavailable"))
        assertEquals(
            ApiErrorCode.Unauthenticated,
            terminalUpgradeFailureCode(
                401,
                """{"code":"Unauthenticated","message":"Authentication required."}""",
            ),
        )
        assertEquals(
            ApiErrorCode.MachineIdentityMismatch,
            terminalUpgradeFailureCode(
                409,
                """{"code":"MachineIdentityMismatch","message":"The machine identity changed. Fleet reset is required."}""",
            ),
        )
        assertEquals(MachineAccess.AuthRequired, terminalAccessLoss(ApiErrorCode.Unauthenticated))
        assertEquals(MachineAccess.IdentityChanged, terminalAccessLoss(ApiErrorCode.MachineIdentityMismatch))
        assertEquals(null, terminalAccessLoss(ApiErrorCode.ReconnectRequired))
        assertThrows(ProtocolDecodeException::class.java) {
            terminalUpgradeFailureCode(
                409,
                """{"code":"MachineIdentityMismatch","message":"Reconnect required."}""",
            )
        }
    }

    @Test
    fun `terminal text sizing is UTF-8 exact and bounded before allocation`() {
        assertEquals(1, "a".utf8ByteCountWithin(4))
        assertEquals(3, "北".utf8ByteCountWithin(4))
        assertEquals(4, "🛶".utf8ByteCountWithin(4))
        assertNull("🛶".utf8ByteCountWithin(3))
        assertNull("\uD800".utf8ByteCountWithin(4))
    }

    @Test
    fun `same-system decoder rejects null metadata and widened terminal events`() {
        assertThrows(ProtocolDecodeException::class.java) {
            decodeSessionsResponse(
                """
                {
                  "machine":{"handle":"mh-0123456789abcdef0123456789abcdef","platform":"Linux"},
                  "observedAt":"2026-08-25T12:00:00Z",
                  "profiles":[{"key":"personal","label":"Codex · Personal","provider":"Codex"}],
                  "sessions":[{
                    "tmuxId":"${'$'}1",
                    "tmuxName":"laptop",
                    "identityToken":"v1-0123456789abcdef0123456789abcdef.100.200.1",
                    "character":{"key":"norse.durinn","displayName":"Durinn"},
                    "launchProfile":null,
                    "attachedClients":1,
                    "attention":false,
                    "status":{"kind":"Shell","signal":"Process","signalAt":"2026-08-25T11:58:00Z"}
                  }]
                }
                """.trimIndent(),
            )
        }
        assertThrows(ProtocolDecodeException::class.java) {
            decodeTerminalServerEvent(
                """{"kind":"Presence","attachedClients":1,"geometry":"Owner","extra":true}""",
            )
        }
        assertThrows(ProtocolDecodeException::class.java) {
            decodeTerminalServerEvent(
                """{"kind":"Error","error":{"code":"SessionNotFound","message":"That session no longer exists."}}""",
            )
        }
        assertThrows(ProtocolDecodeException::class.java) {
            decodeSessionsResponse(
                """{"machine":{"handle":"mh-0123456789abcdef0123456789abcdef","platform":"Linux"},"observedAt":"2026-08-25T12:00:00Z","profiles":[],"sessions":[{"tmuxId":"${'$'}1","tmuxName":"forge","identityToken":"token","character":{"key":"norse.durinn","displayName":"Durinn"},"attachedClients":1,"attention":false,"status":{"kind":"Working","signal":"Notify","signalAt":"2026-08-25T11:58:00Z"}}]}""",
            )
        }
    }

    @Test
    fun `destructive copy has one owner so the button and the dialog cannot drift`() {
        val label = requireNotNull(MachineLabel.parse("MacBook"))
        val target = SessionTarget(
            machineHandle,
            decodeTmuxSession(inventorySession("${'$'}1", "ga-durinn", "token")),
        )

        assertEquals(
            "the kill control must speak its tmux session and its machine, not a bare \"Kill\"",
            "Kill ga-durinn on MacBook",
            killActionLabel(label, target),
        )
        assertEquals(
            "the confirmation title is today's shipped string; this delta changes zero copy",
            "Kill ga-durinn on MacBook?",
            killConfirmationTitle(label, target),
        )
        assertEquals(
            "the title must be exactly the action label plus a question mark, or the spoken " +
                "description and the dialog can name different sessions",
            killActionLabel(label, target) + "?",
            killConfirmationTitle(label, target),
        )
    }

    private fun linuxPressureResponse(
        history: String =
            """[{"sampledAt":"2026-08-25T12:00:00Z","level":"Unknown"}]""",
    ): String =
        """
        {
          "unsupported":["memoryPressure"],
          "current":{
            "sampledAt":"2026-08-25T12:00:00Z",
            "level":"Unknown",
            "phase":"Steady",
            "reasons":["Disk","Load"],
            "signals":{
              "cpuPercent":{"value":12.5,"state":"Informational"},
              "normalizedLoad":{"value":1.25,"state":"Warm"},
              "swapUsedPercent":{"value":3.0,"state":"Informational"},
              "diskAvailablePercent":{"value":4.0,"state":"Hot"},
              "cpuPsiSomeAvg60Percent":{"value":0.2,"state":"Normal"},
              "memoryPsiFullAvg60Percent":{"value":0.1,"state":"Normal"},
              "ioPsiFullAvg60Percent":{"value":0.1,"state":"Normal"}
            },
            "missing":["memoryAvailablePercent"]
          },
          "history":$history
        }
        """.trimIndent()

    private fun darwinPressureResponse(): String =
        """{"unsupported":["cpuPsiSomeAvg60Percent","ioPsiFullAvg60Percent","memoryAvailablePercent","memoryPsiFullAvg60Percent"],"current":{"sampledAt":"2026-08-25T12:00:00Z","level":"Warm","phase":"Steady","reasons":["Memory"],"signals":{"cpuPercent":{"value":12.5,"state":"Informational"},"normalizedLoad":{"value":0.4,"state":"Normal"},"swapUsedPercent":{"value":0.0,"state":"Informational"},"diskAvailablePercent":{"value":60.0,"state":"Normal"},"memoryPressure":{"value":"Warning","state":"Warm"}},"missing":[]},"history":[{"sampledAt":"2026-08-25T12:00:00Z","level":"Warm"}]}"""

    private fun inventorySession(tmuxId: String, tmuxName: String, identityToken: String): String =
        """{"tmuxId":"$tmuxId","tmuxName":"$tmuxName","identityToken":"$identityToken","character":{"key":"norse.durinn","displayName":"Durinn"},"attachedClients":1,"attention":false,"status":{"kind":"Shell","signal":"Process","signalAt":"2026-08-25T11:58:00Z"}}"""

    private fun inventoryWithSessions(vararg sessions: String): String =
        """{"machine":{"handle":"mh-0123456789abcdef0123456789abcdef","platform":"Linux"},"observedAt":"2026-08-25T12:00:00Z","profiles":[{"key":"personal","label":"Codex · Personal","provider":"Codex"}],"sessions":[${sessions.joinToString()}]}"""

    private fun dashboardWithForge(forge: ForgeState): SkidbladnirUiState.Dashboard =
        SkidbladnirUiState.Dashboard(
            machines = listOf(MachineState(
                machine = machine,
                access = MachineAccess.Ready,
                inventory = InventoryState.Fresh(InventorySnapshot(SessionsResponse(
                    machine = MachineSummary(machineHandle, MachinePlatform.Linux),
                observedAt = Instant.parse("2026-08-25T12:00:00Z"),
                profiles = listOf(
                    ProfileChoice(personalProfile, "Codex · Personal", AgentProvider.Codex),
                ),
                sessions = emptyList(),
                ), 0)),
                pressure = PressureState.Reading,
            )),
            selectedMachine = machineHandle,
            refreshing = false,
            forge = forge,
            forgeRecovery = null,
            kill = null,
        )

    private companion object {
        val machineHandle = requireNotNull(MachineHandle.parse("mh-0123456789abcdef0123456789abcdef"))
        val personalProfile = requireNotNull(ProfileKey.parse("personal"))
        val workProfile = requireNotNull(ProfileKey.parse("work"))
        val machine = PairedMachine(
            machineHandle,
            requireNotNull(MachineLabel.parse("Devbox")),
            requireNotNull(MachineOrigin.parse("https://devbox.example.ts.net:8443")),
        )
    }
}
