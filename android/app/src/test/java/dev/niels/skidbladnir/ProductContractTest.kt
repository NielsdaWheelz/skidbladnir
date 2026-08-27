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
    fun `session contract requires character and hard-cut tmux names`() {
        val inventory = decodeSessionsResponse(
            """
            {
              "machine":{"handle":"mh-0123456789abcdef0123456789abcdef","platform":"Linux"},
              "observedAt":"2026-08-25T12:00:00Z",
              "profiles":[{"key":"personal","label":"Personal"}],
              "sessions":[
                {
                  "id":"${'$'}1",
                  "tmuxName":"forge",
                  "identityToken":"v1-0123456789abcdef0123456789abcdef.100.200.1",
                  "profile":"personal",
                  "objective":"Inspect the forge",
                  "character":{"key":"norse.durinn","displayName":"Durinn"},
                  "cwd":"/home/niels/src/personal",
                  "activeCommand":"codex",
                  "attachedClients":2,
                  "attention":true,
                  "status":{"kind":"Working","signal":"Lifecycle","signalAt":"2026-08-25T11:59:48Z"}
                },
                {
                  "id":"${'$'}2",
                  "tmuxName":"laptop",
                  "identityToken":"v1-0123456789abcdef0123456789abcdef.100.200.2",
                  "character":{"key":"norse.bifur","displayName":"Bifur"},
                  "attachedClients":1,
                  "attention":false,
                  "status":{"kind":"Shell","signal":"Process","signalAt":"2026-08-25T11:58:00Z"}
                }
              ]
            }
            """.trimIndent(),
        )

        assertEquals("forge", inventory.sessions.first().tmuxName)
        assertEquals("Durinn", inventory.sessions.first().character.displayName)
        assertEquals(SessionStatusKind.Working, inventory.sessions.first().status.kind)
        assertTrue(inventory.sessions.first().attention)
        assertEquals(2, inventory.sessions.first().attachedClients)
        assertNull(inventory.sessions.last().profile)
        assertEquals("Bifur", inventory.sessions.last().character.displayName)
        assertEquals(
            "{\"tmuxName\":\"forge\",\"identityToken\":\"v1-0123456789abcdef0123456789abcdef.100.200.1\"}",
            encodeKillSessionRequest(inventory.sessions.first()),
        )
        assertThrows(ProtocolDecodeException::class.java) {
            decodeAgentSession(
                """{"id":"${'$'}3","tmuxName":"missing-character","identityToken":"token","attachedClients":0,"attention":false,"status":{"kind":"Shell","signal":"Process","signalAt":"2026-08-25T11:58:00Z"}}""",
            )
        }
    }

    @Test
    fun `inventory decoder rejects duplicate picker and session identities`() {
        assertThrows(ProtocolDecodeException::class.java) {
            decodeSessionsResponse(
                """{"machine":{"handle":"mh-0123456789abcdef0123456789abcdef","platform":"Linux"},"observedAt":"2026-08-25T12:00:00Z","profiles":[{"key":"personal","label":"Personal"},{"key":"personal","label":"Work"}],"sessions":[]}""",
            )
        }
        assertThrows(ProtocolDecodeException::class.java) {
            decodeSessionsResponse(
                """{"machine":{"handle":"mh-0123456789abcdef0123456789abcdef","platform":"Linux"},"observedAt":"2026-08-25T12:00:00Z","profiles":[{"key":"personal","label":"Codex"},{"key":"work","label":"Codex"}],"sessions":[]}""",
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
    fun `pressure decoder keeps missing inputs closed and current`() {
        val sample = unknownPressureSample("2026-08-25T12:00:00Z")
        val pressure = decodePressureResponse(
            """{"unsupported":["memoryPressure"],"current":$sample,"history":[$sample]}""",
        )

        assertEquals(listOf(PressureMetric.MemoryAvailablePercent), pressure.current.missing)
        assertEquals(pressure.current, pressure.history.last())
        assertThrows(ProtocolDecodeException::class.java) {
            decodePressureResponse(
                """{"unsupported":["memoryPressure"],"current":$sample,"history":[${sample.replace("memoryAvailablePercent", "temperature")}] }""",
            )
        }
    }

    @Test
    fun `pressure decoder rejects malformed time chronology and missing metric disagreement`() {
        val current = unknownPressureSample("2026-08-25T12:00:00Z")
        val earlier = unknownPressureSample("2026-08-25T11:59:55Z")
        val oldest = unknownPressureSample("2026-08-25T11:59:50Z")

        assertThrows(ProtocolDecodeException::class.java) {
            decodePressureResponse(
                """{"unsupported":["memoryPressure"],"current":$current,"history":[$earlier,$oldest,$current]}""",
            )
        }
        assertThrows(ProtocolDecodeException::class.java) {
            val malformedTime = unknownPressureSample("not-an-instant")
            decodePressureResponse(
                """{"unsupported":["memoryPressure"],"current":$malformedTime,"history":[$malformedTime]}""",
            )
        }
        assertThrows(ProtocolDecodeException::class.java) {
            val disagreement = unknownPressureSample(
                sampledAt = "2026-08-25T12:00:00Z",
                includeMissingMemoryMetric = true,
            )
            decodePressureResponse(
                """{"unsupported":["memoryPressure"],"current":$disagreement,"history":[$disagreement]}""",
            )
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
    fun `terminal detach copy promises the session lifetime rather than the foreground agent`() {
        assertEquals("Detach · session keeps running", terminalDetachActionLabel())
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
                  "profiles":[{"key":"personal","label":"Personal"}],
                  "sessions":[{
                    "id":"${'$'}1",
                    "tmuxName":"laptop",
                    "identityToken":"v1-0123456789abcdef0123456789abcdef.100.200.1",
                    "character":{"key":"norse.durinn","displayName":"Durinn"},
                    "profile":null,
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
                """{"machine":{"handle":"mh-0123456789abcdef0123456789abcdef","platform":"Linux"},"observedAt":"2026-08-25T12:00:00Z","profiles":[],"sessions":[{"id":"${'$'}1","tmuxName":"forge","identityToken":"token","character":{"key":"norse.durinn","displayName":"Durinn"},"attachedClients":1,"attention":false,"status":{"kind":"Working","signal":"Notify","signalAt":"2026-08-25T11:58:00Z"}}]}""",
            )
        }
    }

    @Test
    fun `destructive copy has one owner so the button and the dialog cannot drift`() {
        val label = requireNotNull(MachineLabel.parse("MacBook"))
        val target = AgentTarget(
            machineHandle,
            decodeAgentSession(inventorySession("${'$'}1", "ga-durinn", "token")),
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

    private fun unknownPressureSample(
        sampledAt: String,
        includeMissingMemoryMetric: Boolean = false,
    ): String {
        val memory = if (includeMissingMemoryMetric) "\"memoryAvailablePercent\":42.0," else ""
        return """
            {
              "sampledAt":"$sampledAt",
              "level":"Unknown",
              "reasons":[],
              "metrics":{
                "cpuPercent":12.5,
                "normalizedLoad":0.4,
                $memory
                "swapUsedPercent":0.0,
                "diskAvailablePercent":60.0,
                "cpuPsiSomeAvg60Percent":0.0,
                "memoryPsiFullAvg60Percent":0.0,
                "ioPsiFullAvg60Percent":0.0
              },
              "missing":["memoryAvailablePercent"]
            }
        """.trimIndent()
    }

    private fun inventorySession(id: String, tmuxName: String, identityToken: String): String =
        """{"id":"$id","tmuxName":"$tmuxName","identityToken":"$identityToken","character":{"key":"norse.durinn","displayName":"Durinn"},"attachedClients":1,"attention":false,"status":{"kind":"Shell","signal":"Process","signalAt":"2026-08-25T11:58:00Z"}}"""

    private fun inventoryWithSessions(vararg sessions: String): String =
        """{"machine":{"handle":"mh-0123456789abcdef0123456789abcdef","platform":"Linux"},"observedAt":"2026-08-25T12:00:00Z","profiles":[{"key":"personal","label":"Personal"}],"sessions":[${sessions.joinToString()}]}"""

    private fun dashboardWithForge(forge: ForgeState): SkidbladnirUiState.Dashboard =
        SkidbladnirUiState.Dashboard(
            machines = listOf(MachineState(
                machine = machine,
                access = MachineAccess.Ready,
                inventory = InventoryState.Fresh(InventorySnapshot(SessionsResponse(
                    machine = MachineSummary(machineHandle, MachinePlatform.Linux),
                observedAt = Instant.parse("2026-08-25T12:00:00Z"),
                profiles = listOf(ProfileChoice(personalProfile, "Personal")),
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
