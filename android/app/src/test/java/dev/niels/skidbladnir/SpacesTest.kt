package dev.niels.skidbladnir

import java.time.Instant
import java.util.concurrent.Executor
import okhttp3.OkHttpClient
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.Protocol
import okhttp3.Response
import okhttp3.ResponseBody.Companion.toResponseBody
import okio.Buffer
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test

class SpacesTest {
    @Test
    fun `inventory accepts canonical session membership`() {
        val decoded = decodeSessionsResponse(inventory("\"space\":\"alpha\","))
        assertEquals(1, decoded.sessions.size)
        assertTrue("named membership projection", decoded.sessions.single().space == label("alpha"))
        assertNull(decodeSessionsResponse(inventory("")).sessions.single().space)
        listOf("null", "\"\"", "8", "true", "[]", "{}", "\"e\\u0301\"", "\"a\\ud800\"").forEachIndexed { index, value ->
            assertThrows("invalid space wire case $index", ProtocolDecodeException::class.java) {
                decodeSessionsResponse(inventory("\"space\":$value,"))
            }
        }
    }

    @Test
    fun `canonical labels and human drafts have different ingress`() {
        val supplementary = String(Character.toChars(0x1f680))
        val valid = listOf("alpha", "Alpha", "alpha  beta", "all spaces", "unassigned", "previously selected space",
            "é", supplementary.repeat(64), "a".repeat(64), "x" + "\u035c".repeat(31))
        valid.forEachIndexed { index, value -> assertTrue("valid label case $index", SpaceLabel.parse(value) != null) }
        val invalid = listOf("", " alpha", "alpha ", "a".repeat(65), supplementary.repeat(65), "e\u0301", "a\ud800") +
            listOf(0, 0x1f, 0x7f, 0x9f, 0x061c, 0x200e, 0x200f, 0x2028, 0x202e, 0x2066, 0x2069,
                0x00a0, 0x1680, 0x2000, 0x200a, 0x202f, 0x205f, 0x3000).map { "a${it.toChar()}b" }
        invalid.forEachIndexed { index, value -> assertTrue("invalid label case $index", SpaceLabel.parse(value) == null) }
        assertTrue("draft normalization", SpaceLabel.fromDraft("e\u0301") == label("é"))
        val combining = "x" + "\u035c".repeat(31)
        assertTrue("canonical combining sequence remains unchanged", SpaceLabel.fromDraft(combining)?.text == combining)
        val explicitJoiner = combining + "\u034f" + "\u035c".repeat(30)
        assertTrue("explicit combining joiner is preserved", SpaceLabel.fromDraft(explicitJoiner)?.text == explicitJoiner)
        listOf("a\ud800", "a\udc00", "a\udc00\ud800").forEachIndexed { index, candidate ->
            assertNull("human surrogate case $index remains invalid", SpaceLabel.fromDraft(candidate))
        }
        assertTrue("exact equality", label("alpha") != label("Alpha"))
    }

    @Test
    fun `normalization uses Unicode15 regardless of platform Unicode version`() {
        val kawi = "x\u0315" + String(Character.toChars(0x11f41))
        val orderedKawi = "x" + String(Character.toChars(0x11f41)) + "\u0315"
        assertNull("Unicode15 canonical ordering is required", SpaceLabel.parse(kawi))
        assertTrue("Unicode15 draft reorders combining classes", SpaceLabel.fromDraft(kawi)?.text == orderedKawi)
        val laterMark = "x\u0315\u0897"
        assertTrue("post15 scalar is an inert boundary", SpaceLabel.parse(laterMark)?.text == laterMark)
        assertTrue("post15 draft preserves the inert boundary", SpaceLabel.fromDraft(laterMark)?.text == laterMark)
        val laterComposition = String(Character.toChars(0x105d2)) + "\u0307"
        assertTrue("post15 composition stays uncomposed", SpaceLabel.fromDraft(laterComposition)?.text == laterComposition)
        val unassigned = "x\u0315" + String(Character.toChars(0x10ffff))
        assertTrue("unassigned scalar stays inert", SpaceLabel.fromDraft(unassigned)?.text == unassigned)
    }

    @Test
    fun `create accepts the space validation error contract and emits omission or canonical label`() {
        val failure = decodeCreateHttpFailure(422, """{"code":"SpaceInvalid","message":"use 1–64 nfc characters; only interior ordinary spaces, without display controls.","dispatch":"not_sent"}""")
        assertTrue("validation copy", gatewayFailureMessage(failure) == SPACE_INVALID)
        val form = ForgeForm(machine.handle, "~", LaunchChoice.Agent(profile), "", "", SpaceDraft.Chosen("e\u0301"))
        val draft = requireNotNull(form.submission())
        assertTrue("normalized creation field", strictJsonObject(encodeCreateSessionRequest(draft))["space"].toString() == "\"é\"")
        assertFalse(strictJsonObject(encodeCreateSessionRequest(draft.copy(space = null))).containsKey("space"))
        assertNull(form.copy(space = SpaceDraft.Unresolved).submission())
        assertNull(form.copy(space = SpaceDraft.Chosen("bad\nlabel")).submission())
        val changed = changeForgeDraft(form, form.copy(machineHandle = other.handle))
        assertTrue("machine preserves label draft", changed.space == form.space)
        assertTrue("machine clears working directory", changed.cwd.isEmpty())
        assertNull(changed.launch)
    }

    @Test
    fun `membership request addresses retained session only and unknown dispatch remains unknown`() {
        val target = SessionTarget(machine.handle, session("1", label("alpha")))
        val client = GatewayClient()
        val request = client.spaceRequest(credential, target, null)
        assertTrue("put route", request.method == "PUT" && request.url.pathSegments == listOf("v1", "sessions", "${'$'}1", "space"))
        assertTrue("pinned machine", request.header("Skidbladnir-Machine") == machine.handle.encoded)
        val body = strictJsonObject(Buffer().also { request.body!!.writeTo(it) }.readUtf8())
        assertTrue("exact membership keys", body.keys == setOf("identityToken", "space"))
        assertTrue("clear body", body["space"].toString() == "\"\"")
        assertFalse(client.http.retryOnConnectionFailure)
        assertFalse(client.http.followRedirects)
        for (dispatch in listOf("not_sent", "unknown")) {
            val failure = decodeSpaceHttpFailure(500,
                """{"code":"InternalError","message":"Skíðblaðnir could not complete the request.","dispatch":"$dispatch"}""") as GatewayFailure.Api
            assertTrue("dispatch survives decoding", failure.dispatch == if (dispatch == "not_sent") MutationDispatch.NotSent else MutationDispatch.Unknown)
        }
        listOf(
            """{"code":"SpaceInvalid","message":"$SPACE_INVALID","dispatch":"unknown"}""",
            """{"code":"SpaceInvalid","message":"$SPACE_INVALID"}""",
            """{"code":"SpaceInvalid","message":"$SPACE_INVALID","dispatch":null}""",
        ).forEachIndexed { index, body ->
            assertThrows("invalid dispatch case $index", ProtocolDecodeException::class.java) { decodeSpaceHttpFailure(422, body) }
        }
        var calls = 0
        val malformed = GatewayClient(OkHttpClient.Builder().retryOnConnectionFailure(false).addInterceptor { chain ->
            calls += 1
            Response.Builder().request(chain.request()).protocol(Protocol.HTTP_1_1).code(500).message("failure")
                .header("Content-Type", "application/json")
                .body("{".toResponseBody("application/json".toMediaType())).build()
        }.build())
        assertTrue("malformed completion stays uncertain", malformed.setSessionSpace(credential, target, label("beta")) ==
            GatewayResult.Failure(GatewayFailure.Transport))
        assertEquals(1, calls)
    }

    @Test
    fun `grouping intersects independent filters while retaining stale source facts`() {
        val a = label("alpha")
        val upper = label("Alpha")
        val beta = label("beta")
        val first = ready(machine, session("1", a), session("2", null), session("3", beta))
        val second = ready(other, session("4", a), session("5", upper))
        val machines = listOf(first, second)
        val items = dashboardItems(machines, DashboardScope.All, DashboardSpaceSelection.All)
        assertTrue("deterministic heading order", items.filterIsInstance<DashboardItem.Heading>().map { it.label } == listOf(upper, a, beta, null))
        assertTrue("cross-host grouping", items.drop(3).take(2).filterIsInstance<DashboardItem.Session>().size == 2)
        val selected = DashboardSpaceSelection.Named(spaceFingerprint(a), a)
        val filtered = dashboardItems(machines, DashboardScope.Machine(other.handle), selected)
        assertEquals(2, filtered.size)
        assertTrue("machine intersection", (filtered.last() as DashboardItem.Session).visible.target.machineHandle == other.handle)
        val stale = second.copy(inventory = InventoryState.Stale(requireNotNull(second.inventory.lastSnapshot()), GatewayFailure.Transport))
        assertEquals(3, dashboardItems(listOf(first, stale), DashboardScope.All, selected).size)
        assertFalse(stale.canMutate)
        assertTrue("suggestions independent of filtering", observedSpaces(machines) == listOf(upper, a, beta))
        val unicodeOrder = ready(machine, session("6", a).copy(tmuxName = "ä"), session("7", a).copy(tmuxName = "Z"))
        val before = visibleSessions(listOf(unicodeOrder), DashboardScope.All)
        val groupedOrder = dashboardItems(listOf(unicodeOrder), DashboardScope.All, selected)
            .filterIsInstance<DashboardItem.Session>().map { it.visible }
        assertTrue("within-group order preserved", groupedOrder == before)
        assertTrue("headings have content-free keys", items.filterIsInstance<DashboardItem.Heading>().all {
            it.key is DashboardItemKey.Space || it.key == DashboardItemKey.Unassigned
        })
    }

    @Test
    fun `editor keeps its draft and session lifetime across concurrent metadata changes`() {
        val target = SessionTarget(machine.handle, session("1", label("alpha")))
        val editor = SpaceEditor(target, "beta")
        assertTrue(spaceSubmissionAdmissible(editor, ready(machine, target.session)))
        val renamed = target.session.copy(tmuxName = "changed", space = label("beta"))
        assertFalse(spaceSubmissionAdmissible(editor, ready(machine, renamed)))
        assertTrue("rename does not change filing authority", sameSessionLifetime(target, target.copy(session = renamed)))
        val sending = editor.copy(phase = SpacePhase.Sending)
        assertTrue("sending ignores early absence", reconcileSpaceEditor(sending, ready(machine)) == sending)
        val acknowledged = completeSpaceHttp(sending, GatewayResult.Success(Unit))
        assertNull(reconcileSpaceEditor(acknowledged, ready(machine, renamed.copy(space = label("gamma")))))
        val unknown = completeSpaceHttp(sending, GatewayResult.Failure(GatewayFailure.Transport))
        val reviewed = requireNotNull(reconcileSpaceEditor(unknown, ready(machine, renamed)))
        assertTrue("matching observation does not prove uncertain assignment", reviewed.error == SPACE_OUTCOME_UNKNOWN)
        assertTrue("draft survives observation", reviewed.draft == editor.draft && reviewed.phase == SpacePhase.Editing)
        assertNull(reconcileSpaceEditor(editor, ready(machine, target.session.copy(identityToken = "replacement"))))
        val rejected = completeSpaceHttp(sending, GatewayResult.Failure(GatewayFailure.Api(ApiErrorCode.SpaceInvalid, MutationDispatch.NotSent)))
        assertTrue("definite rejection remains editable", rejected.phase == SpacePhase.Editing && rejected.draft == editor.draft)
    }

    @Test
    fun `pre-dispatch stale or unreadable inventory stays fenced`() {
        val sending = SpaceEditor(SessionTarget(machine.handle, session("1", label("alpha"))), "beta", SpacePhase.Sending)
        for (code in listOf(ApiErrorCode.SessionNotFound, ApiErrorCode.SessionIdentityMismatch, ApiErrorCode.InternalError)) {
            val completed = completeSpaceHttp(sending, GatewayResult.Failure(GatewayFailure.Api(code, MutationDispatch.NotSent)))
            assertTrue("invalidated inventory requires an ordered read", completed.phase is SpacePhase.Checking)
            assertTrue("definite rejection remains definite", completed.error == apiErrorMessage(code))
        }
    }

    @Test
    fun `ordered metadata read follows the mutation while older read cannot clear fence`() {
        val queued = ArrayDeque<Runnable>()
        val lane = InventoryOperationLane(Executor { queued.add(it) }) { throw it }
        val observed = mutableListOf<Long>()
        var fence = 0L
        lane.submitRead { observed.add(it) }
        lane.submitMutation(onReserved = { fence = it }) { }
        lane.submitRead { observed.add(it) }
        queued.removeFirst().run()
        assertTrue("ordered read fences", observed == listOf(0L, fence))
        val snapshot = requireNotNull(ready(machine, session("1", null)).inventory.lastSnapshot())
        val superseded = InventoryState.Superseded(snapshot, fence)
        assertTrue("older completion cannot clear", clearMetadataMutationFence(superseded, 0) == superseded)
        assertTrue("exact rejected fence clears", clearMetadataMutationFence(superseded, fence) is InventoryState.Fresh)
    }

    @Test
    fun `fresh navigation uses current task schema and preserves absent label intent`() {
        assertEquals(2, DashboardEntryState().snapshot().schemaVersion)
        val label = label("alpha")
        val fingerprint = spaceFingerprint(label)
        val heading = DashboardItemKey.Space(fingerprint)
        val snapshot = DashboardEntrySnapshot(2, DashboardScope.All, DashboardViewport(heading, 3, 17), DashboardSpaceKey.Named(fingerprint))
        val entry = DashboardEntryState(snapshot)
        entry.acceptFleet(setOf(machine.handle, other.handle))
        assertTrue("unresolved intent", entry.space == DashboardSpaceSelection.Named(fingerprint))
        assertTrue("explicit create decision", entry.space.creationDraft() == SpaceDraft.Unresolved)
        entry.resolveSpace(emptyList())
        assertTrue(entry.restorationPending)
        entry.resolveSpace(listOf(label))
        assertTrue("name resolution keeps comparison and pending restore", entry.space == DashboardSpaceSelection.Named(fingerprint, label) && entry.restorationPending)
        assertTrue("snapshot remains content-free key", entry.snapshot().space == DashboardSpaceKey.Named(fingerprint))
        entry.restoreOnce(listOf(DashboardItemKey.Unassigned, heading))
        assertEquals(1, entry.gridState.firstVisibleItemIndex)
        assertEquals(17, entry.gridState.firstVisibleItemScrollOffset)
        val grid = entry.gridState
        entry.selectSpace(DashboardSpaceSelection.Named(fingerprint, label))
        assertTrue("same filter retains grid", grid === entry.gridState)
        entry.followCreatedMembership(label("beta"))
        assertFalse("explicit creation reset has no old measured grid", grid === entry.gridState)
        assertNull(entry.snapshot().viewport.anchor)
        assertEquals(0, entry.snapshot().viewport.fallbackIndex)
        assertEquals(0, entry.snapshot().viewport.offsetPx)
        entry.selectTerminalAccessLoss(other.handle)
        assertTrue("access loss retains selected space", entry.space.matches(label("beta")))
        entry.acceptFleet(setOf(machine.handle))
        assertTrue("missing pairing resets both dimensions", entry.scope == DashboardScope.All && entry.space == DashboardSpaceSelection.All)
    }

    private fun label(text: String) = requireNotNull(SpaceLabel.parse(text))
    private fun session(id: String, space: SpaceLabel?) = TmuxSession(
        tmuxId = "${'$'}$id", tmuxName = "session$id", identityToken = "identity$id",
        character = CharacterSummary("norse.durinn", "Durinn"), attachedClients = 0, space = space,
    )
    private fun ready(machine: PairedMachine, vararg sessions: TmuxSession) = MachineState(
        machine, MachineAccess.Ready, InventoryState.Fresh(InventorySnapshot(
            SessionsResponse(MachineSummary(machine.handle, MachinePlatform.Linux), Instant.parse("2026-09-15T12:00:00Z"), emptyList(), sessions.toList()), 10,
        )), PressureState.Reading,
    )
    private fun inventory(space: String): String = """{"machine":{"handle":"mh-0123456789abcdef0123456789abcdef","platform":"Linux"},"observedAt":"2026-09-15T12:00:00Z","profiles":[],"sessions":[{"tmuxId":"${'$'}1","tmuxName":"one","identityToken":"token","character":{"key":"norse.durinn","displayName":"Durinn"},${space}"attachedClients":0}]}"""
    private val profile = requireNotNull(ProfileKey.parse("personal"))
    private val machine = PairedMachine(requireNotNull(MachineHandle.parse("mh-0123456789abcdef0123456789abcdef")),
        requireNotNull(MachineLabel.parse("one")), requireNotNull(MachineOrigin.parse("https://one.example:8443/")))
    private val other = PairedMachine(requireNotNull(MachineHandle.parse("mh-11111111111111111111111111111111")),
        requireNotNull(MachineLabel.parse("two")), requireNotNull(MachineOrigin.parse("https://two.example:8443/")))
    private val credential = MachineCredential(machine, requireNotNull(GatewayBearer.parse("A".repeat(43))))
}
