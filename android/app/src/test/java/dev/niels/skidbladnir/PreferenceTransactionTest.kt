package dev.niels.skidbladnir

import android.content.SharedPreferences
import java.util.ArrayDeque
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Test

class PreferenceTransactionTest {
    @Test
    fun `commit false after process memory changes restores the exact empty pre-state`() {
        val preferences = FailingPreferences(
            initial = emptyMap(),
            commitOutcomes = listOf(false, true),
        )

        assertFalse(
            replacePreferencesWithVerifiedRollback(
                preferences,
                mapOf("machine.handles" to setOf("mh-target")),
                verifyTarget = { true },
                onUnconfirmedRollback = { error("confirmed rollback destroyed the fleet key") },
            ),
        )

        assertEquals(emptyMap<String, Any>(), preferences.snapshot())
        assertEquals(2, preferences.commitCount)
    }

    @Test
    fun `failed readback restores the exact reconnect pre-state`() {
        val prior = mapOf<String, Any>(
            "machine.handles" to setOf("mh-prior"),
            "machine.mh-prior.ciphertext" to "sealed-prior",
        )
        val preferences = FailingPreferences(
            initial = prior,
            commitOutcomes = listOf(true, true),
        )

        assertFalse(
            replacePreferencesWithVerifiedRollback(
                preferences,
                mapOf(
                    "machine.handles" to setOf("mh-prior"),
                    "machine.mh-prior.ciphertext" to "sealed-replacement",
                ),
                verifyTarget = { false },
                onUnconfirmedRollback = { error("confirmed rollback destroyed the fleet key") },
            ),
        )

        assertEquals(prior, preferences.snapshot())
        assertEquals(2, preferences.commitCount)
    }

    @Test
    fun `unconfirmed rollback leaves a whole collection quarantine instead of readable target state`() {
        val preferences = FailingPreferences(
            initial = mapOf("machine.handles" to setOf("mh-prior")),
            commitOutcomes = listOf(false, false, false),
        )

        assertFalse(
            replacePreferencesWithVerifiedRollback(
                preferences,
                mapOf("machine.handles" to setOf("mh-target")),
                verifyTarget = { true },
                onUnconfirmedRollback = { error("commit-false target destroyed the fleet key") },
            ),
        )

        assertEquals(mapOf(FLEET_QUARANTINE_FIELD to true), preferences.snapshot())
        assertEquals(3, preferences.commitCount)
    }

    @Test
    fun `disk-confirmed target with unconfirmed rollback destroys authority before process quarantine`() {
        val prior = mapOf<String, Any>("machine.handles" to setOf("mh-prior"))
        val preferences = FailingPreferences(
            initial = prior,
            commitOutcomes = listOf(true, false, false),
        )
        val events = mutableListOf<String>()

        assertFalse(
            replacePreferencesWithVerifiedRollback(
                preferences,
                mapOf("machine.handles" to setOf("mh-target")),
                verifyTarget = { false },
                onUnconfirmedRollback = {
                    assertEquals(prior, preferences.snapshot())
                    events += "destroy-key"
                },
            ),
        )

        assertEquals(listOf("destroy-key"), events)
        assertEquals(mapOf(FLEET_QUARANTINE_FIELD to true), preferences.snapshot())
        assertEquals(3, preferences.commitCount)
    }
}

private class FailingPreferences(
    initial: Map<String, Any>,
    commitOutcomes: List<Boolean>,
) : SharedPreferences {
    private val values = initial.toMutableMap()
    private val outcomes = ArrayDeque(commitOutcomes)
    var commitCount: Int = 0
        private set

    fun snapshot(): Map<String, Any> = values.mapValues { (_, value) ->
        if (value is Set<*>) value.toSet() else value
    }

    override fun getAll(): MutableMap<String, *> = snapshot().toMutableMap()
    override fun getString(key: String?, default: String?): String? = values[key] as? String ?: default
    @Suppress("UNCHECKED_CAST")
    override fun getStringSet(key: String?, default: MutableSet<String>?): MutableSet<String>? =
        (values[key] as? Set<String>)?.toMutableSet() ?: default
    override fun getInt(key: String?, default: Int): Int = values[key] as? Int ?: default
    override fun getLong(key: String?, default: Long): Long = values[key] as? Long ?: default
    override fun getFloat(key: String?, default: Float): Float = values[key] as? Float ?: default
    override fun getBoolean(key: String?, default: Boolean): Boolean = values[key] as? Boolean ?: default
    override fun contains(key: String?): Boolean = values.containsKey(key)
    override fun edit(): SharedPreferences.Editor = Editor()
    override fun registerOnSharedPreferenceChangeListener(
        listener: SharedPreferences.OnSharedPreferenceChangeListener?,
    ) = Unit
    override fun unregisterOnSharedPreferenceChangeListener(
        listener: SharedPreferences.OnSharedPreferenceChangeListener?,
    ) = Unit

    private inner class Editor : SharedPreferences.Editor {
        private val writes = mutableMapOf<String, Any>()
        private val removals = mutableSetOf<String>()
        private var clear = false

        override fun putString(key: String?, value: String?): SharedPreferences.Editor = apply {
            stage(key, value)
        }
        override fun putStringSet(key: String?, value: MutableSet<String>?): SharedPreferences.Editor = apply {
            stage(key, value?.toSet())
        }
        override fun putInt(key: String?, value: Int): SharedPreferences.Editor = apply { stage(key, value) }
        override fun putLong(key: String?, value: Long): SharedPreferences.Editor = apply { stage(key, value) }
        override fun putFloat(key: String?, value: Float): SharedPreferences.Editor = apply { stage(key, value) }
        override fun putBoolean(key: String?, value: Boolean): SharedPreferences.Editor = apply { stage(key, value) }
        override fun remove(key: String?): SharedPreferences.Editor = apply {
            requireNotNull(key)
            removals += key
            writes -= key
        }
        override fun clear(): SharedPreferences.Editor = apply { clear = true }
        override fun apply() = commit().let { Unit }
        override fun commit(): Boolean {
            if (clear) values.clear()
            removals.forEach(values::remove)
            values.putAll(writes)
            commitCount += 1
            return if (outcomes.isEmpty()) true else outcomes.removeFirst()
        }

        private fun stage(key: String?, value: Any?) {
            requireNotNull(key)
            if (value == null) {
                removals += key
                writes -= key
            } else {
                writes[key] = value
                removals -= key
            }
        }
    }
}
