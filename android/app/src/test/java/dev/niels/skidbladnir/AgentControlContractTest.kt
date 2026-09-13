package dev.niels.skidbladnir

import org.junit.Assert.assertEquals
import org.junit.Test

class AgentControlContractTest {
    @Test
    fun `inventory accepts provider state and process target without terminal activity`() {
        val encoded = """{"machine":{"handle":"mh-11111111111111111111111111111111","platform":"Linux"},"observedAt":"2026-09-12T00:00:00Z","profiles":[],"sessions":[{"tmuxId":"${'$'}3","tmuxName":"reviewer","identityToken":"lifetime","character":{"key":"norse.durinn","displayName":"Durinn"},"attachedClients":0,"agent":{"provider":"Claude","pid":1234,"paneId":"%4","startIdentity":"5678","status":{"state":"blocked","source":"native","reason":"permission"},"methods":{"read":"native","send":"terminal","interrupt":"terminal"}}}]}"""
        assertEquals(1234L, decodeSessionsResponse(encoded).sessions.single().agent?.pid)
    }
}

class AgentControlRequestTest {
    @Test
    fun `agent controls bind machine pane and process without credentials in input`() {
        val machine = PairedMachine(
            requireNotNull(MachineHandle.parse("mh-11111111111111111111111111111111")),
            requireNotNull(MachineLabel.parse("Arch")),
            requireNotNull(MachineOrigin.parse("https://arch.example:8443")),
        )
        val credential = MachineCredential(machine, requireNotNull(GatewayBearer.parse("A".repeat(43))))
        val session = TmuxSession("${'$'}3", "reviewer", "lifetime", CharacterSummary("norse.durinn", "Durinn"), attachedClients = 1, agent = agentRuntimeFixture(AgentProvider.Claude, 1234))
        val target = SessionTarget(machine.handle, session)
        val client = GatewayClient()
        for (operation in listOf("interrupt", "stop")) {
            val request = client.agentRequest(credential, target, operation)
            assertEquals("/v1/sessions/${'$'}3/agent/$operation", request.url.encodedPath)
            assertEquals(machine.handle.encoded, request.header("Skidbladnir-Machine"))
            val body = okio.Buffer()
            requireNotNull(request.body).writeTo(body)
            assertEquals("""{"identityToken":"lifetime","paneId":"%4","pid":1234,"startIdentity":"5678"}""", body.readUtf8())
        }
    }

    @Test
    fun `partial stop and terminal interrupt stay explicit`() {
        val machine = requireNotNull(MachineLabel.parse("Arch"))
        val stop = decodeAgentStopResult("""{"agent":"unconfirmed","terminal":"closed"}""")
        assertEquals("Arch: agent unconfirmed; terminal closed.", agentStopMessage(machine, stop))
        val interrupt = decodeAgentInterruptResult("""{"method":"terminal","outcome":"written"}""")
        assertEquals("Arch: interrupt key sent; stopping is not yet confirmed.", agentInterruptMessage(machine, interrupt))
    }

    @Test
    fun `native rejection remains an API failure and malformed outcomes fail`() {
        val failure = decodeAgentHttpFailure(409, """{"code":"AgentTargetStale","message":"The agent changed. Refresh and try again.","dispatch":"not_sent"}""")
        assertEquals(GatewayFailure.Api(ApiErrorCode.AgentTargetStale), failure)
        org.junit.Assert.assertThrows(ProtocolDecodeException::class.java) {
            decodeAgentStopResult("""{"agent":"success","terminal":"closed"}""")
        }
        org.junit.Assert.assertThrows(ProtocolDecodeException::class.java) {
            decodeAgentInterruptResult("""{"method":"terminal","outcome":"written","outcome":"interrupted"}""")
        }
    }
}
