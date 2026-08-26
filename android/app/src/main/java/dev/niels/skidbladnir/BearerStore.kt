package dev.niels.skidbladnir

import android.annotation.SuppressLint
import android.content.Context
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.util.Base64
import java.io.IOException
import java.nio.charset.StandardCharsets
import java.security.KeyStore
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

private const val KEY_ALIAS = "skidbladnir.gateway.bearer"
private const val PREFERENCES_NAME = "skidbladnir.pairing"
private const val CIPHERTEXT_KEY = "bearer.ciphertext"
private const val NONCE_KEY = "bearer.nonce"
private const val CIPHER_TRANSFORMATION = "AES/GCM/NoPadding"
private val associatedData = "dev.niels.skidbladnir.gateway.bearer.v1".toByteArray(StandardCharsets.UTF_8)

@SuppressLint("UseKtx") // Direct commit results are required; the KTX helper discards them.
internal class BearerStore(context: Context) {
    private val preferences = context.getSharedPreferences(PREFERENCES_NAME, Context.MODE_PRIVATE)

    fun read(): String? {
        val encodedCiphertext = preferences.getString(CIPHERTEXT_KEY, null)
        val encodedNonce = preferences.getString(NONCE_KEY, null)
        if (encodedCiphertext == null && encodedNonce == null) return null
        if (encodedCiphertext == null || encodedNonce == null) throw IOException("incomplete encrypted bearer")
        val cipher = Cipher.getInstance(CIPHER_TRANSFORMATION)
        cipher.init(
            Cipher.DECRYPT_MODE,
            requireKey(),
            GCMParameterSpec(128, Base64.decode(encodedNonce, Base64.NO_WRAP)),
        )
        cipher.updateAAD(associatedData)
        return String(
            cipher.doFinal(Base64.decode(encodedCiphertext, Base64.NO_WRAP)),
            StandardCharsets.UTF_8,
        )
    }

    fun write(bearer: String) {
        val cipher = Cipher.getInstance(CIPHER_TRANSFORMATION)
        cipher.init(Cipher.ENCRYPT_MODE, requireKey())
        cipher.updateAAD(associatedData)
        val ciphertext = cipher.doFinal(bearer.toByteArray(StandardCharsets.UTF_8))
        if (!preferences.edit()
                .putString(CIPHERTEXT_KEY, Base64.encodeToString(ciphertext, Base64.NO_WRAP))
                .putString(NONCE_KEY, Base64.encodeToString(cipher.iv, Base64.NO_WRAP))
                .commit()
        ) {
            throw IOException("could not persist encrypted bearer")
        }
    }

    fun clear() {
        if (!preferences.contains(CIPHERTEXT_KEY) && !preferences.contains(NONCE_KEY)) return
        if (!preferences.edit()
                .remove(CIPHERTEXT_KEY)
                .remove(NONCE_KEY)
                .commit()
        ) {
            throw IOException("could not clear encrypted bearer")
        }
    }

    fun reset() {
        clear()
        val keyStore = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }
        if (keyStore.containsAlias(KEY_ALIAS)) keyStore.deleteEntry(KEY_ALIAS)
    }

    private fun requireKey(): SecretKey {
        val keyStore = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }
        (keyStore.getKey(KEY_ALIAS, null) as? SecretKey)?.let { return it }
        return KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, "AndroidKeyStore").run {
            init(
                KeyGenParameterSpec.Builder(
                    KEY_ALIAS,
                    KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT,
                )
                    .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                    .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                    .setKeySize(256)
                    .build(),
            )
            generateKey()
        }
    }
}
