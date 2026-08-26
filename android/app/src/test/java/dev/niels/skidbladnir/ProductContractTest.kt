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
    fun `pairing bearer accepts only canonical 256-bit raw base64url`() {
        val canonical = "A".repeat(43)
        assertEquals(canonical, GatewayBearer.parse(canonical)?.encoded)

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
    fun `inventory decoder preserves observed facts and optional metadata`() {
        val inventory = decodeSessionsResponse(
            """
            {
              "observedAt":"2026-08-25T12:00:00Z",
              "profiles":[{"key":"personal","label":"Personal"}],
              "sessions":[
                {
                  "id":"${'$'}1",
                  "name":"ga-durinn",
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
                  "name":"laptop",
                  "identityToken":"v1-0123456789abcdef0123456789abcdef.100.200.2",
                  "attachedClients":1,
                  "attention":false,
                  "status":{"kind":"Shell","signal":"Process","signalAt":"2026-08-25T11:58:00Z"}
                }
              ]
            }
            """.trimIndent(),
        )

        assertEquals("Durinn", inventory.sessions.first().character?.displayName)
        assertEquals(SessionStatusKind.Working, inventory.sessions.first().status.kind)
        assertTrue(inventory.sessions.first().attention)
        assertEquals(2, inventory.sessions.first().attachedClients)
        assertNull(inventory.sessions.last().profile)
        assertNull(inventory.sessions.last().character)
    }

    @Test
    fun `inventory decoder rejects duplicate picker and session identities`() {
        assertThrows(ProtocolDecodeException::class.java) {
            decodeSessionsResponse(
                """{"observedAt":"2026-08-25T12:00:00Z","profiles":[{"key":"personal","label":"Personal"},{"key":"personal","label":"Work"}],"sessions":[]}""",
            )
        }
        assertThrows(ProtocolDecodeException::class.java) {
            decodeSessionsResponse(
                """{"observedAt":"2026-08-25T12:00:00Z","profiles":[{"key":"personal","label":"Codex"},{"key":"work","label":"Codex"}],"sessions":[]}""",
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
    fun `forge request omits empty optional drafts but preserves invalid nonempty drafts`() {
        val emptyOptional = encodeCreateSessionRequest(
            ForgeDraft(cwd = "~/src", profile = "work", optionalName = "", objective = ""),
        )
        val invalidDraft = encodeCreateSessionRequest(
            ForgeDraft(cwd = "~/src", profile = "work", optionalName = "bad name", objective = "\u001b"),
        )

        assertFalse(emptyOptional.contains("optionalName"))
        assertFalse(emptyOptional.contains("objective"))
        assertTrue(invalidDraft.contains("\"optionalName\":\"bad name\""))
        assertTrue(invalidDraft.contains("\"objective\":\"\\u001b\""))
    }

    @Test
    fun `dashboard uses declared profile labels and preserves selected profile keys`() {
        val profiles = listOf(ProfileChoice("claude-work", "Claude · Work"))
        val session = AgentSession(
            id = "${'$'}1",
            name = "ga-durinn",
            identityToken = "token",
            profile = "claude-work",
            activeCommand = "claude",
            attachedClients = 1,
            attention = false,
            status = SessionStatus(
                kind = SessionStatusKind.Running,
                signal = SessionStatusSignal.Process,
                signalAt = "2026-08-25T12:00:00Z",
            ),
        )

        assertEquals(
            "known cards should render the declared label before the separately observed command",
            listOf("Claude · Work", "claude"),
            agentCardRuntimeFacts(session, profiles),
        )
        assertEquals(
            "unlisted profile keys should stay visibly unknown",
            listOf("profile unknown", "claude"),
            agentCardRuntimeFacts(session.copy(profile = "unlisted"), profiles),
        )
        assertEquals(
            "Forge should submit the selected profile key without provider fields",
            "{\"cwd\":\"~/src\",\"profile\":\"claude-work\"}",
            encodeCreateSessionRequest(
                ForgeDraft(cwd = "~/src", profile = "claude-work", optionalName = "", objective = ""),
            ),
        )
    }

    @Test
    fun `status content names lifecycle signal and age`() {
        val content = statusContent(
            status = SessionStatus(
                kind = SessionStatusKind.Working,
                signal = SessionStatusSignal.Lifecycle,
                signalAt = "2026-08-25T11:59:48Z",
            ),
            now = Instant.parse("2026-08-25T12:00:00Z"),
        )

        assertEquals("WORKING", content.kind)
        assertEquals("lifecycle · 12s", content.evidence)
        assertEquals("Observed working from lifecycle 12 seconds ago", content.accessibilityLabel)
    }

    @Test
    fun `pressure decoder keeps missing inputs closed and current`() {
        val sample = unknownPressureSample("2026-08-25T12:00:00Z")
        val pressure = decodePressureResponse("""{"current":$sample,"history":[$sample]}""")

        assertEquals(listOf(PressureMetric.MemoryAvailablePercent), pressure.current.missing)
        assertEquals(pressure.current, pressure.history.last())
        assertThrows(ProtocolDecodeException::class.java) {
            decodePressureResponse(
                """{"current":$sample,"history":[${sample.replace("memoryAvailablePercent", "temperature")}] }""",
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
                """{"current":$current,"history":[$earlier,$oldest,$current]}""",
            )
        }
        assertThrows(ProtocolDecodeException::class.java) {
            val malformedTime = unknownPressureSample("not-an-instant")
            decodePressureResponse("""{"current":$malformedTime,"history":[$malformedTime]}""")
        }
        assertThrows(ProtocolDecodeException::class.java) {
            val disagreement = unknownPressureSample(
                sampledAt = "2026-08-25T12:00:00Z",
                includeMissingMemoryMetric = true,
            )
            decodePressureResponse("""{"current":$disagreement,"history":[$disagreement]}""")
        }
    }

    @Test
    fun `all gateway errors have fixed literal product messages`() {
        assertEquals("Authentication required.", apiErrorMessage(ApiErrorCode.Unauthenticated))
        assertEquals("The request is not valid.", apiErrorMessage(ApiErrorCode.InvalidRequest))
        assertEquals("The request is too large.", apiErrorMessage(ApiErrorCode.RequestTooLarge))
        assertEquals("Choose a valid working directory.", apiErrorMessage(ApiErrorCode.WorkingDirectoryInvalid))
        assertEquals(
            "That directory does not exist or cannot be opened.",
            apiErrorMessage(ApiErrorCode.WorkingDirectoryUnavailable),
        )
        assertEquals("Choose an available profile.", apiErrorMessage(ApiErrorCode.ProfileUnknown))
        assertEquals(
            "Use 1–64 letters, numbers, underscores, or hyphens, beginning with a letter or number.",
            apiErrorMessage(ApiErrorCode.SessionNameInvalid),
        )
        assertEquals("Use 1–240 characters without terminal controls.", apiErrorMessage(ApiErrorCode.ObjectiveInvalid))
        assertEquals("A session with that name already exists.", apiErrorMessage(ApiErrorCode.SessionNameConflict))
        assertEquals("That session no longer exists.", apiErrorMessage(ApiErrorCode.SessionNotFound))
        assertEquals(
            "The session changed. Refresh before killing it.",
            apiErrorMessage(ApiErrorCode.SessionIdentityMismatch),
        )
        assertEquals(
            "This session shares its work with another non-phone tmux session. Resolve the group in tmux before killing it.",
            apiErrorMessage(ApiErrorCode.SessionGroupedConflict),
        )
        assertEquals("Skíðblaðnir could not complete the request.", apiErrorMessage(ApiErrorCode.InternalError))
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
    fun `pressure authentication loss requires pairing instead of publishing inventory`() {
        assertTrue(
            pressurePollRequiresPairing(
                GatewayResult.Failure(GatewayFailure.Api(ApiErrorCode.Unauthenticated)),
            ),
        )
        assertFalse(
            pressurePollRequiresPairing(
                GatewayResult.Failure(GatewayFailure.Api(ApiErrorCode.InternalError)),
            ),
        )
        assertFalse(pressurePollRequiresPairing(GatewayResult.Failure(GatewayFailure.Transport)))
    }

    @Test
    fun `dashboard reentry preserves rejected Forge draft but never replays pending Start`() {
        val draft = ForgeDraft(
            cwd = "bad relative directory",
            profile = "personal",
            optionalName = "bad name",
            objective = "Inspect the forge",
        )
        val rejected = dashboardWithForge(
            ForgeState(draft = draft, pending = false, error = "Choose a valid working directory."),
        )

        val rejectedCarry = forgeCarry(rejected)
        assertEquals(draft, rejectedCarry.forge?.draft)
        assertEquals("Choose a valid working directory.", rejectedCarry.forge?.error)
        assertNull(rejectedCarry.recovery)

        val pendingCarry = forgeCarry(
            dashboardWithForge(ForgeState(draft = draft, pending = true, error = null)),
        )
        assertNull(pendingCarry.forge)
        assertEquals(draft, (pendingCarry.recovery as ForgeRecovery.RefreshRequired).draft)
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
                  "observedAt":"2026-08-25T12:00:00Z",
                  "profiles":[{"key":"personal","label":"Personal"}],
                  "sessions":[{
                    "id":"${'$'}1",
                    "name":"laptop",
                    "identityToken":"v1-0123456789abcdef0123456789abcdef.100.200.1",
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
                """{"observedAt":"2026-08-25T12:00:00Z","profiles":[],"sessions":[{"id":"${'$'}1","name":"ga-durinn","identityToken":"token","attachedClients":1,"attention":false,"status":{"kind":"Working","signal":"Notify","signalAt":"2026-08-25T11:58:00Z"}}]}""",
            )
        }
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

    private fun inventorySession(id: String, name: String, identityToken: String): String =
        """{"id":"$id","name":"$name","identityToken":"$identityToken","attachedClients":1,"attention":false,"status":{"kind":"Shell","signal":"Process","signalAt":"2026-08-25T11:58:00Z"}}"""

    private fun inventoryWithSessions(vararg sessions: String): String =
        """{"observedAt":"2026-08-25T12:00:00Z","profiles":[{"key":"personal","label":"Personal"}],"sessions":[${sessions.joinToString()}]}"""

    private fun dashboardWithForge(forge: ForgeState): SkidbladnirUiState.Dashboard =
        SkidbladnirUiState.Dashboard(
            inventory = SessionsResponse(
                observedAt = "2026-08-25T12:00:00Z",
                profiles = listOf(ProfileChoice("personal", "Personal")),
                sessions = emptyList(),
            ),
            pressure = null,
            inventoryStale = false,
            inventoryAgeAdvanceSeconds = 0,
            refreshing = false,
            error = null,
            forge = forge,
            forgeRecovery = null,
            kill = null,
        )
}
