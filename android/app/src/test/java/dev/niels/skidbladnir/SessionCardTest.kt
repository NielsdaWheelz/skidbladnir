package dev.niels.skidbladnir

import org.junit.Assert.assertTrue
import org.junit.Test

class SessionCardTest {
    @Test
    fun `directory context preserves short paths and roots but abbreviates a long path to its tail`() {
        val cases = mapOf(
            "/" to "/",
            "/src/skidbladnir" to "/src/skidbladnir",
            "src/skidbladnir" to "src/skidbladnir",
            "/srv/workspaces/skidbladnir/android" to "…/skidbladnir/android",
            "workspace/src/skidbladnir" to "…/src/skidbladnir",
            "~/src/skidbladnir" to "…/src/skidbladnir",
        )

        cases.entries.forEachIndexed { index, (source, expected) ->
            assertTrue(
                "directory abbreviation contract case $index failed",
                abbreviatedDirectory(source) == expected,
            )
        }
    }
}
