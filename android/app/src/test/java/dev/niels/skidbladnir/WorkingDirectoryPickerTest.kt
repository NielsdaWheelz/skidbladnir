package dev.niels.skidbladnir

import java.time.Instant
import okio.Buffer
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertSame
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test

class WorkingDirectoryPickerTest {
    private val handle = requireNotNull(
        MachineHandle.parse("mh-0123456789abcdef0123456789abcdef"),
    )
    private val otherHandle = requireNotNull(
        MachineHandle.parse("mh-fedcba9876543210fedcba9876543210"),
    )
    private val summary = MachineSummary(handle, MachinePlatform.Linux)
    private val machine = PairedMachine(
        handle = handle,
        label = requireNotNull(MachineLabel.parse("Devbox")),
        origin = requireNotNull(MachineOrigin.parse("https://devbox.example.ts.net:8443")),
    )
    private val profile = requireNotNull(ProfileKey.parse("personal"))

    @Test
    fun `strict directory decoder projects the bound Home listing and exact request`() {
        val listing = decodeDirectoryListingResponse(
            listingJson(
                directory = "~",
                children = listOf(
                    "~/.hidden" to "Directory",
                    "~/Alpha" to "Directory",
                    "~/alpha" to "SymbolicLink",
                    "~/iz" to "Directory",
                    "~/İa" to "Directory",
                ),
                omitted = true,
            ),
            summary,
        )

        assertEquals(summary, listing.machine)
        assertEquals(HomeDirectory.Home, listing.directory)
        assertEquals(ParentDirectory.Absent, listing.parent)
        assertEquals(DirectoryOmissions.Present, listing.omissions)
        assertEquals(
            listOf("~/.hidden", "~/Alpha", "~/alpha", "~/iz", "~/İa"),
            listing.children.map { it.directory.encoded },
        )
        assertEquals(
            listOf(
                DirectoryEntryKind.Directory,
                DirectoryEntryKind.Directory,
                DirectoryEntryKind.SymbolicLink,
                DirectoryEntryKind.Directory,
                DirectoryEntryKind.Directory,
            ),
            listing.children.map(DirectoryEntry::kind),
        )
        assertTrue(listing.children.first().directory.hidden)
        assertEquals("Alpha", listing.children[1].directory.basename)
        assertEquals("{\"directory\":\"~/Work\"}", encodeDirectoryListingRequest(home("~/Work")))
    }

    @Test
    fun `directory listing request binds method path auth machine and exact body`() {
        val credential = MachineCredential(
            machine,
            requireNotNull(GatewayBearer.parse("A".repeat(43))),
        )
        val request = GatewayClient().directoryListingRequest(credential, home("~/Work tree"))
        val body = Buffer().also { buffer -> checkNotNull(request.body).writeTo(buffer) }.readUtf8()

        assertEquals("POST", request.method)
        assertEquals("https://devbox.example.ts.net:8443/v1/directory-listings", request.url.toString())
        assertEquals("Bearer ${"A".repeat(43)}", request.header("Authorization"))
        assertEquals(handle.encoded, request.header("Skidbladnir-Machine"))
        assertEquals("application/json", request.header("Accept"))
        assertEquals("application/json; charset=utf-8", request.body?.contentType().toString())
        assertEquals("{\"directory\":\"~/Work tree\"}", body)
    }

    @Test
    fun `strict directory decoder rejects identity shape topology order and bounds disagreements`() {
        val valid = listingJson(
            directory = "~/Work",
            children = listOf(
                "~/Work/Alpha" to "Directory",
                "~/Work/beta" to "SymbolicLink",
            ),
        )
        val wrongHandle = valid.replace(handle.encoded, otherHandle.encoded)
        val wrongPlatform = valid.replace("\"platform\":\"Linux\"", "\"platform\":\"Darwin\"")
        val invalid = listOf(
            valid.replace("\"omitted\":false", "\"omitted\":false,\"extra\":true"),
            valid.replace("\"parentDirectory\":\"~\"", "\"parentDirectory\":null"),
            wrongHandle,
            wrongPlatform,
            valid.replace("\"directory\":\"~/Work\"", "\"directory\":\"/Work\""),
            valid.replace("\"directory\":\"~/Work\"", "\"directory\":\"~/Work/\""),
            valid.replace("\"parentDirectory\":\"~\"", "\"parentDirectory\":\"~/Elsewhere\""),
            valid.replace("~/Work/Alpha", "~/Elsewhere/Alpha"),
            valid.replace("~/Work/beta", "~/Work/Alpha"),
            valid.replace(
                "{\"directory\":\"~/Work/Alpha\",\"kind\":\"Directory\"},{\"directory\":\"~/Work/beta\",\"kind\":\"SymbolicLink\"}",
                "{\"directory\":\"~/Work/beta\",\"kind\":\"SymbolicLink\"},{\"directory\":\"~/Work/Alpha\",\"kind\":\"Directory\"}",
            ),
            valid.replace("\"kind\":\"Directory\"", "\"kind\":\"File\""),
            valid.replace("~/Work/Alpha", "~/Work/.."),
            valid.replace("~/Work/Alpha", "~/Work/A\u202eB"),
        )

        invalid.forEachIndexed { index, encoded ->
            assertThrows("accepted invalid listing case $index", ProtocolDecodeException::class.java) {
                decodeDirectoryListingResponse(encoded, summary)
            }
        }

        val tooMany = (0..MAXIMUM_DIRECTORY_LISTING_CHILDREN).map { index ->
            "~/d${index.toString().padStart(3, '0')}" to "Directory"
        }
        assertThrows(ProtocolDecodeException::class.java) {
            decodeDirectoryListingResponse(listingJson("~", tooMany), summary)
        }

        val tooMuchPathText = (0..8).map { index ->
            "~/a$index${"x".repeat(3_800)}" to "Directory"
        }
        assertThrows(ProtocolDecodeException::class.java) {
            decodeDirectoryListingResponse(listingJson("~", tooMuchPathText), summary)
        }
    }

    @Test
    fun `directory ordering exactly matches the server ASCII fold and unsigned UTF8 tie break`() {
        val accepted = listingJson(
            "~",
            listOf(
                "~/A" to "Directory",
                "~/a" to "Directory",
                "~/iz" to "Directory",
                "~/İa" to "Directory",
            ),
        )
        assertEquals(
            listOf("~/A", "~/a", "~/iz", "~/İa"),
            decodeDirectoryListingResponse(accepted, summary).children.map { it.directory.encoded },
        )
        assertThrows(ProtocolDecodeException::class.java) {
            decodeDirectoryListingResponse(
                listingJson(
                    "~",
                    listOf(
                        "~/A" to "Directory",
                        "~/a" to "Directory",
                        "~/İa" to "Directory",
                        "~/iz" to "Directory",
                    ),
                ),
                summary,
            )
        }
    }

    @Test
    fun `browse and exact path types enforce separate bounded grammars`() {
        listOf("~", "~/Alpha", "~/a b", "~/å/東京").forEach { candidate ->
            assertNotNull("rejected canonical Home token $candidate", HomeDirectory.parse(candidate))
        }
        assertNotNull(HomeDirectory.parse("~/" + "x".repeat(4_094)))
        listOf(
            "", "/absolute", "relative", "~user", "~/", "~/a/", "~/a//b", "~/./a",
            "~/a/../b", "~/a\u0000b", "~/a\u2066b", "~/" + "x".repeat(4_095),
        ).forEach { candidate ->
            assertNull("accepted invalid Home token", HomeDirectory.parse(candidate))
        }

        listOf("~", "~/", "~/a/../b", "/", "/srv//work/").forEach { candidate ->
            assertNotNull("rejected valid exact candidate $candidate", WorkingDirectoryPath.parse(candidate))
        }
        listOf("", "relative", "~user", "/bad\u0000path", "/bad\u202epath").forEach { candidate ->
            assertNull("accepted invalid exact candidate", WorkingDirectoryPath.parse(candidate))
        }
        assertNull(WorkingDirectoryPath.parse("/" + "x".repeat(MAXIMUM_WORKING_DIRECTORY_BYTES)))
    }

    @Test
    fun `every HTTP route owns a closed error set after listing codes join the global enum`() {
        val listingUnavailable = errorJson(ApiErrorCode.DirectoryListingUnavailable)
        val nonListingDecoders: List<(Int, String) -> GatewayFailure> = listOf(
            ::decodeGatewayHttpFailure,
            ::decodeSessionsHttpFailure,
            ::decodePressureHttpFailure,
            ::decodeCreateHttpFailure,
            ::decodeKillHttpFailure,
            ::decodePairingHttpFailure,
            ::decodeRenameHttpFailure,
        )
        nonListingDecoders.forEachIndexed { index, decoder ->
            assertThrows("route $index admitted a listing-only code", ProtocolDecodeException::class.java) {
                decoder(422, listingUnavailable)
            }
        }
        assertEquals(
            GatewayFailure.Api(ApiErrorCode.DirectoryListingUnavailable),
            decodeDirectoryListingHttpFailure(422, listingUnavailable),
        )
        assertEquals(
            GatewayFailure.Api(ApiErrorCode.DirectoryListingTooLarge),
            decodeDirectoryListingHttpFailure(422, errorJson(ApiErrorCode.DirectoryListingTooLarge)),
        )
        assertThrows(ProtocolDecodeException::class.java) {
            decodeDirectoryListingHttpFailure(422, errorJson(ApiErrorCode.WorkingDirectoryUnavailable))
        }
        assertEquals(GatewayFailure.Transport, decodeDirectoryListingHttpFailure(503, ""))
    }

    @Test
    fun `Places projects Active only from exact Fresh inventory and selection owns only cwd failure`() {
        val ready = readyMachine(
            session("${'$'}1", "one", "/z"),
            session("${'$'}2", "two", "/a"),
            session("${'$'}3", "three", "/A"),
            session("${'$'}4", "four", "/a"),
            session("${'$'}5", "five", null),
        )
        val initial = forge(cwd = "~/old")
        val opened = requireNotNull(openWorkingDirectoryPicker(initial, ready, pickerInstance = 41))
        val picker = opened.picker()

        assertNull("the picker surface must never admit Create", opened.admissibleSubmission())

        assertEquals(41, picker.instance)
        assertEquals(machine, picker.machine)
        assertEquals(summary, picker.machineSummary)
        assertEquals(listOf("/A", "/a", "/z"), picker.activeDirectories.map { it.encoded })
        assertEquals("~/old", picker.exactDraft)
        assertEquals(WorkingDirectoryPage.Places, picker.page)
        assertEquals(1, picker.nextSequence)

        assertNull(openWorkingDirectoryPicker(initial, ready.copy(inventory = InventoryState.Reading), 42))
        val stale = ready.copy(
            inventory = (ready.inventory as InventoryState.Fresh).let {
                InventoryState.Stale(it.snapshot, GatewayFailure.Transport)
            },
        )
        assertNull(openWorkingDirectoryPicker(initial, stale, 42))
        assertNull(openWorkingDirectoryPicker(initial.copy(pending = true), ready, 42))
        assertNull(openWorkingDirectoryPicker(initial.copy(form = initial.form.copy(machineHandle = otherHandle)), ready, 42))

        val cwdRejected = opened.copy(
            failure = ForgeFailure.Definite(GatewayFailure.Api(ApiErrorCode.WorkingDirectoryUnavailable)),
        )
        val selected = chooseActiveWorkingDirectory(cwdRejected, picker.activeDirectories.last())
        assertEquals("/z", selected.form.cwd)
        assertEquals(ForgeFailure.None, selected.failure)
        assertEquals(ForgeSurface.Form, selected.surface)
        assertNotNull(selected.admissibleSubmission())

        val profileRejected = opened.copy(
            failure = ForgeFailure.Definite(GatewayFailure.Api(ApiErrorCode.ProfileUnknown)),
        )
        val selectedWithUnrelatedFailure = chooseActiveWorkingDirectory(
            profileRejected,
            picker.activeDirectories.first(),
        )
        assertEquals("/A", selectedWithUnrelatedFailure.form.cwd)
        assertEquals(profileRejected.failure, selectedWithUnrelatedFailure.failure)
        assertEquals(profileRejected, chooseActiveWorkingDirectory(profileRejected, path("/not-active")))
    }

    @Test
    fun `browse loading failure retry history filter hidden viewport Parent and Back stay truthful`() {
        var forge = requireNotNull(openWorkingDirectoryPicker(forge(), readyMachine(), 5))
        val homeStart = requireNotNull(browseWorkingDirectoryHome(forge.picker(), generation = 7))
        assertEquals(7, homeStart.request.generation)
        assertEquals(summary, homeStart.request.machine)
        assertEquals(5, homeStart.request.pickerInstance)
        assertEquals(1, homeStart.request.sequence)
        assertEquals(HomeDirectory.Home, homeStart.request.directory)
        assertEquals(2, homeStart.picker.nextSequence)
        val loadingForge = forge.withPicker(homeStart.picker)
        assertEquals(loadingForge, useCurrentWorkingDirectory(loadingForge))

        val homeListing = listing(
            "~",
            listOf("~/.cache", "~/Alpha", "~/alphabet", "~/palp"),
        )
        var picker = updated(
            completeWorkingDirectoryRequest(
                homeStart.picker,
                homeStart.request,
                foregroundGeneration = 7,
                result = GatewayResult.Success(homeListing),
            ),
        )
        picker = updateWorkingDirectoryFilter(picker, "alp")
        picker = setWorkingDirectoryHidden(picker, true)
        val anchor = DirectoryViewport.Anchor(home("~/Alpha"), 17)
        picker = updateWorkingDirectoryViewport(picker, anchor)
        assertEquals(
            listOf("~/Alpha", "~/alphabet", "~/palp"),
            visibleWorkingDirectoryEntries(picker.view()).map { it.directory.encoded },
        )
        assertTrue(workingDirectoryHasHiddenEntries(picker.view()))

        val childStart = requireNotNull(openWorkingDirectoryChild(picker, home("~/Alpha"), generation = 7))
        assertNull(openWorkingDirectoryChild(childStart.picker, home("~/alphabet"), generation = 7))
        assertNull(openWorkingDirectoryParent(childStart.picker, generation = 7))
        forge = forge.withPicker(childStart.picker)
        assertEquals(forge, useCurrentWorkingDirectory(forge))

        val failed = updated(
            completeWorkingDirectoryRequest(
                childStart.picker,
                childStart.request,
                7,
                GatewayResult.Failure(GatewayFailure.Api(ApiErrorCode.DirectoryListingUnavailable)),
            ),
        )
        val failedLoad = failed.load() as DirectoryLoad.Failed
        assertEquals(DirectoryBrowseFailure.Unavailable, failedLoad.failure)
        assertEquals("alp", failed.view().filter)
        assertTrue(failed.view().showHidden)
        assertEquals(anchor, failed.view().viewport)
        assertEquals("~", failed.view().listing.directory.encoded)
        assertEquals("~", useCurrentWorkingDirectory(forge.withPicker(failed)).form.cwd)

        val retry = requireNotNull(retryWorkingDirectory(failed, generation = 7))
        assertEquals(3, retry.request.sequence)
        assertEquals(home("~/Alpha"), retry.request.directory)
        val childListing = listing("~/Alpha", listOf("~/Alpha/Child"))
        picker = updated(
            completeWorkingDirectoryRequest(
                retry.picker,
                retry.request,
                7,
                GatewayResult.Success(childListing),
            ),
        )
        assertEquals(1, picker.history.size)
        assertEquals("alp", picker.history.single().filter)
        assertEquals("", picker.view().filter)
        assertFalse(picker.view().showHidden)
        assertEquals(DirectoryViewport.Top, picker.view().viewport)

        val parent = requireNotNull(openWorkingDirectoryParent(picker, generation = 7))
        assertEquals(HomeDirectory.Home, parent.request.directory)
        val backFromLoading = workingDirectoryBack(forge.withPicker(parent.picker)).picker()
        assertEquals("~/Alpha", backFromLoading.view().listing.directory.encoded)

        val back = workingDirectoryBack(forge.withPicker(picker)).picker()
        assertEquals("~", back.view().listing.directory.encoded)
        assertEquals("alp", back.view().filter)
        assertTrue(back.view().showHidden)
        assertEquals(anchor, back.view().viewport)
        assertEquals(WorkingDirectoryPage.Places, workingDirectoryBack(forge.withPicker(back)).picker().page)
    }

    @Test
    fun `Back from failed browse restores retained view without consuming older history`() {
        val older = DirectoryView(
            listing("~", listOf("~/Current")),
            "older filter",
            true,
            DirectoryViewport.Top,
        )
        val retained = DirectoryView(
            listing("~/Current", listOf("~/Current/Child")),
            "current filter",
            false,
            DirectoryViewport.Anchor(home("~/Current/Child"), 9),
        )
        val failed = loadedPicker(retained.listing).copy(
            page = WorkingDirectoryPage.Browsing(
                DirectoryLoad.Failed(
                    candidate = home("~/Current/Child"),
                    retained = RetainedDirectoryView.Present(retained),
                    failure = DirectoryBrowseFailure.Transport,
                ),
            ),
            history = listOf(older),
        )

        assertEquals(
            failed.copy(
                page = WorkingDirectoryPage.Browsing(DirectoryLoad.Loaded(retained)),
            ),
            workingDirectoryBack(forge().withPicker(failed)).picker(),
        )
    }

    @Test
    fun `foreground invalidation settles only an active loading directory request`() {
        val opened = requireNotNull(openWorkingDirectoryPicker(forge(), readyMachine(), 78))
        val withoutRetained = requireNotNull(
            browseWorkingDirectoryHome(opened.picker(), generation = 31),
        )
        val loadingWithoutRetained = opened.withPicker(withoutRetained.picker)
        assertEquals(
            loadingWithoutRetained.withPicker(
                withoutRetained.picker.copy(page = WorkingDirectoryPage.Places),
            ),
            workingDirectoryPickerAfterForegroundInvalidation(loadingWithoutRetained),
        )

        val older = DirectoryView(
            listing("~", listOf("~/Current")),
            "older filter",
            true,
            DirectoryViewport.Top,
        )
        val retained = DirectoryView(
            listing("~/Current", listOf("~/Current/Child")),
            "current filter",
            false,
            DirectoryViewport.Anchor(home("~/Current/Child"), 9),
        )
        val retainedPicker = loadedPicker(retained.listing).copy(
            page = WorkingDirectoryPage.Browsing(DirectoryLoad.Loaded(retained)),
            history = listOf(older),
            exactDraft = "/outside/Home",
            nextSequence = 12,
        )
        val withRetained = requireNotNull(
            openWorkingDirectoryChild(retainedPicker, home("~/Current/Child"), generation = 31),
        )
        val loadingWithRetained = opened.withPicker(withRetained.picker)
        assertEquals(
            loadingWithRetained.withPicker(
                withRetained.picker.copy(
                    page = WorkingDirectoryPage.Browsing(DirectoryLoad.Loaded(retained)),
                ),
            ),
            workingDirectoryPickerAfterForegroundInvalidation(loadingWithRetained),
        )

        val failedWithoutRetained = withoutRetained.picker.copy(
            page = WorkingDirectoryPage.Browsing(
                DirectoryLoad.Failed(
                    candidate = HomeDirectory.Home,
                    retained = RetainedDirectoryView.None,
                    failure = DirectoryBrowseFailure.Transport,
                ),
            ),
        )
        val failedWithRetained = withRetained.picker.copy(
            page = WorkingDirectoryPage.Browsing(
                DirectoryLoad.Failed(
                    candidate = home("~/Current/Child"),
                    retained = RetainedDirectoryView.Present(retained),
                    failure = DirectoryBrowseFailure.Unavailable,
                ),
            ),
        )
        val loaded = retainedPicker
        val exactFromPlaces = showExactWorkingDirectory(opened).picker()
        val exactFromBrowse = showExactWorkingDirectory(opened.withPicker(loaded)).picker()
        val unchanged = listOf(
            forge(),
            opened,
            opened.withPicker(loaded),
            opened.withPicker(failedWithoutRetained),
            opened.withPicker(failedWithRetained),
            opened.withPicker(exactFromPlaces),
            opened.withPicker(exactFromBrowse),
        )
        unchanged.forEachIndexed { index, state ->
            assertEquals(
                "foreground invalidation changed non-loading state $index",
                state,
                workingDirectoryPickerAfterForegroundInvalidation(state),
            )
        }
    }

    @Test
    fun `filter ranks locally with bounded Unicode scalars and hidden is presentation only`() {
        val decoded = listing(
            "~",
            listOf("~/.alp", "~/aleph", "~/alp", "~/alphabet", "~/palp"),
        )
        var picker = loadedPicker(decoded)
        picker = updateWorkingDirectoryFilter(picker, "ALP")
        assertEquals(
            listOf("~/alp", "~/alphabet", "~/palp", "~/aleph"),
            visibleWorkingDirectoryEntries(picker.view()).map { it.directory.encoded },
        )
        picker = setWorkingDirectoryHidden(picker, true)
        assertEquals(
            listOf("~/alp", "~/alphabet", "~/.alp", "~/palp", "~/aleph"),
            visibleWorkingDirectoryEntries(picker.view()).map { it.directory.encoded },
        )
        assertEquals(5, picker.view().listing.children.size)

        val maximum = "😀".repeat(256)
        val bounded = updateWorkingDirectoryFilter(picker, maximum)
        assertEquals(maximum, bounded.view().filter)
        assertSame(bounded, updateWorkingDirectoryFilter(bounded, maximum + "😀"))
        val missingAnchor = DirectoryViewport.Anchor(home("~/missing"), 1)
        assertEquals(bounded, updateWorkingDirectoryViewport(bounded, missingAnchor))

        val visibleAnchor = DirectoryViewport.Anchor(home("~/alp"), 7)
        val anchoredVisible = updateWorkingDirectoryViewport(
            updateWorkingDirectoryFilter(picker, ""),
            visibleAnchor,
        )
        assertEquals(
            DirectoryViewport.Top,
            updateWorkingDirectoryFilter(anchoredVisible, "aleph").view().viewport,
        )

        val hiddenAnchor = DirectoryViewport.Anchor(home("~/.alp"), 11)
        val anchoredHidden = updateWorkingDirectoryViewport(anchoredVisible, hiddenAnchor)
        assertEquals(
            DirectoryViewport.Top,
            setWorkingDirectoryHidden(anchoredHidden, false).view().viewport,
        )
    }

    @Test
    fun `exact page retains expert input rejects over-limit edits and selects byte-for-byte`() {
        for (code in listOf(
            ApiErrorCode.WorkingDirectoryInvalid,
            ApiErrorCode.WorkingDirectoryUnavailable,
        )) {
            val rejection = ForgeFailure.Definite(GatewayFailure.Api(code))
            assertTrue(rejection.isWorkingDirectoryRejection())
            assertNotNull(
                openExactWorkingDirectoryPicker(forge(cwd = "~/old", failure = rejection), readyMachine(), 8),
            )
        }
        assertFalse(
            ForgeFailure.Definite(GatewayFailure.Api(ApiErrorCode.ProfileUnknown))
                .isWorkingDirectoryRejection(),
        )
        var forge = requireNotNull(
            openExactWorkingDirectoryPicker(
                forge(
                    cwd = "~/old",
                    failure = ForgeFailure.Definite(
                        GatewayFailure.Api(ApiErrorCode.WorkingDirectoryInvalid),
                    ),
                ),
                readyMachine(),
                9,
            ),
        )
        assertEquals(ExactPathValidation.Valid, forge.picker().exactPage().validation)

        forge = updateExactWorkingDirectory(forge, "relative")
        assertEquals("relative", forge.picker().exactDraft)
        assertEquals(ExactPathValidation.Invalid, forge.picker().exactPage().validation)
        assertEquals(forge, useExactWorkingDirectory(forge))

        forge = updateExactWorkingDirectory(forge, "")
        assertEquals(ExactPathValidation.Pristine, forge.picker().exactPage().validation)
        forge = useExactWorkingDirectory(forge)
        assertEquals(ExactPathValidation.Invalid, forge.picker().exactPage().validation)

        forge = updateExactWorkingDirectory(forge, "/srv//work/../opaque/")
        assertEquals(ExactPathValidation.Valid, forge.picker().exactPage().validation)
        val beforeOverLimit = forge
        forge = updateExactWorkingDirectory(forge, "/" + "x".repeat(MAXIMUM_WORKING_DIRECTORY_BYTES))
        assertEquals(beforeOverLimit, forge)
        assertEquals("/srv//work/../opaque/", forge.picker().exactDraft)
        assertEquals(ExactPathValidation.Valid, forge.picker().exactPage().validation)

        val back = workingDirectoryBack(forge)
        assertEquals(WorkingDirectoryPage.Places, back.picker().page)
        val reopened = showExactWorkingDirectory(back)
        assertEquals("/srv//work/../opaque/", reopened.picker().exactDraft)
        val selected = useExactWorkingDirectory(reopened)
        assertEquals("/srv//work/../opaque/", selected.form.cwd)
        assertEquals(ForgeFailure.None, selected.failure)
        assertEquals(ForgeSurface.Form, selected.surface)
    }

    @Test
    fun `Cancel and machine change close the picker while same-machine form updates do not`() {
        val opened = requireNotNull(openWorkingDirectoryPicker(forge(), readyMachine(), 10))
        assertEquals(ForgeSurface.Form, cancelWorkingDirectoryPicker(opened).surface)

        val sameMachine = updateForgeState(opened, opened.form.copy(objective = "new objective"))
        assertTrue(sameMachine.surface is ForgeSurface.DirectoryPicker)
        assertEquals("new objective", sameMachine.form.objective)

        val changed = updateForgeState(
            opened,
            opened.form.copy(machineHandle = otherHandle, cwd = "/must-clear", profile = profile),
        )
        assertEquals(otherHandle, changed.form.machineHandle)
        assertEquals("", changed.form.cwd)
        assertNull(changed.form.profile)
        assertEquals(ForgeSurface.Form, changed.surface)
    }

    @Test
    fun `access loss closes an active picker and stores the typed Forge rejection`() {
        val ready = readyMachine()
        val opened = requireNotNull(openWorkingDirectoryPicker(forge(), ready, 11))
        val dashboardEntry = DashboardEntryState().apply {
            acceptFleet(setOf(handle))
        }
        val dashboard = SkidbladnirUiState.Dashboard(
            machines = listOf(ready),
            refreshing = false,
            forge = opened,
            forgeRecovery = null,
            kill = null,
        )
        val failure = GatewayFailure.Api(ApiErrorCode.Unauthenticated)
        val unavailable = ready.copy(access = MachineAccess.AuthRequired)

        val updated = dashboardAfterMachineAccessLoss(
            dashboard = dashboard,
            machines = listOf(unavailable),
            handle = handle,
            refreshing = false,
            dashboardEntry = dashboardEntry,
            failure = failure,
        )

        assertEquals(ForgeSurface.Form, requireNotNull(updated.forge).surface)
        assertEquals(ForgeFailure.Definite(failure), updated.forge.failure)
        assertFalse(updated.forge.pending)
        assertEquals(DashboardScope.Machine(handle), dashboardEntry.scope)
    }

    @Test
    fun `request completion is fenced by foreground machine chooser latest sequence and loading candidate`() {
        val base = requireNotNull(openWorkingDirectoryPicker(forge(), readyMachine(), 12)).picker()
        val start = requireNotNull(browseWorkingDirectoryHome(base, generation = 99))
        val result = GatewayResult.Success(listing("~", listOf("~/Alpha")))
        val wrongSummary = summary.copy(platform = MachinePlatform.Darwin)

        val ignored = listOf(
            completeWorkingDirectoryRequest(start.picker, start.request, null, result),
            completeWorkingDirectoryRequest(start.picker, start.request, 100, result),
            completeWorkingDirectoryRequest(start.picker, start.request.copy(machine = wrongSummary), 99, result),
            completeWorkingDirectoryRequest(start.picker.copy(instance = 13), start.request, 99, result),
            completeWorkingDirectoryRequest(start.picker.copy(nextSequence = 4), start.request, 99, result),
            completeWorkingDirectoryRequest(start.picker.copy(page = WorkingDirectoryPage.Places), start.request, 99, result),
        )
        ignored.forEach { completion -> assertEquals(WorkingDirectoryCompletion.Ignored, completion) }

        val accepted = completeWorkingDirectoryRequest(start.picker, start.request, 99, result)
        assertTrue(accepted is WorkingDirectoryCompletion.Updated)
        assertThrows(ProtocolDecodeException::class.java) {
            completeWorkingDirectoryRequest(
                start.picker,
                start.request,
                99,
                GatewayResult.Success(listing("~/Elsewhere", emptyList())),
            )
        }

        listOf(ApiErrorCode.Unauthenticated, ApiErrorCode.MachineIdentityMismatch).forEach { code ->
            assertEquals(
                WorkingDirectoryCompletion.AccessLost(GatewayFailure.Api(code)),
                completeWorkingDirectoryRequest(
                    start.picker,
                    start.request,
                    99,
                    GatewayResult.Failure(GatewayFailure.Api(code)),
                ),
            )
        }
        val typedFailures = mapOf(
            GatewayFailure.Transport to DirectoryBrowseFailure.Transport,
            GatewayFailure.Api(ApiErrorCode.DirectoryListingUnavailable) to DirectoryBrowseFailure.Unavailable,
            GatewayFailure.Api(ApiErrorCode.DirectoryListingTooLarge) to DirectoryBrowseFailure.TooLarge,
            GatewayFailure.Api(ApiErrorCode.InternalError) to DirectoryBrowseFailure.Internal,
        )
        typedFailures.forEach { (failure, expected) ->
            val completion = updated(
                completeWorkingDirectoryRequest(
                    start.picker,
                    start.request,
                    99,
                    GatewayResult.Failure(failure),
                ),
            )
            assertEquals(expected, (completion.load() as DirectoryLoad.Failed).failure)
        }
        listOf(ApiErrorCode.InvalidRequest, ApiErrorCode.RequestTooLarge).forEach { code ->
            val defect = assertThrows(ProtocolDecodeException::class.java) {
                completeWorkingDirectoryRequest(
                    start.picker,
                    start.request,
                    99,
                    GatewayResult.Failure(GatewayFailure.Api(code)),
                )
            }
            assertEquals(
                "Protocol payload could not be decoded: directory-listing completion error set.",
                defect.message,
            )
        }
    }

    @Test
    fun `Back history retains only the latest 32 decoded views then returns to Places`() {
        var picker = loadedPicker(listing("~", listOf("~/d0")))
        var current = "~"
        repeat(33) { depth ->
            val next = if (current == "~") "~/d0" else "$current/d${depth}"
            val start = requireNotNull(openWorkingDirectoryChild(picker, home(next), generation = 1))
            val child = listing(next, listOf("$next/d${depth + 1}"))
            picker = updated(
                completeWorkingDirectoryRequest(
                    start.picker,
                    start.request,
                    1,
                    GatewayResult.Success(child),
                ),
            )
            current = next
        }
        assertEquals(32, picker.history.size)
        assertNotEquals("~", picker.history.first().listing.directory.encoded)

        var forge = forge().withPicker(picker)
        repeat(32) { forge = workingDirectoryBack(forge) }
        assertEquals("~/d0", forge.picker().view().listing.directory.encoded)
        forge = workingDirectoryBack(forge)
        assertEquals(WorkingDirectoryPage.Places, forge.picker().page)
        assertEquals(ForgeSurface.Form, workingDirectoryBack(forge).surface)
    }

    private fun home(encoded: String): HomeDirectory = requireNotNull(HomeDirectory.parse(encoded))
    private fun path(encoded: String): WorkingDirectoryPath = requireNotNull(WorkingDirectoryPath.parse(encoded))

    private fun listing(
        directory: String,
        children: List<String>,
        omitted: Boolean = false,
    ): DirectoryListing = decodeDirectoryListingResponse(
        listingJson(directory, children.map { it to "Directory" }, omitted = omitted),
        summary,
    )

    private fun listingJson(
        directory: String,
        children: List<Pair<String, String>>,
        omitted: Boolean = false,
        machineHandle: MachineHandle = handle,
        platform: String = "Linux",
    ): String {
        val parent = if (directory == "~") {
            ""
        } else {
            ",\"parentDirectory\":\"${directory.substringBeforeLast('/').ifEmpty { "~" }}\""
        }
        val entries = children.joinToString(",") { (path, kind) ->
            "{\"directory\":\"$path\",\"kind\":\"$kind\"}"
        }
        return "{\"machine\":{\"handle\":\"${machineHandle.encoded}\",\"platform\":\"$platform\"}," +
            "\"directory\":\"$directory\"$parent,\"children\":[$entries],\"omitted\":$omitted}"
    }

    private fun errorJson(code: ApiErrorCode): String =
        "{\"code\":\"${code.wireName}\",\"message\":\"${apiErrorMessage(code)}\"}"

    private fun forge(
        cwd: String = "~",
        failure: ForgeFailure = ForgeFailure.None,
    ): ForgeState = ForgeState(
        form = ForgeForm(
            machineHandle = handle,
            cwd = cwd,
            profile = profile,
            optionalTmuxName = "",
            objective = "",
        ),
        pending = false,
        failure = failure,
        surface = ForgeSurface.Form,
    )

    private fun readyMachine(vararg sessions: TmuxSession): MachineState = MachineState(
        machine = machine,
        access = MachineAccess.Ready,
        inventory = InventoryState.Fresh(
            InventorySnapshot(
                inventory = SessionsResponse(
                    machine = summary,
                    observedAt = Instant.parse("2026-08-31T12:00:00Z"),
                    profiles = listOf(ProfileChoice(profile, "Personal", AgentProvider.Codex)),
                    sessions = sessions.toList(),
                ),
                receivedAtElapsedMillis = 1,
            ),
        ),
        pressure = PressureState.Reading,
    )

    private fun session(id: String, name: String, cwd: String?): TmuxSession = TmuxSession(
        tmuxId = id,
        tmuxName = name,
        identityToken = "token-$name",
        character = CharacterSummary("skuld", "Skuld"),
        cwd = cwd,
        attachedClients = 0,
        activity = SessionActivity.Quiet,
    )

    private fun loadedPicker(listing: DirectoryListing): WorkingDirectoryPickerState =
        requireNotNull(openWorkingDirectoryPicker(forge(), readyMachine(), 77)).picker().copy(
            page = WorkingDirectoryPage.Browsing(
                DirectoryLoad.Loaded(
                    DirectoryView(listing, "", false, DirectoryViewport.Top),
                ),
            ),
        )

    private fun ForgeState.picker(): WorkingDirectoryPickerState =
        (surface as ForgeSurface.DirectoryPicker).picker

    private fun ForgeState.withPicker(picker: WorkingDirectoryPickerState): ForgeState =
        copy(surface = ForgeSurface.DirectoryPicker(picker))

    private fun WorkingDirectoryPickerState.load(): DirectoryLoad =
        (page as WorkingDirectoryPage.Browsing).load

    private fun WorkingDirectoryPickerState.view(): DirectoryView =
        requireNotNull(retainedWorkingDirectoryView(this))

    private fun WorkingDirectoryPickerState.exactPage(): WorkingDirectoryPage.ExactPath =
        page as WorkingDirectoryPage.ExactPath

    private fun updated(completion: WorkingDirectoryCompletion): WorkingDirectoryPickerState =
        (completion as WorkingDirectoryCompletion.Updated).picker
}
