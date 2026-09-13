package dev.niels.skidbladnir

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Test

class ThemeTest {
    @Test
    fun `status distinguishes inferred and retained observations`() {
        val native = AgentStatus(AgentState.Blocked, AgentMethod.Native)
        val inferred = AgentStatus(AgentState.Working, AgentMethod.Terminal)
        assertEquals(SessionStatusContent("BLOCKED", "blocked"), sessionStatusContent(native, true))
        assertEquals(SessionStatusContent("WORKING · inferred", "Last observed: working · inferred"), sessionStatusContent(inferred, false))
        assertEquals(SessionStatusContent("TERMINAL", "terminal"), sessionStatusContent(null, true))
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
