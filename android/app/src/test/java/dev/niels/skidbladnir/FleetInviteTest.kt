package dev.niels.skidbladnir

import okhttp3.MediaType.Companion.toMediaType
import okhttp3.Protocol
import okhttp3.Request
import okhttp3.Response
import okhttp3.ResponseBody.Companion.toResponseBody
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test

class FleetInviteTest {
    @Test
    fun `only the exact ordered three-machine fleet invite is accepted`() {
        val accepted = requireNotNull(parseFleetInvite(validInvite))

        assertEquals(listOf("Arch", "Devbox", "MacBook"), accepted.machines.map { it.machine.label.text })

        val rejected = listOf(
            "wrong discriminant" to validInvite.replace("skidbladnir.fleet-invite.v1", "skidbladnir.fleet-invite.v2"),
            "wrong order" to validInvite.replace(arch, devbox).replace(macBook, arch).replace(devbox, macBook),
            "partial fleet" to validInvite.replace(",$macBook", ""),
            "extra fleet member" to validInvite.replace("]}", ",$arch]}"),
            "unknown field" to validInvite.replace("\"kind\"", "\"unknown\":true,\"kind\""),
            "duplicate top-level field" to validInvite.replace(
                "\"kind\":\"skidbladnir.fleet-invite.v1\"",
                "\"kind\":\"skidbladnir.fleet-invite.v1\",\"kind\":\"skidbladnir.fleet-invite.v1\"",
            ),
            "duplicate machine field" to validInvite.replace(
                "\"label\":\"Arch\"",
                "\"label\":\"Arch\",\"label\":\"Arch\"",
            ),
            "escaped duplicate machine field" to validInvite.replace(
                "\"label\":\"Arch\"",
                "\"label\":\"Arch\",\"\\u006cabel\":\"Arch\"",
            ),
            "null field" to validInvite.replace("\"Arch\"", "null"),
            "noncanonical origin" to validInvite.replace("https://arch.example.ts.net:8443/", "HTTPS://arch.example.ts.net:8443/"),
            "duplicate handle" to validInvite.replace("mh-22222222222222222222222222222222", "mh-11111111111111111111111111111111"),
            "duplicate token" to validInvite.replace("CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"),
            "malformed token" to validInvite.replace("BAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", "short"),
            "oversized payload" to validInvite + " ".repeat(4_097),
        )
        rejected.forEach { (case, encoded) ->
            assertNull(case, parseFleetInvite(encoded))
        }
    }

    @Test
    fun `pairing request carries only transient invitation authority and expected identity`() {
        val machine = requireNotNull(parseFleetInvite(validInvite)).machines.single {
            it.machine.label.text == "Devbox"
        }
        val client = GatewayClient()
        try {
            val request = client.pairingRequest(machine)

            assertEquals("POST", request.method)
            assertEquals("https://devbox.example.ts.net:8443/v1/pairings", request.url.toString())
            assertEquals("Skidbladnir-Invite ${machine.pairingInviteToken.encoded}", request.header("Authorization"))
            assertEquals(machine.machine.handle.encoded, request.header("Skidbladnir-Machine"))
            assertEquals(0L, requireNotNull(request.body).contentLength())
            assertTrue(request.header("Authorization")?.startsWith("Bearer ") == false)
        } finally {
            client.closeAsync()
        }
    }

    @Test
    fun `pairing results authorize only matching handles and unique durable bearers`() {
        val invite = requireNotNull(parseFleetInvite(validInvite))
        val responses = invite.machines.mapIndexed { index, machine ->
            PairingResponse(
                MachineSummary(machine.machine.handle, if (index == 2) MachinePlatform.Darwin else MachinePlatform.Linux),
                requireNotNull(GatewayBearer.parse(('D'.code + index).toChar() + "A".repeat(42))),
            )
        }
        val accepted: List<GatewayResult<PairingResponse>> = responses.map { GatewayResult.Success(it) }

        assertEquals(invite.machines.map { it.machine }, requireNotNull(acceptPairingResults(invite, accepted)).map { it.machine })
        val wrongHandle = accepted.toMutableList().also { results ->
            results[1] = GatewayResult.Success(
                PairingResponse(
                    MachineSummary(invite.machines[0].machine.handle, MachinePlatform.Linux),
                    responses[1].bearer,
                ),
            )
        }
        assertNull(acceptPairingResults(invite, wrongHandle))
        val wrongPlatform = accepted.toMutableList().also { results ->
            results[2] = GatewayResult.Success(
                responses[2].copy(machine = responses[2].machine.copy(platform = MachinePlatform.Linux)),
            )
        }
        assertNull(acceptPairingResults(invite, wrongPlatform))
        val duplicateBearer = accepted.toMutableList().also { results ->
            results[2] = GatewayResult.Success(responses[2].copy(bearer = responses[0].bearer))
        }
        assertNull(acceptPairingResults(invite, duplicateBearer))
        assertNull(
            acceptPairingResults(
                invite,
                accepted.toMutableList().also { it[0] = GatewayResult.Failure(GatewayFailure.Transport) },
            ),
        )

        assertEquals(
            GatewayFailure.Api(ApiErrorCode.PairingInviteRejected),
            decodePairingHttpFailure(
                401,
                """{"code":"PairingInviteRejected","message":"This fleet invite is invalid, expired, or already used."}""",
            ),
        )
        assertThrows(ProtocolDecodeException::class.java) {
            decodePairingHttpFailure(
                409,
                """{"code":"MachineIdentityMismatch","message":"The machine identity changed. Fleet reset is required."}""",
            )
        }
        assertThrows(ProtocolDecodeException::class.java) {
            decodePairingHttpFailure(
                422,
                """{"code":"ProfileUnknown","message":"Choose an available profile."}""",
            )
        }
        assertThrows(ProtocolDecodeException::class.java) {
            decodePairingResponse(
                """{"machine":{"handle":"${invite.machines[0].machine.handle.encoded}","platform":"Linux"},"bearer":"short"}""",
            )
        }
    }

    @Test
    fun `pairing DTOs reject literal and escaped duplicate semantic keys`() {
        val handle = "mh-0123456789abcdef0123456789abcdef"
        for (encoded in listOf(
            """{"machine":{"handle":"$handle","platform":"Linux"},"bearer":"${"A".repeat(43)}","bearer":"${"B".repeat(43)}"}""",
            """{"machine":{"handle":"$handle","platform":"Linux"},"bearer":"${"A".repeat(43)}","bea\u0072er":"${"B".repeat(43)}"}""",
        )) {
            assertThrows(ProtocolDecodeException::class.java) { decodePairingResponse(encoded) }
        }
        for (encoded in listOf(
            """{"code":"InvalidRequest","code":"InternalError","message":"Internal server error."}""",
            """{"code":"InvalidRequest","co\u0064e":"InternalError","message":"Internal server error."}""",
        )) {
            assertThrows(ProtocolDecodeException::class.java) {
                decodePairingHttpFailure(500, encoded)
            }
        }
    }

    @Test
    fun `wrong media type and oversized decoded pairing bodies settle without credentials`() {
        val invite = requireNotNull(parseFleetInvite(validInvite))
        val validResponse = { machine: FleetInviteMachine ->
            """{"machine":{"handle":"${machine.machine.handle.encoded}","platform":"Linux"},"bearer":"${"D" + "A".repeat(42)}"}"""
        }
        val results = invite.machines.mapIndexed { index, machine ->
            val encoded = if (index == 0) {
                validResponse(machine)
            } else {
                validResponse(machine) + " ".repeat(MAXIMUM_HTTP_BODY_BYTES + 1)
            }
            gatewayResponse(
                encoded = encoded,
                contentType = if (index == 0) "text/plain" else "application/json",
            )
        }

        assertTrue(results.all { it == GatewayResult.Failure(GatewayFailure.Transport) })
        assertNull(acceptPairingResults(invite, results))
    }

    @Test
    fun `gateway response rejects malformed UTF-8 before protocol decoding`() {
        val prefix = "{\"machine\":{\"handle\":\"mh-0123456789abcdef0123456789abcdef\",\"platform\":\"Linux\"},\"observedAt\":\"2026-08-25T12:00:00Z\",\"profiles\":[],\"sessions\":[{\"tmuxId\":\"\$1\",\"tmuxName\":\""
        val suffix = "\",\"identityToken\":\"token\",\"character\":{\"key\":\"one\",\"displayName\":\"One\"},\"attachedClients\":0,\"activity\":\"Quiet\"}]}"
        val malformed = prefix.encodeToByteArray() + byteArrayOf(0xff.toByte()) + suffix.encodeToByteArray()
        val response = Response.Builder()
            .request(Request.Builder().url("https://gateway.example.ts.net:8443/v1/sessions").build())
            .protocol(Protocol.HTTP_1_1)
            .code(200)
            .message("OK")
            .body(malformed.toResponseBody("application/json".toMediaType()))
            .build()

        assertThrows(ProtocolDecodeException::class.java) {
            response.use {
                decodeGatewayResponse(it, expectedStatus = 200, decode = ::decodeSessionsResponse)
            }
        }
    }

    @Test
    fun `bodyless success with malformed bytes remains a transport failure`() {
        val response = Response.Builder()
            .request(Request.Builder().url("https://gateway.example.ts.net:8443/v1/sessions/1").build())
            .protocol(Protocol.HTTP_1_1)
            .code(204)
            .message("No Content")
            .body(byteArrayOf(0xff.toByte()).toResponseBody())
            .build()

        assertEquals(
            GatewayResult.Failure(GatewayFailure.Transport),
            response.use { decodeGatewayResponse(it, expectedStatus = 204, decode = { Unit }) },
        )
    }

    private fun gatewayResponse(
        encoded: String,
        contentType: String,
    ): GatewayResult<PairingResponse> {
        val response = Response.Builder()
            .request(Request.Builder().url("https://gateway.example.ts.net:8443/v1/pairings").build())
            .protocol(Protocol.HTTP_1_1)
            .code(200)
            .message("OK")
            .body(encoded.toResponseBody(contentType.toMediaType()))
            .build()
        return response.use {
            decodeGatewayResponse(it, expectedStatus = 200, decode = ::decodePairingResponse)
        }
    }

    private companion object {
        const val arch = "{\"label\":\"Arch\",\"origin\":\"https://arch.example.ts.net:8443/\",\"machineHandle\":\"mh-11111111111111111111111111111111\",\"pairingInviteToken\":\"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA\"}"
        const val devbox = "{\"label\":\"Devbox\",\"origin\":\"https://devbox.example.ts.net:8443/\",\"machineHandle\":\"mh-22222222222222222222222222222222\",\"pairingInviteToken\":\"BAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA\"}"
        const val macBook = "{\"label\":\"MacBook\",\"origin\":\"https://macbook.example.ts.net:8443/\",\"machineHandle\":\"mh-33333333333333333333333333333333\",\"pairingInviteToken\":\"CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA\"}"
        const val validInvite = "{\"kind\":\"skidbladnir.fleet-invite.v1\",\"machines\":[$arch,$devbox,$macBook]}"
    }
}
