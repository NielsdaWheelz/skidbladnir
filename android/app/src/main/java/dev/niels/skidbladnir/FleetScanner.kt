package dev.niels.skidbladnir

import android.app.Activity
import android.content.Context
import android.content.Intent
import android.net.Uri
import com.google.mlkit.vision.barcode.common.Barcode
import com.google.mlkit.vision.codescanner.GmsBarcodeScannerOptions
import com.google.mlkit.vision.codescanner.GmsBarcodeScanning

private const val TAILSCALE_PACKAGE = "com.tailscale.ipn"
private val TAILSCALE_INSTALL_URI = Uri.parse(
    "https://play.google.com/store/apps/details?id=$TAILSCALE_PACKAGE",
)

internal class FleetScanner(activity: Activity) {
    private val scanner = GmsBarcodeScanning.getClient(
        activity,
        GmsBarcodeScannerOptions.Builder()
            .setBarcodeFormats(Barcode.FORMAT_QR_CODE)
            .build(),
    )

    fun scan(
        onResult: (String) -> Unit,
        onCancelled: () -> Unit,
        onFailure: () -> Unit,
    ) {
        scanner.startScan()
            .addOnSuccessListener { barcode ->
                val encoded = barcode.rawValue
                if (encoded == null) onFailure() else onResult(encoded)
            }
            .addOnCanceledListener(onCancelled)
            .addOnFailureListener {
                // justify-ignore-error: Google Code Scanner is an external boundary whose raw
                // failure is intentionally collapsed into the frozen credential-free scan state.
                onFailure()
            }
    }
}

internal fun tailscaleInstalled(context: Context): Boolean =
    context.packageManager.getLaunchIntentForPackage(TAILSCALE_PACKAGE) != null

internal fun openOrInstallTailscale(context: Context) {
    val intent = context.packageManager.getLaunchIntentForPackage(TAILSCALE_PACKAGE)
        ?: Intent(Intent.ACTION_VIEW, TAILSCALE_INSTALL_URI)
    context.startActivity(intent)
}
