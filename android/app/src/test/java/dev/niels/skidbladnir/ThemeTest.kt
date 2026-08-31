package dev.niels.skidbladnir

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Test

class ThemeTest {
    @Test
    fun `session activity copy is exact and retained values are qualified`() {
        assertEquals(
            SessionActivityContent("ACTIVE", "Recent tmux activity at the last check"),
            sessionActivityContent(SessionActivity.Active, fresh = true),
        )
        assertEquals(
            SessionActivityContent("QUIET", "No recent tmux activity at the last check"),
            sessionActivityContent(SessionActivity.Quiet, fresh = true),
        )
        assertEquals(
            SessionActivityContent("ACTIVE", "Last observed: recent tmux activity"),
            sessionActivityContent(SessionActivity.Active, fresh = false),
        )
        assertEquals(
            SessionActivityContent("QUIET", "Last observed: no recent tmux activity"),
            sessionActivityContent(SessionActivity.Quiet, fresh = false),
        )
    }

    @Test
    fun `session activity tones map to the normative design tokens`() {
        assertEquals(
            mapOf(
                SessionActivity.Active to Moss,
                SessionActivity.Quiet to Muted,
            ),
            SessionActivity.entries.associateWith(::sessionActivityColor),
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
