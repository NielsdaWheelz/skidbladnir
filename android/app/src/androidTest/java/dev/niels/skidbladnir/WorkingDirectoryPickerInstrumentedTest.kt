package dev.niels.skidbladnir

import android.content.Context
import android.view.KeyEvent
import androidx.activity.compose.setContent
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.test.SemanticsNodeInteraction
import androidx.compose.ui.test.assertContentDescriptionEquals
import androidx.compose.ui.test.assertCountEquals
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsEnabled
import androidx.compose.ui.test.assertIsFocused
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.assertTextContains
import androidx.compose.ui.test.getUnclippedBoundsInRoot
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasTestTag
import androidx.compose.ui.test.junit4.v2.createEmptyComposeRule
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performImeAction
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.performScrollToNode
import androidx.compose.ui.test.performTextClearance
import androidx.compose.ui.test.performTextInput
import androidx.compose.ui.test.performTextReplacement
import androidx.compose.ui.test.performSemanticsAction
import androidx.compose.ui.unit.Density
import androidx.compose.ui.unit.dp
import androidx.compose.ui.semantics.SemanticsActions
import androidx.test.core.app.ActivityScenario
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import java.security.KeyStore
import java.util.concurrent.LinkedBlockingQueue
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicInteger
import java.util.concurrent.atomic.AtomicReference
import kotlin.math.absoluteValue
import okhttp3.HttpUrl.Companion.toHttpUrlOrNull
import okhttp3.Interceptor
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Protocol
import okhttp3.Request
import okhttp3.Response
import okhttp3.ResponseBody.Companion.toResponseBody
import okio.Buffer
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class WorkingDirectoryPickerInstrumentedTest {
    @get:Rule
    val compose = createEmptyComposeRule()

    private val context: Context = InstrumentationRegistry.getInstrumentation().targetContext
    private val storage = MachineStorage(TEST_PREFERENCES, TEST_KEY_ALIAS)

    @Before
    fun clearFixture() = resetFixture()

    @After
    fun removeFixture() = resetFixture()

    @Test
    fun workingDirectorySelectionIsTouchFirstTruthfulAndSeparateFromCreate() {
        assertEquals(
            FleetInstallation.Installed,
            MachineStore(context, storage).installFixedFleet(FLEET),
        )
        val boundary = ExternalMachineBoundary(FLEET)
        val client = GatewayClient(
            OkHttpClient.Builder()
                .addInterceptor(boundary)
                .retryOnConnectionFailure(false)
                .followRedirects(false)
                .followSslRedirects(false)
                .build(),
        )
        val dashboardEntry = DashboardEntryState()
        val controller = SkidbladnirController(
            context = context,
            dashboardEntry = dashboardEntry,
            storage = storage,
            client = client,
        )
        var fontScale by mutableStateOf(1f)

        try {
            ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
                scenario.onActivity { activity ->
                    val density = activity.resources.displayMetrics.density
                    activity.setContent {
                        CompositionLocalProvider(
                            LocalDensity provides Density(density, fontScale),
                        ) {
                            NidavellirTheme {
                                SkidbladnirApp(
                                    controller = controller,
                                    dashboardEntry = dashboardEntry,
                                    scanner = FleetScanner(activity),
                                    onTailscale = {},
                                )
                            }
                        }
                    }
                    controller.start()
                }

                waitForEnabledDescription("New dwarf")
                compose.onNodeWithContentDescription("New dwarf").performClick()
                compose.onNodeWithTag("forge-sheet").assertIsDisplayed()

                compose.onNodeWithTag("forge-machine-${DEVBOX.machine.handle.encoded}")
                    .assertIsEnabled()
                    .performClick()
                compose.onNodeWithTag("forge-profile-${DEVBOX.machine.handle.encoded}")
                    .assertIsEnabled()
                    .performClick()
                    .assertIsSelected()
                compose.onNodeWithTag("forge-name")
                    .performScrollTo()
                    .performTextReplacement("kept-name")
                compose.onNodeWithTag("forge-objective")
                    .performScrollTo()
                    .performTextReplacement("kept objective")

                val chooser = compose.onNodeWithTag("forge-working-directory")
                    .performScrollTo()
                    .assertIsDisplayed()
                    .assertIsEnabled()
                    .assertMinimumTarget()
                compose.onNodeWithText("Choose a working directory").assertIsDisplayed()
                chooser.performClick()

                compose.onNodeWithTag("working-directory-picker").assertIsDisplayed()
                compose.onNodeWithText("Choose working directory").assertIsDisplayed()
                compose.onNodeWithContentDescription("On ${DEVBOX.machine.label.text}")
                    .assertIsDisplayed()
                compose.onNodeWithText("Browse Home").assertIsDisplayed()
                compose.onNodeWithText("Active on", substring = true).assertIsDisplayed()
                compose.onNodeWithText("Enter exact path").assertIsDisplayed()
                compose.onNodeWithTag("forge-submit").assertDoesNotExist()
                compose.onNodeWithContentDescription(
                    "Browse Home. Opens Home folders on ${DEVBOX.machine.label.text}.",
                ).assertMinimumTarget()
                val active = compose.onNodeWithContentDescription(
                    "Working directory $ACTIVE_DIRECTORY. " +
                        "Selects this directory on ${DEVBOX.machine.label.text}.",
                ).assertMinimumTarget()
                compose.onNodeWithTag(
                    "working-directory-active-path-scroll",
                    useUnmergedTree = true,
                ).assertScrolledToTail()

                active.performClick()
                compose.onNodeWithTag("working-directory-picker").assertDoesNotExist()
                compose.onNodeWithText("Working directory on", substring = true)
                    .performScrollTo()
                    .assertIsDisplayed()
                compose.onNodeWithTag("forge-name").performScrollTo()
                    .assertEditableTextEquals("kept-name")
                compose.onNodeWithTag("forge-objective").performScrollTo()
                    .assertEditableTextEquals("kept objective")
                compose.onNodeWithTag("forge-profile-${DEVBOX.machine.handle.encoded}")
                    .assertIsSelected()

                compose.onNodeWithTag("forge-working-directory-change")
                    .performScrollTo()
                    .assertMinimumTarget()
                    .performClick()
                compose.onNodeWithContentDescription(
                    "Browse Home. Opens Home folders on ${DEVBOX.machine.label.text}.",
                ).performClick()
                waitForDescription("Opening “Home”…")
                compose.onNodeWithContentDescription("Opening “Home”…").assertIsDisplayed()
                compose.onNodeWithText("Parent folder").assertDoesNotExist()
                compose.onNodeWithTag("working-directory-use").assertDoesNotExist()
                compose.onNodeWithText("Enter exact path").assertIsEnabled()
                compose.onNodeWithTag("working-directory-live-region")
                    .assertPoliteLiveRegion()
                boundary.respond(homeListing())

                waitForDescription("Current folder ~")
                compose.onNodeWithTag("working-directory-location", useUnmergedTree = true)
                    .assertMinimumTarget()
                    .assertContentDescriptionEquals("Current folder ~")
                compose.onNodeWithText("Filter folders").assertIsDisplayed()
                compose.onNodeWithText("Show hidden folders").assertMinimumTarget()
                compose.onNodeWithText("Some folders cannot be shown.").assertIsDisplayed()
                compose.onNodeWithContentDescription("Folder Alpha. Opens folder.")
                    .assertMinimumTarget()
                compose.onNodeWithContentDescription("Linked folder current. Opens folder.")
                    .assertMinimumTarget()
                compose.onNodeWithContentDescription("Folder .hidden. Opens folder.")
                    .assertDoesNotExist()
                compose.onNodeWithTag("working-directory-use")
                    .assertMinimumTarget()
                    .assertContentDescriptionEquals(
                        "Use Home as working directory on ${DEVBOX.machine.label.text}.",
                    )

                compose.onNodeWithText("Show hidden folders").performClick()
                compose.onNodeWithContentDescription("Folder .hidden. Opens folder.")
                    .assertIsDisplayed()
                compose.onNodeWithTag("working-directory-filter").performTextInput("missing")
                compose.onNodeWithText("No visible folders here.").assertIsDisplayed()
                compose.onNodeWithTag("working-directory-live-region")
                    .assertPoliteLiveRegion()
                compose.onNodeWithTag("working-directory-filter").performTextClearance()

                scenario.onActivity { fontScale = 2f }
                compose.onNodeWithTag("working-directory-location", useUnmergedTree = true)
                    .assertIsDisplayed()
                compose.onNodeWithTag("working-directory-filter").assertIsDisplayed()
                compose.onNodeWithTag("working-directory-use").assertMinimumTarget()
                compose.onNodeWithTag("working-directory-back").assertMinimumTarget()
                compose.onNodeWithTag("working-directory-cancel").assertMinimumTarget()
                scenario.onActivity { fontScale = 1f }

                compose.onNodeWithContentDescription("Folder Alpha. Opens folder.")
                    .performScrollTo()
                    .performClick()
                waitForDescription("Opening “Alpha”…")
                compose.onNodeWithContentDescription("Opening “Alpha”…").assertIsDisplayed()
                compose.onNodeWithContentDescription("Folder Alpha. Opens folder.")
                    .assertIsNotEnabled()
                compose.onNodeWithTag("working-directory-use").assertIsNotEnabled()
                compose.onNodeWithText("Enter exact path").assertIsEnabled()
                boundary.respond(ListingReply.transport("~/Alpha"))

                waitForText("Could not reach this machine over your Tailnet.")
                compose.onNodeWithContentDescription("Could not open “Alpha”.")
                    .assertIsDisplayed()
                compose.onNodeWithText("Could not reach this machine over your Tailnet.")
                    .assertIsDisplayed()
                compose.onNodeWithText("Try again").assertMinimumTarget()
                compose.onNodeWithText("Enter exact path").assertMinimumTarget()
                compose.onNodeWithTag("working-directory-live-region")
                    .assertPoliteLiveRegion()
                waitForEnabledTag("working-directory-use")
                compose.onNodeWithTag("working-directory-list").performScrollToNode(
                    hasContentDescription("Folder Alpha. Opens folder."),
                )
                compose.waitForIdle()
                compose.onNodeWithContentDescription("Folder Alpha. Opens folder.")
                    .assertIsEnabled()

                compose.waitForIdle()
                compose.onNodeWithText("Try again")
                    .assertIsDisplayed()
                    .assertIsEnabled()
                    .performClick()
                waitForDescription("Opening “Alpha”…")
                boundary.respond(alphaListing())
                waitForDescription("Current folder ~/Alpha")
                compose.onNodeWithText("Parent folder").assertMinimumTarget()
                compose.onNodeWithTag("working-directory-use")
                    .assertContentDescriptionEquals(
                        "Use ~/Alpha as working directory on ${DEVBOX.machine.label.text}.",
                    )
                compose.onNodeWithTag("working-directory-filter").performTextInput("nested")

                val targetDescription = "Folder nested-20. Opens folder."
                compose.onNodeWithTag("working-directory-list").performScrollToNode(
                    hasContentDescription(targetDescription),
                )
                compose.waitForIdle()
                val target = compose.onNodeWithContentDescription(targetDescription)
                    .assertIsDisplayed()
                    .assertIsEnabled()
                val priorBounds = target.getUnclippedBoundsInRoot()
                target.performClick()
                waitForDescription("Opening “nested-20”…")
                boundary.respond(deepListing(2))
                waitForDescription("Current folder ~/Alpha/nested-20")

                InstrumentationRegistry.getInstrumentation()
                    .sendKeyDownUpSync(KeyEvent.KEYCODE_BACK)
                waitForDescription("Current folder ~/Alpha")
                compose.onNodeWithTag("working-directory-picker").assertIsDisplayed()
                compose.onNodeWithTag("working-directory-filter").assertTextContains("nested")
                val platformRestored = compose.onNodeWithContentDescription(targetDescription)
                    .assertIsDisplayed()
                    .assertIsEnabled()
                assertTrue(
                    "System Back must restore the captured row offset inside the picker",
                    (
                        platformRestored.getUnclippedBoundsInRoot().top - priorBounds.top
                    ).value.absoluteValue <= 1f,
                )
                val controlledPriorBounds = platformRestored.getUnclippedBoundsInRoot()
                platformRestored.performClick()
                waitForDescription("Opening “nested-20”…")
                boundary.respond(deepListing(2))
                waitForDescription("Current folder ~/Alpha/nested-20")

                compose.mainClock.autoAdvance = false
                compose.onNodeWithTag("working-directory-back").performClick()
                compose.waitForIdle()
                compose.mainClock.advanceTimeByFrame()
                compose.waitForIdle()
                compose.onNodeWithTag("working-directory-picker").assertIsDisplayed()
                compose.onNodeWithTag("working-directory-use").assertIsNotEnabled()
                val restoredBeforeEnable = compose.onNodeWithContentDescription(targetDescription)
                    .assertIsDisplayed()
                    .assertIsNotEnabled()
                val restoredOffsetDelta = (
                    restoredBeforeEnable.getUnclippedBoundsInRoot().top - controlledPriorBounds.top
                ).value.absoluteValue
                assertTrue(
                    "Back must restore the captured row offset without animation",
                    restoredOffsetDelta <= 1f,
                )
                compose.waitUntil(timeoutMillis = 1_000) {
                    compose.mainClock.advanceTimeByFrame()
                    compose.onAllNodes(hasContentDescription(targetDescription))
                        .fetchSemanticsNodes()
                        .singleOrNull()
                        ?.let { node -> node.config.getOrNull(SemanticsProperties.Disabled) == null }
                        ?: false
                }
                val restored = compose.onNodeWithContentDescription(targetDescription)
                    .assertIsDisplayed()
                    .assertIsEnabled()
                assertTrue(
                    "Back must restore the captured row offset before re-enabling rows",
                    (
                        restored.getUnclippedBoundsInRoot().top - controlledPriorBounds.top
                    ).value.absoluteValue <= 1f,
                )
                compose.onNodeWithTag("working-directory-filter").assertTextContains("nested")
                compose.onNodeWithTag("working-directory-location", useUnmergedTree = true)
                    .assertContentDescriptionEquals("Current folder ~/Alpha")
                compose.mainClock.autoAdvance = true
                compose.waitForIdle()

                compose.onNodeWithTag("working-directory-list").performScrollToNode(
                    hasTestTag("working-directory-parent"),
                )
                compose.onNodeWithTag("working-directory-parent")
                    .assertIsDisplayed()
                    .assertIsEnabled()
                    .performClick()
                waitForDescription("Opening “Home”…")
                boundary.respond(homeListing())
                waitForDescription("Current folder ~")
                compose.onNodeWithTag("working-directory-back").performClick()
                waitForDescription("Current folder ~/Alpha")
                compose.onNodeWithTag("working-directory-filter").assertTextContains("nested")

                compose.onNodeWithTag("working-directory-list").performScrollToNode(
                    hasContentDescription(targetDescription),
                )
                compose.onNodeWithContentDescription(targetDescription)
                    .assertIsDisplayed()
                    .assertIsEnabled()
                    .performClick()
                boundary.respond(deepListing(2))
                waitForDescription("Current folder ${DEEP_DIRECTORIES[1]}")
                for (depth in 3..6) {
                    val directory = DEEP_DIRECTORIES[depth - 1]
                    val basename = directory.substringAfterLast('/')
                    compose.onNodeWithTag("working-directory-list").performScrollToNode(
                        hasContentDescription("Folder $basename. Opens folder."),
                    )
                    compose.onNodeWithContentDescription("Folder $basename. Opens folder.")
                        .assertIsDisplayed()
                        .assertIsEnabled()
                        .performClick()
                    waitForDescription("Opening “$basename”…")
                    boundary.respond(deepListing(depth))
                    waitForDescription("Current folder $directory")
                }
                compose.onNodeWithTag("working-directory-use")
                    .assertContentDescriptionEquals(
                        "Use ${DEEP_DIRECTORIES.last()} as working directory on " +
                            "${DEVBOX.machine.label.text}.",
                    )
                    .performClick()
                compose.onNodeWithTag("working-directory-picker").assertDoesNotExist()
                compose.onNodeWithTag("working-directory-path-scroll")
                    .performScrollTo()
                    .assertMinimumTarget()
                    .assertContentDescriptionEquals(DEEP_DIRECTORIES.last())
                compose.onNodeWithTag("forge-name").performScrollTo()
                    .assertEditableTextEquals("kept-name")
                compose.onNodeWithTag("forge-objective").performScrollTo()
                    .assertEditableTextEquals("kept objective")
                assertEquals("Browse Use must not invoke Create", 0, boundary.createRequests.get())

                compose.onNodeWithTag("forge-working-directory-change")
                    .performScrollTo()
                    .performClick()
                compose.onNodeWithText("Enter exact path").performScrollTo().performClick()
                compose.onNodeWithText("Enter exact path").assertIsDisplayed()
                compose.onNodeWithText("Working directory").assertIsDisplayed()
                compose.onNodeWithText("Use an absolute path or ~/…").assertIsDisplayed()
                val exactField = compose.onNodeWithTag("working-directory-exact-field")
                    .assertIsFocused()
                    .assertTextContains(DEEP_DIRECTORIES.last())
                compose.onNodeWithTag("forge-submit").assertDoesNotExist()

                exactField.performTextClearance()
                exactField.assertEditableTextEquals("")
                exactField.performImeAction()
                compose.onNodeWithText("Choose a valid working directory.")
                    .assertIsDisplayed()
                compose.onNodeWithTag("working-directory-live-region")
                    .assertPoliteLiveRegion()
                val exact = "/srv/" + "long-directory/".repeat(12) + "work"
                exactField.performTextInput(exact)
                exactField.performImeAction()

                compose.onNodeWithTag("working-directory-picker").assertDoesNotExist()
                compose.onNodeWithText("Working directory on", substring = true)
                    .performScrollTo()
                    .assertIsDisplayed()
                compose.onNodeWithTag("working-directory-path-scroll")
                    .assertContentDescriptionEquals(exact)
                    .assertScrolledToTail()
                compose.onNodeWithTag("forge-name").performScrollTo()
                    .assertEditableTextEquals("kept-name")
                compose.onNodeWithTag("forge-objective").performScrollTo()
                    .assertEditableTextEquals("kept objective")
                compose.onNodeWithTag("forge-submit").performScrollTo().assertIsEnabled()
                assertEquals("Use must not invoke Create", 0, boundary.createRequests.get())

                compose.onNodeWithTag("forge-working-directory-change")
                    .performScrollTo()
                    .performClick()
                compose.onNodeWithText("Enter exact path").performScrollTo().performClick()
                compose.onNodeWithTag("working-directory-exact-field")
                    .assertTextContains(exact)
                compose.onNodeWithTag("working-directory-cancel")
                    .assertMinimumTarget()
                    .performClick()
                compose.onNodeWithTag("working-directory-picker").assertDoesNotExist()
                compose.onNodeWithTag("working-directory-path-scroll")
                    .assertContentDescriptionEquals(exact)
                compose.onNodeWithTag("forge-sheet").assertIsDisplayed()
                assertEquals("the chooser journey must never Create", 0, boundary.createRequests.get())

                compose.onNodeWithTag("forge-working-directory-change")
                    .performScrollTo()
                    .performClick()
                compose.onNodeWithTag("working-directory-picker").assertIsDisplayed()
                compose.onNodeWithContentDescription("Close sheet")
                    .performSemanticsAction(SemanticsActions.OnClick)
                compose.mainClock.advanceTimeBy(1_000)
                compose.waitForIdle()
                compose.waitUntil(timeoutMillis = 5_000) {
                    compose.onAllNodesWithTag("working-directory-picker")
                        .fetchSemanticsNodes().isEmpty()
                }
                compose.onNodeWithTag("forge-sheet").assertDoesNotExist()
                compose.onNodeWithContentDescription("New dwarf").assertIsDisplayed()
            }
        } finally {
            InstrumentationRegistry.getInstrumentation().runOnMainSync(controller::close)
            compose.mainClock.autoAdvance = true
        }

        boundary.assertSatisfied()
        assertEquals("the production journey must never POST Create", 0, boundary.createRequests.get())
    }

    private fun waitForEnabledDescription(description: String) {
        compose.waitUntil(timeoutMillis = 10_000) {
            compose.onAllNodes(hasContentDescription(description)).fetchSemanticsNodes()
                .singleOrNull()
                ?.let { node -> node.config.getOrNull(SemanticsProperties.Disabled) == null }
                ?: false
        }
        compose.onNodeWithContentDescription(description).assertIsEnabled()
    }

    private fun waitForDescription(description: String) {
        compose.waitUntil(timeoutMillis = 5_000) {
            compose.onAllNodes(hasContentDescription(description), useUnmergedTree = true)
                .fetchSemanticsNodes().isNotEmpty()
        }
    }

    private fun waitForEnabledTag(tag: String) {
        compose.waitUntil(timeoutMillis = 10_000) {
            compose.onAllNodesWithTag(tag).fetchSemanticsNodes()
                .singleOrNull()
                ?.let { node -> node.config.getOrNull(SemanticsProperties.Disabled) == null }
                ?: false
        }
        compose.onNodeWithTag(tag).assertIsEnabled()
    }

    private fun waitForText(text: String) {
        compose.waitUntil(timeoutMillis = 5_000) {
            compose.onAllNodesWithText(text).fetchSemanticsNodes().isNotEmpty()
        }
    }

    private fun SemanticsNodeInteraction.assertMinimumTarget(): SemanticsNodeInteraction {
        val bounds = getUnclippedBoundsInRoot()
        val minimumPixels = with(compose.density) { 48.dp.roundToPx() }
        val widthPixels = with(compose.density) { (bounds.right - bounds.left).roundToPx() }
        val heightPixels = with(compose.density) { (bounds.bottom - bounds.top).roundToPx() }
        assertTrue(
            "interactive target is smaller than 48dp: $bounds",
            widthPixels >= minimumPixels && heightPixels >= minimumPixels,
        )
        return this
    }

    private fun SemanticsNodeInteraction.assertPoliteLiveRegion(): SemanticsNodeInteraction {
        assertEquals(
            "async and validation announcements must use one polite live region",
            LiveRegionMode.Polite,
            fetchSemanticsNode().config.getOrNull(SemanticsProperties.LiveRegion),
        )
        compose.onAllNodesWithTag("working-directory-live-region").assertCountEquals(1)
        return this
    }

    private fun SemanticsNodeInteraction.assertEditableTextEquals(
        expected: String,
    ): SemanticsNodeInteraction {
        assertEquals(
            "editable value must be preserved exactly",
            expected,
            fetchSemanticsNode().config.getOrNull(SemanticsProperties.EditableText)?.text,
        )
        return this
    }

    private fun SemanticsNodeInteraction.assertScrolledToTail(): SemanticsNodeInteraction {
        compose.waitUntil(timeoutMillis = 5_000) {
            val range = fetchSemanticsNode().config.getOrNull(
                SemanticsProperties.HorizontalScrollAxisRange,
            ) ?: return@waitUntil false
            range.maxValue() > 0f && range.value() >= range.maxValue() - 1f
        }
        return this
    }

    private fun resetFixture() {
        val preferences = storage.preferences(context)
        check(preferences.edit().clear().commit()) { "could not clear picker preferences" }
        check(preferences.all.isEmpty()) { "picker preferences survived cleanup" }
        val keyStore = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }
        if (keyStore.containsAlias(TEST_KEY_ALIAS)) keyStore.deleteEntry(TEST_KEY_ALIAS)
        check(!keyStore.containsAlias(TEST_KEY_ALIAS)) { "picker Keystore alias survived cleanup" }
    }

    private class ExternalMachineBoundary(
        credentials: List<MachineCredential>,
    ) : Interceptor {
        val createRequests = AtomicInteger()

        private val credentialsByHost = credentials.associateBy { credential ->
            checkNotNull(credential.machine.origin.encoded.toHttpUrlOrNull()).host
        }
        private val listingReplies = LinkedBlockingQueue<ListingReply>()
        private val listingRequests = AtomicInteger()
        private val violation = AtomicReference<String?>(null)

        override fun intercept(chain: Interceptor.Chain): Response {
            val request = chain.request()
            val credential = credentialsByHost[request.url.host]
            if (credential == null) recordViolation("unknown machine origin")
            credential?.let { validateAuthority(request, it) }

            return when (request.method to request.url.encodedPath) {
                "GET" to "/v1/sessions" -> jsonResponse(
                    request,
                    200,
                    sessionsResponse(checkNotNull(credential)),
                )
                "GET" to "/v1/pressure" -> response(request, 503, ByteArray(0).toResponseBody())
                "POST" to "/v1/directory-listings" -> {
                    listingRequests.incrementAndGet()
                    val reply = try {
                        listingReplies.poll(10, TimeUnit.SECONDS)
                    } catch (_: InterruptedException) {
                        Thread.currentThread().interrupt()
                        recordViolation("directory response wait was interrupted")
                        return response(request, 503, ByteArray(0).toResponseBody())
                    }
                    if (reply == null) {
                        recordViolation("directory response was not supplied")
                        return response(request, 503, ByteArray(0).toResponseBody())
                    }
                    val encodedBody = request.body?.let { body ->
                        Buffer().apply { body.writeTo(this) }.readUtf8()
                    }
                    if (encodedBody != "{\"directory\":\"${reply.expectedDirectory}\"}") {
                        recordViolation("directory request body disagreed")
                    }
                    if (request.body?.contentType()?.toString() != "application/json; charset=utf-8") {
                        recordViolation("directory request content type disagreed")
                    }
                    if (credential != DEVBOX) recordViolation("directory request crossed machines")
                    if (reply.status == 200) {
                        jsonResponse(request, reply.status, reply.body)
                    } else {
                        response(request, reply.status, ByteArray(0).toResponseBody())
                    }
                }
                "POST" to "/v1/sessions" -> {
                    createRequests.incrementAndGet()
                    jsonResponse(
                        request,
                        500,
                        "{\"code\":\"InternalError\",\"message\":" +
                            "\"Skíðblaðnir could not complete the request.\"}",
                    )
                }
                else -> {
                    recordViolation("unexpected external route")
                    jsonResponse(
                        request,
                        500,
                        "{\"code\":\"InternalError\",\"message\":" +
                            "\"Skíðblaðnir could not complete the request.\"}",
                    )
                }
            }
        }

        fun respond(reply: ListingReply) {
            check(listingReplies.offer(reply, 5, TimeUnit.SECONDS)) {
                "external directory response boundary did not accept its fixture"
            }
        }

        fun assertSatisfied() {
            assertEquals("external machine boundary violation", null, violation.get())
            assertEquals(
                "every supplied directory response must have one real controller request",
                0,
                listingReplies.size,
            )
            assertTrue("the journey never reached the directory transport", listingRequests.get() > 0)
        }

        private fun validateAuthority(request: Request, credential: MachineCredential) {
            if (
                request.headers.values("Authorization") !=
                listOf("Bearer ${credential.bearer.encoded}")
            ) {
                recordViolation("request authentication disagreed")
            }
            if (
                request.headers.values("Skidbladnir-Machine") !=
                listOf(credential.machine.handle.encoded)
            ) {
                recordViolation("request machine binding disagreed")
            }
            if (request.url.scheme != "https" || request.url.port != 8443) {
                recordViolation("request origin shape disagreed")
            }
        }

        private fun recordViolation(message: String) {
            violation.compareAndSet(null, message)
        }

        private fun sessionsResponse(credential: MachineCredential): String {
            val platform = if (credential.machine.label.text == "MacBook") "Darwin" else "Linux"
            val sessions = if (credential == DEVBOX) {
                "[{\"tmuxId\":\"${'$'}1\",\"tmuxName\":\"active-session\"," +
                    "\"identityToken\":\"synthetic-lifetime\"," +
                    "\"character\":{\"key\":\"alvis\",\"displayName\":\"Alvís\"}," +
                    "\"launchProfile\":\"personal\",\"cwd\":\"$ACTIVE_DIRECTORY\"," +
                    "\"attachedClients\":0,\"activity\":\"Quiet\"}]"
            } else {
                "[]"
            }
            return "{\"machine\":{\"handle\":\"${credential.machine.handle.encoded}\"," +
                "\"platform\":\"$platform\"},\"observedAt\":\"2026-08-31T12:00:00Z\"," +
                "\"profiles\":[{\"key\":\"personal\",\"label\":\"Codex · Personal\"," +
                "\"provider\":\"Codex\"}],\"sessions\":$sessions}"
        }

        private fun jsonResponse(request: Request, status: Int, body: String): Response = response(
            request,
            status,
            body.toResponseBody(JSON_MEDIA_TYPE),
        )

        private fun response(
            request: Request,
            status: Int,
            body: okhttp3.ResponseBody,
        ): Response = Response.Builder()
            .request(request)
            .protocol(Protocol.HTTP_1_1)
            .code(status)
            .message(if (status == 200) "OK" else "Unavailable")
            .body(body)
            .build()
    }

    private data class ListingReply(
        val expectedDirectory: String,
        val status: Int,
        val body: String,
    ) {
        companion object {
            fun transport(expectedDirectory: String) = ListingReply(expectedDirectory, 503, "")
        }
    }

    private companion object {
        const val TEST_PREFERENCES = "skidbladnir.machines.working-directory-picker-test"
        const val TEST_KEY_ALIAS = "skidbladnir.machine-bearers.working-directory-picker-test"
        val JSON_MEDIA_TYPE = "application/json; charset=utf-8".toMediaType()
        val ACTIVE_DIRECTORY = "/work/" + "long-active-directory/".repeat(10) + "current"
        val DEEP_DIRECTORIES = listOf(
            "~/Alpha",
            "~/Alpha/nested-20",
            "~/Alpha/nested-20/level-3",
            "~/Alpha/nested-20/level-3/level-4",
            "~/Alpha/nested-20/level-3/level-4/level-5",
            "~/Alpha/nested-20/level-3/level-4/level-5/level-6",
        )
        val ARCH = credential(
            "mh-11111111111111111111111111111111",
            "Arch",
            "https://arch.picker.invalid:8443/",
            "A".repeat(43),
        )
        val DEVBOX = credential(
            "mh-22222222222222222222222222222222",
            "Devbox",
            "https://devbox.picker.invalid:8443/",
            "B" + "A".repeat(42),
        )
        val MACBOOK = credential(
            "mh-33333333333333333333333333333333",
            "MacBook",
            "https://macbook.picker.invalid:8443/",
            "C" + "A".repeat(42),
        )
        val FLEET = listOf(ARCH, DEVBOX, MACBOOK)

        fun homeListing(): ListingReply = listing(
            directory = "~",
            parent = null,
            children = buildList {
                add("~/.hidden" to "Directory")
                add("~/Alpha" to "Directory")
                add("~/current" to "SymbolicLink")
                repeat(24) { index ->
                    add("~/folder-${index.toString().padStart(2, '0')}" to "Directory")
                }
            },
            omitted = true,
        )

        fun alphaListing(): ListingReply = listing(
            directory = "~/Alpha",
            parent = "~",
            children = List(32) { index ->
                "~/Alpha/nested-${index.toString().padStart(2, '0')}" to "Directory"
            },
        )

        fun deepListing(depth: Int): ListingReply = listing(
            directory = DEEP_DIRECTORIES[depth - 1],
            parent = DEEP_DIRECTORIES.getOrElse(depth - 2) { "~" },
            children = if (depth == DEEP_DIRECTORIES.size) {
                emptyList()
            } else {
                listOf(DEEP_DIRECTORIES[depth] to "Directory")
            },
        )

        fun listing(
            directory: String,
            parent: String?,
            children: List<Pair<String, String>>,
            omitted: Boolean = false,
        ): ListingReply {
            val parentField = parent?.let { "\"parentDirectory\":\"$it\"," }.orEmpty()
            val childrenField = children.joinToString(",") { (child, kind) ->
                "{\"directory\":\"$child\",\"kind\":\"$kind\"}"
            }
            return ListingReply(
                expectedDirectory = directory,
                status = 200,
                body = "{\"machine\":{\"handle\":\"${DEVBOX.machine.handle.encoded}\"," +
                    "\"platform\":\"Linux\"},\"directory\":\"$directory\"," +
                    parentField + "\"children\":[$childrenField],\"omitted\":$omitted}",
            )
        }

        fun credential(
            handle: String,
            label: String,
            origin: String,
            bearer: String,
        ): MachineCredential = MachineCredential(
            PairedMachine(
                requireNotNull(MachineHandle.parse(handle)),
                requireNotNull(MachineLabel.parse(label)),
                requireNotNull(MachineOrigin.parse(origin)),
            ),
            requireNotNull(GatewayBearer.parse(bearer)),
        )
    }
}
