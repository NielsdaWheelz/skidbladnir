package dev.niels.skidbladnir

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class ThemeTest {
    @Test
    fun `statusColor is injective across all session status kinds`() {
        val mapping = SessionStatusKind.entries.associateWith(::statusColor)
        val distinctColors = mapping.values.toSet()

        assertEquals(
            "statusColor must be injective so every status kind is visually distinct; " +
                "colliding mapping was $mapping",
            mapping.size,
            distinctColors.size,
        )
    }

    @Test
    fun `attentionPulseEnabled is false only when the animator duration scale is zero`() {
        assertFalse(
            "scale 0f means the system disabled animations; the pulse must render static",
            attentionPulseEnabled(0f),
        )
        assertTrue(
            "scale 1f is the normal animator rate; the pulse must be enabled",
            attentionPulseEnabled(1f),
        )
        assertTrue(
            "scale 0.5f is reduced but nonzero; the pulse must still be enabled",
            attentionPulseEnabled(0.5f),
        )
    }

    @Test
    fun `noticeToneColor is injective and paints degradation as absence rather than failure`() {
        val mapping = NoticeTone.entries.associateWith(::noticeToneColor)
        val distinctColors = mapping.values.toSet()

        assertEquals(
            "noticeToneColor must be injective so failure, degradation, and armed recovery are " +
                "visually distinct; colliding mapping was $mapping",
            mapping.size,
            distinctColors.size,
        )
        assertNotEquals(
            "staleness is absence, not failure: Degraded must never be Ember, or a routinely stale " +
                "host makes the alarm color the dashboard's resting state; mapping was $mapping",
            Ember,
            noticeToneColor(NoticeTone.Degraded),
        )
    }
}
