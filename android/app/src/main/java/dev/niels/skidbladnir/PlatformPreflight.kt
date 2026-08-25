package dev.niels.skidbladnir

import android.content.Context
import android.content.pm.PackageManager
import android.os.Build
import android.view.inputmethod.InputMethodManager
import android.webkit.WebView
import kotlinx.serialization.Serializable
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json

private const val PREFLIGHT_SCHEMA = "android-target-preflight.v1"
private const val TARGET_MODEL = "SM-S906W"
private const val TARGET_API = 36
private const val GBOARD_PACKAGE = "com.google.android.inputmethod.latin"
private const val TAILSCALE_PACKAGE = "com.tailscale.ipn"
private const val NO_DEVICE_REASON = "SM-S906W is not attached"

private val preflightJson = Json {
    encodeDefaults = true
    explicitNulls = false
    prettyPrint = true
}

@Serializable
internal enum class PreflightStatus {
    PASS,
    FAIL,
    NOT_RUN,
}

@Serializable
internal data class PreflightTarget(
    val model: String,
    val api: Int,
)

/** Credential-free device facts needed to interpret the interactive checks. */
@Serializable
internal data class PreflightObserved(
    val model: String? = null,
    val api: Int? = null,
    val buildId: String? = null,
    val webViewPackage: String? = null,
    val webViewVersion: String? = null,
    val imePackage: String? = null,
    val tailscaleInstalled: Boolean? = null,
)

@Serializable
internal data class PreflightCheck(
    val id: String,
    val status: PreflightStatus,
    val reason: String,
)

/**
 * Host-readable result for the target-device gate. It deliberately has no
 * serial, account, bearer, prompt, terminal, or user-content fields.
 */
@Serializable
internal data class PlatformPreflightReport(
    val schema: String,
    val target: PreflightTarget,
    val observed: PreflightObserved,
    val overall: PreflightStatus,
    val checks: List<PreflightCheck>,
) {
    fun toJson(): String = preflightJson.encodeToString(this)

    companion object {
        fun noDevice(): PlatformPreflightReport {
            val checks = requiredCheckIds.map { id ->
                PreflightCheck(id, PreflightStatus.NOT_RUN, NO_DEVICE_REASON)
            }
            return report(PreflightObserved(), checks)
        }

        fun collect(context: Context): PlatformPreflightReport {
            val webView = WebView.getCurrentWebViewPackage()
            val imePackage = context.getSystemService(InputMethodManager::class.java)
                ?.currentInputMethodInfo
                ?.packageName
            val tailscaleInstalled = try {
                context.packageManager.getApplicationInfo(TAILSCALE_PACKAGE, 0)
                true
            } catch (_: PackageManager.NameNotFoundException) {
                // justify-ignore-error: package absence is the expected negative preflight result.
                false
            }
            val observed = PreflightObserved(
                model = Build.MODEL,
                api = Build.VERSION.SDK_INT,
                buildId = Build.ID,
                webViewPackage = webView?.packageName,
                webViewVersion = webView?.versionName,
                imePackage = imePackage,
                tailscaleInstalled = tailscaleInstalled,
            )
            val checks = buildList {
                add(
                    check(
                        id = "target-device",
                        passes = observed.model == TARGET_MODEL,
                        reason = "model=${observed.model}",
                    ),
                )
                add(
                    check(
                        id = "api-36",
                        passes = observed.api == TARGET_API,
                        reason = "api=${observed.api}",
                    ),
                )
                add(
                    check(
                        id = "webview-runtime",
                        passes = observed.webViewPackage != null,
                        reason = "package=${observed.webViewPackage ?: "missing"}",
                    ),
                )
                add(
                    check(
                        id = "gboard-selected",
                        passes = observed.imePackage == GBOARD_PACKAGE,
                        reason = "package=${observed.imePackage ?: "missing"}",
                    ),
                )
                add(
                    check(
                        id = "tailscale-client-present",
                        passes = observed.tailscaleInstalled == true,
                        reason = "installed=${observed.tailscaleInstalled}",
                    ),
                )
                addAll(
                    interactiveCheckIds.map { id ->
                        PreflightCheck(id, PreflightStatus.NOT_RUN, "requires interactive SM-S906W run")
                    },
                )
            }
            return report(observed, checks)
        }

        private fun report(
            observed: PreflightObserved,
            checks: List<PreflightCheck>,
        ): PlatformPreflightReport = PlatformPreflightReport(
            schema = PREFLIGHT_SCHEMA,
            target = PreflightTarget(TARGET_MODEL, TARGET_API),
            observed = observed,
            overall = when {
                checks.any { it.status == PreflightStatus.FAIL } -> PreflightStatus.FAIL
                checks.any { it.status == PreflightStatus.NOT_RUN } -> PreflightStatus.NOT_RUN
                else -> PreflightStatus.PASS
            },
            checks = checks,
        )

        private fun check(id: String, passes: Boolean, reason: String): PreflightCheck =
            PreflightCheck(
                id = id,
                status = if (passes) PreflightStatus.PASS else PreflightStatus.FAIL,
                reason = reason,
            )
    }
}

private val interactiveCheckIds = listOf(
    "ansi-utf8",
    "ime-composition",
    "editable-dictation",
    "clipboard-paste",
    "ime-resize",
    "navigation-gesture",
    "navigation-buttons",
    "scale-200",
    "talkback",
    "switch-access",
    "rotation",
    "process-recreation",
)

private val requiredCheckIds = listOf(
    "target-device",
    "api-36",
    "webview-runtime",
    "gboard-selected",
    "tailscale-client-present",
) + interactiveCheckIds
