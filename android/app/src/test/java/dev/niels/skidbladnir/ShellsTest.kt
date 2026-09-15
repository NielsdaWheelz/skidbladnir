package dev.niels.skidbladnir

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Assert.assertThrows
import org.junit.Test

class ShellsTest {
    @Test
    fun `fresh machine with zero profiles permits terminal creation`() {
        val inventory = decodeSessionsResponse("""{"machine":{"handle":"${machine.handle.encoded}","platform":"Linux"},"observedAt":"2026-09-15T12:00:00Z","profiles":[],"sessions":[]}""")
        val ready = MachineState(machine, MachineAccess.Ready,
            InventoryState.Fresh(InventorySnapshot(inventory, 0)), PressureState.Reading)
        assertTrue("terminal creation needs no agent installation", ready.canForge)
        assertFalse(ready.copy(access = MachineAccess.AuthRequired).canForge)
        assertFalse(ready.copy(inventory = InventoryState.Reading).canForge)
    }

    @Test
    fun `creation encodes its required launch discriminator`() {
        val draft = ForgeDraft(machine.handle, "~", LaunchChoice.Agent(requireNotNull(ProfileKey.parse("work"))), "", "")
        val encoded = strictJsonObject(encodeCreateSessionRequest(draft))
        assertEquals("agent", encoded["kind"]?.toString()?.trim('"'))
        assertEquals("""{"kind":"terminal","cwd":"~"}""",
            encodeCreateSessionRequest(draft.copy(launch = LaunchChoice.Terminal)))
    }

    @Test
    fun `machine changes retain terminal and metadata but reset machine-local choices`() {
        val form = ForgeForm(machine.handle, "/source", LaunchChoice.Terminal, "shell", "intent", SpaceDraft.Chosen("one"))
        val other = requireNotNull(MachineHandle.parse("mh-1123456789abcdef0123456789abcdef"))
        val changed = changeForgeDraft(form, form.copy(machineHandle = other))
        assertTrue("machine change preserves independent draft fields", form.copy(machineHandle = other, cwd = "") == changed)
        assertNull("changing host requires another directory choice", changed.submission())
        assertEquals(LaunchChoice.Terminal, changed.copy(cwd = "~").submission()?.launch)
        val agent = form.copy(launch = LaunchChoice.Agent(requireNotNull(ProfileKey.parse("work"))))
        assertNull(changeForgeDraft(agent, agent.copy(machineHandle = other)).launch)
        assertNull("a launch choice remains explicit", form.copy(launch = null).submission())
    }

    @Test
    fun `creation uncertainty follows dispatch instead of error code`() {
        assertTrue(createFailureIsDefinitive(GatewayFailure.Api(ApiErrorCode.InternalError, MutationDispatch.NotSent)))
        assertFalse(createFailureIsDefinitive(GatewayFailure.Api(ApiErrorCode.SessionNameConflict, MutationDispatch.Unknown)))
        assertFalse(createFailureIsDefinitive(GatewayFailure.Transport))
        for (status in listOf(502, 503, 504)) {
            assertEquals(GatewayFailure.Transport, decodeCreateHttpFailure(status, ""))
        }
        val code = ApiErrorCode.SessionNotFound
        assertEquals(GatewayFailure.Api(code, MutationDispatch.NotSent), decodeCreateHttpFailure(404,
            """{"code":"SessionNotFound","message":"${apiErrorMessage(code)}","dispatch":"not_sent"}"""))
        assertThrows(ProtocolDecodeException::class.java) {
            decodeCreateHttpFailure(404, """{"code":"SessionNotFound","message":"${apiErrorMessage(code)}"}""")
        }
        val valid = """{"code":"InternalError","message":"${apiErrorMessage(ApiErrorCode.InternalError)}","dispatch":"unknown"}"""
        assertFalse(createFailureIsDefinitive(decodeCreateHttpFailure(500, valid)))
        listOf(valid.replace("unknown", "not-sent"), valid.replace("\"unknown\"", "null"),
            valid.dropLast(1) + ",\"dispatch\":\"unknown\"}", valid.dropLast(1) + ",\"legacy\":true}")
            .forEachIndexed { index, encoded ->
                assertThrows("invalid create error shape $index", ProtocolDecodeException::class.java) { decodeCreateHttpFailure(500, encoded) }
            }
    }

    private val machine = PairedMachine(requireNotNull(MachineHandle.parse("mh-0123456789abcdef0123456789abcdef")),
        requireNotNull(MachineLabel.parse("one")), requireNotNull(MachineOrigin.parse("https://one.example:8443/")))
}
