package dev.niels.skidbladnir

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertThrows
import org.junit.Test

class AgentIdentityContractTest {
    @Test
    fun `inventory accepts every legal optional agent identity shape`() {
        val inventory = decodeSessionsResponse(identityInventory())

        assertEquals(
            listOf(
                ProfileChoice(workProfile, "Codex · Work", AgentProvider.Codex),
                ProfileChoice(claudeWorkProfile, "Claude · Work", AgentProvider.Claude),
            ),
            inventory.profiles,
        )
        assertEquals(workProfile, inventory.sessions[0].launchProfile)
        assertEquals(
            AgentRuntime(
                provider = AgentProvider.Codex,
                pid = 1234,
                profile = workProfile,
                providerSession =
                    ProviderSessionFacts.withId("019f0d13-8f42-7ce0-8420-a37a1ef2e769"),
            ),
            inventory.sessions[0].agent,
        )
        assertEquals(claudeWorkProfile, inventory.sessions[1].launchProfile)
        assertEquals(
            AgentRuntime(
                provider = AgentProvider.Claude,
                pid = 2345,
                profile = claudeWorkProfile,
                providerSession = ProviderSessionFacts.withId("claude-session", "ga-claude"),
            ),
            inventory.sessions[1].agent,
        )
        assertEquals(
            AgentRuntime(
                AgentProvider.Claude,
                3456,
                providerSession = ProviderSessionFacts.withName("raw-claude"),
            ),
            inventory.sessions[2].agent,
        )
        assertNull(inventory.sessions[2].launchProfile)
        assertEquals(claudeWorkProfile, inventory.sessions[3].launchProfile)
        assertNull(inventory.sessions[3].agent)
        assertNull(inventory.sessions[4].launchProfile)
        assertNull(inventory.sessions[4].agent)
        assertEquals(List(5) { SessionActivity.Quiet }, inventory.sessions.map(TmuxSession::activity))
    }

    @Test
    fun `profile keys use the gateway canonical grammar`() {
        val accepted = listOf("a", "a0_-z", "a".repeat(32))
        val rejected = listOf(
            "", "A", "0work", "-work", "_work", "work.profile", "work/profile",
            "work profile", "a".repeat(33), "wörk",
        )

        accepted.forEachIndexed { index, candidate ->
            assertEquals(
                "rejected canonical profile key case $index",
                candidate,
                ProfileKey.parse(candidate)?.encoded,
            )
        }
        rejected.forEachIndexed { index, candidate ->
            assertNull("accepted malformed profile key case $index", ProfileKey.parse(candidate))
        }
    }

    @Test
    fun `identity decoder rejects legacy nullable and malformed states`() {
        val codex = tmuxSession(
            tmuxId = "${'$'}1",
            agent =
                """{"provider":"Codex","pid":1234,"profile":"work","providerSession":{"id":"codex-session"}}""",
        )
        val claude = tmuxSession(
            tmuxId = "${'$'}2",
            launchProfile = null,
            agent =
                """{"provider":"Claude","pid":2345,"providerSession":{"id":"claude-session"}}""",
        )
        val invalidSessions = mapOf(
            "legacy id" to codex.replaceFirst("\"tmuxId\"", "\"id\""),
            "legacy profile" to codex.replaceFirst("\"launchProfile\"", "\"profile\""),
            "missing character" to codex.replace(
                "\"character\":{\"key\":\"norse.durinn\",\"displayName\":\"Durinn\"},",
                "",
            ),
            "null agent" to tmuxSession(agent = "null"),
            "null agent profile" to codex.replace("\"profile\":\"work\"", "\"profile\":null"),
            "zero pid" to codex.replace("\"pid\":1234", "\"pid\":0"),
            "Codex provider name" to codex.replace(
                "{\"id\":\"codex-session\"}",
                "{\"id\":\"codex-session\",\"name\":\"ga-codex\"}",
            ),
            "empty provider session" to codex.replace(
                "\"providerSession\":{\"id\":\"codex-session\"}",
                "\"providerSession\":{}",
            ),
            "null provider session" to codex.replace(
                "\"providerSession\":{\"id\":\"codex-session\"}",
                "\"providerSession\":null",
            ),
            "null provider id" to codex.replace("\"id\":\"codex-session\"", "\"id\":null"),
            "empty provider id" to codex.replace("codex-session", ""),
            "long provider id" to codex.replace("codex-session", "A".repeat(129)),
            "non-visible provider id" to codex.replace("codex-session", "codex session"),
            "empty provider name" to claude.replace(
                "{\"id\":\"claude-session\"}",
                "{\"name\":\"\"}",
            ),
            "long provider name" to claude.replace(
                "{\"id\":\"claude-session\"}",
                "{\"name\":\"${"A".repeat(129)}\"}",
            ),
            "non-NFC provider name" to claude.replace(
                "{\"id\":\"claude-session\"}",
                "{\"name\":\"Cafe${'\u0301'}\"}",
            ),
            "bidi provider name" to claude.replace(
                "{\"id\":\"claude-session\"}",
                "{\"name\":\"bad${'\u202e'}name\"}",
            ),
            "line separator provider name" to claude.replace(
                "{\"id\":\"claude-session\"}",
                "{\"name\":\"bad${'\u2028'}name\"}",
            ),
            "paragraph separator provider name" to claude.replace(
                "{\"id\":\"claude-session\"}",
                "{\"name\":\"bad${'\u2029'}name\"}",
            ),
            "unknown provider" to codex.replace("\"Codex\"", "\"Gemini\""),
            "extra agent field" to codex.replace("\"pid\":1234", "\"pid\":1234,\"unexpected\":true"),
        )
        invalidSessions.forEach { (case, payload) ->
            assertThrows(case, ProtocolDecodeException::class.java) { decodeInventorySession(payload) }
        }

        val providerlessProfile = identityInventory().replace(
            """{"key":"work","label":"Codex · Work","provider":"Codex"}""",
            """{"key":"work","label":"Codex · Work"}""",
        )
        val mismatchedAgentProfile = identityInventory().replaceFirst(
            "\"profile\":\"work\"",
            "\"profile\":\"claude-work\"",
        )
        val unknownLaunchProfile = identityInventory().replaceFirst(
            "\"launchProfile\":\"work\"",
            "\"launchProfile\":\"retired-profile\"",
        )
        assertThrows(ProtocolDecodeException::class.java) { decodeSessionsResponse(providerlessProfile) }
        assertThrows(ProtocolDecodeException::class.java) { decodeSessionsResponse(mismatchedAgentProfile) }
        assertThrows(ProtocolDecodeException::class.java) { decodeSessionsResponse(unknownLaunchProfile) }
    }

    @Test
    fun `optional agent identity is orthogonal to activity and profile presentation`() {
        val activeWithAgent = decodeInventorySession(
            tmuxSession(activity = "Active", agent = """{"provider":"Codex","pid":1234}"""),
        )
        val activeWithoutAgent = decodeInventorySession(tmuxSession(activity = "Active"))
        val quietWithAgent = decodeInventorySession(
            tmuxSession(activity = "Quiet", agent = """{"provider":"Codex","pid":1234}"""),
        )
        val profiles = listOf(ProfileChoice(workProfile, "Codex · Work", AgentProvider.Codex))

        assertEquals(SessionActivity.Active, activeWithAgent.activity)
        assertEquals(SessionActivity.Active, activeWithoutAgent.activity)
        assertEquals(SessionActivity.Quiet, quietWithAgent.activity)
        assertEquals("Codex · profile unknown", sessionProfileLabel(activeWithAgent, profiles))
        assertEquals("Codex · Work", sessionProfileLabel(activeWithoutAgent, profiles))
    }

    @Test
    fun `session target binds machine tmux identity and displayed name`() {
        val session = decodeSessionsResponse(identityInventory()).sessions.first()
        val target = SessionTarget(devboxHandle, session)

        assertEquals("${'$'}1", target.session.tmuxId)
        assertNotEquals(target, SessionTarget(macBookHandle, session))
        assertEquals(
            "{\"tmuxName\":\"ga-codex\",\"identityToken\":\"codex-token\"}",
            encodeKillSessionRequest(target.session),
        )
    }

    private fun identityInventory(): String = inventory(
        tmuxSession(
            tmuxId = "${'$'}1",
            tmuxName = "ga-codex",
            identityToken = "codex-token",
            launchProfile = "work",
            agent =
                """{"provider":"Codex","pid":1234,"profile":"work","providerSession":{"id":"019f0d13-8f42-7ce0-8420-a37a1ef2e769"}}""",
        ),
        tmuxSession(
            tmuxId = "${'$'}2",
            tmuxName = "ga-claude",
            identityToken = "claude-token",
            launchProfile = "claude-work",
            agent =
                """{"provider":"Claude","pid":2345,"profile":"claude-work","providerSession":{"id":"claude-session","name":"ga-claude"}}""",
        ),
        tmuxSession(
            tmuxId = "${'$'}3",
            tmuxName = "raw-claude",
            identityToken = "raw-claude-token",
            launchProfile = null,
            agent = """{"provider":"Claude","pid":3456,"providerSession":{"name":"raw-claude"}}""",
        ),
        tmuxSession(
            tmuxId = "${'$'}4",
            tmuxName = "shell-from-claude",
            identityToken = "shell-claude-token",
            launchProfile = "claude-work",
        ),
        tmuxSession(
            tmuxId = "${'$'}5",
            tmuxName = "laptop-shell",
            identityToken = "laptop-shell-token",
            launchProfile = null,
        ),
    )

    private fun inventory(vararg sessions: String): String =
        """
        {
          "machine":{"handle":"${devboxHandle.encoded}","platform":"Linux"},
          "observedAt":"2026-08-25T12:00:00Z",
          "profiles":[
            {"key":"work","label":"Codex · Work","provider":"Codex"},
            {"key":"claude-work","label":"Claude · Work","provider":"Claude"}
          ],
          "sessions":[${sessions.joinToString()}]
        }
        """.trimIndent()

    private fun decodeInventorySession(session: String): TmuxSession =
        decodeSessionsResponse(inventory(session)).sessions.single()

    private fun tmuxSession(
        tmuxId: String = "${'$'}9",
        tmuxName: String = "fixture",
        identityToken: String = "fixture-token",
        launchProfile: String? = "work",
        activity: String = "Quiet",
        agent: String? = null,
    ): String {
        val launchProfileField = launchProfile?.let { ""","launchProfile":"$it"""" }.orEmpty()
        val agentField = agent?.let { ",\"agent\":$it" }.orEmpty()
        return """{"tmuxId":"$tmuxId","tmuxName":"$tmuxName","identityToken":"$identityToken"$launchProfileField,"character":{"key":"norse.durinn","displayName":"Durinn"},"attachedClients":0,"activity":"$activity"$agentField}"""
    }

    private companion object {
        val devboxHandle = requireNotNull(MachineHandle.parse("mh-0123456789abcdef0123456789abcdef"))
        val macBookHandle = requireNotNull(MachineHandle.parse("mh-fedcba9876543210fedcba9876543210"))
        val workProfile = requireNotNull(ProfileKey.parse("work"))
        val claudeWorkProfile = requireNotNull(ProfileKey.parse("claude-work"))
    }
}
