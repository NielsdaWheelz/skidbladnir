package dev.niels.skidbladnir

import java.time.Instant
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class PressurePresentationTest {
    @Test
    fun `rail header follows freshness then aggregate precedence with literal causes`() {
        val hotRecovery = response(
            level = PressureLevel.Hot,
            phase = PressurePhase.Recovering,
            reasons = listOf(PressureReason.Disk, PressureReason.Load),
            signals = linuxSignals(),
        )
        val unknown = response(
            level = PressureLevel.Unknown,
            signals = linuxSignals(memory = PressureSignal.Missing(PressureMetric.MemoryAvailablePercent)),
        )
        val warmRecovery = response(
            level = PressureLevel.Warm,
            phase = PressurePhase.Recovering,
            reasons = listOf(PressureReason.Load),
            signals = linuxSignals(
                load = measured(PressureMetric.NormalizedLoad, 1.2, PressureSignalState.Warm),
            ),
        )

        val cases = listOf(
            PressureState.Reading to
                PressureRailHeaderContent("Devbox", "READING", PressureRailAccent.Gold),
            PressureState.Unavailable(GatewayFailure.Transport) to
                PressureRailHeaderContent("Devbox", "PRESSURE UNAVAILABLE", PressureRailAccent.Muted),
            PressureState.Stale(hotRecovery, GatewayFailure.Transport) to
                PressureRailHeaderContent("Devbox", "PRESSURE STALE · LAST HOT", PressureRailAccent.Muted),
            PressureState.Fresh(unknown) to
                PressureRailHeaderContent("Devbox", "UNKNOWN · MEM NO DATA", PressureRailAccent.Muted),
            PressureState.Fresh(warmRecovery) to PressureRailHeaderContent(
                "Devbox",
                "RECOVERING FROM WARM · LOAD",
                PressureRailAccent.Gold,
            ),
            PressureState.Fresh(hotRecovery) to PressureRailHeaderContent(
                "Devbox",
                "RECOVERING FROM HOT · DISK +1",
                PressureRailAccent.Ember,
            ),
            PressureState.Fresh(response(PressureLevel.Normal, signals = linuxSignals())) to
                PressureRailHeaderContent("Devbox", "NORMAL", PressureRailAccent.Muted),
            PressureState.Fresh(
                response(
                    PressureLevel.Warm,
                    reasons = listOf(PressureReason.Memory),
                    signals = linuxSignals(
                        memory = measured(
                            PressureMetric.MemoryAvailablePercent,
                            12.0,
                            PressureSignalState.Warm,
                        ),
                    ),
                ),
            ) to PressureRailHeaderContent("Devbox", "WARM · MEMORY", PressureRailAccent.Gold),
            PressureState.Fresh(
                response(
                    PressureLevel.Hot,
                    reasons = listOf(PressureReason.IoPsi),
                    signals = linuxSignals(
                        ioPsi = measured(
                            PressureMetric.IoPsiFullAvg60Percent,
                            6.0,
                            PressureSignalState.Hot,
                        ),
                    ),
                ),
            ) to PressureRailHeaderContent("Devbox", "HOT · I/O PRESSURE", PressureRailAccent.Ember),
        )

        cases.forEach { (state, expected) ->
            assertEquals(
                "pressure state $state must produce the reviewed verdict-first header",
                expected,
                pressureRailContent("Devbox", state).header,
            )
        }

        val missingMemory = pressureRailContent("Devbox", PressureState.Fresh(unknown)).metrics[1]
        assertEquals(
            PressureRailMetricContent(
                PressureMetric.MemoryAvailablePercent,
                "MEM",
                "NO DATA",
                "?",
                "No data",
                PressureRailAccent.Muted,
            ),
            missingMemory,
        )
        val missingMemoryDetail = pressureDetailsContent("Devbox", PressureState.Fresh(unknown)).rows[1]
        assertEquals(
            PressureDetailRow(
                PressureMetric.MemoryAvailablePercent,
                "RAM available",
                "NO DATA",
                "No data",
                PressureColorRole.Muted,
            ),
            missingMemoryDetail,
        )
        assertEquals(
            "Stale pressure. Last recovering from hot. Causes: disk, load.",
            pressureDetailsContent(
                "Devbox",
                PressureState.Stale(hotRecovery, GatewayFailure.Transport),
            ).summary,
        )
    }

    @Test
    fun `Linux rail and details keep stable order honest states and directional units`() {
        val state = PressureState.Fresh(
            response(
                level = PressureLevel.Hot,
                reasons = listOf(
                    PressureReason.Disk,
                    PressureReason.Load,
                    PressureReason.CpuPsi,
                    PressureReason.IoPsi,
                ),
                signals = linuxSignals(
                    cpu = measured(PressureMetric.CpuPercent, 34.49, PressureSignalState.Informational),
                    memory = measured(PressureMetric.MemoryAvailablePercent, 42.6, PressureSignalState.Normal),
                    swap = measured(PressureMetric.SwapUsedPercent, 0.0, PressureSignalState.Informational),
                    load = measured(PressureMetric.NormalizedLoad, 1.26, PressureSignalState.Warm),
                    disk = measured(PressureMetric.DiskAvailablePercent, 4.6, PressureSignalState.Hot),
                    cpuPsi = measured(PressureMetric.CpuPsiSomeAvg60Percent, 21.0, PressureSignalState.Warm),
                    ioPsi = measured(PressureMetric.IoPsiFullAvg60Percent, 5.25, PressureSignalState.Hot),
                ),
            ),
        )

        val rail = pressureRailContent("Devbox", state)
        assertEquals(
            "rail metrics must keep conceptual order, collapsed precision, marks, and host accents",
            listOf(
                PressureRailMetricContent(
                    PressureMetric.CpuPercent,
                    "CPU",
                    "34%",
                    "i",
                    "Informational",
                    PressureRailAccent.None,
                ),
                PressureRailMetricContent(
                    PressureMetric.MemoryAvailablePercent,
                    "MEM",
                    "43%",
                    "N",
                    "Normal",
                    PressureRailAccent.None,
                ),
                PressureRailMetricContent(
                    PressureMetric.SwapUsedPercent,
                    "SWAP",
                    "0%",
                    "i",
                    "Informational",
                    PressureRailAccent.None,
                ),
                PressureRailMetricContent(
                    PressureMetric.NormalizedLoad,
                    "LOAD",
                    "1.3",
                    "W",
                    "Warm",
                    PressureRailAccent.Gold,
                ),
                PressureRailMetricContent(
                    PressureMetric.DiskAvailablePercent,
                    "DISK",
                    "5%",
                    "H",
                    "Hot",
                    PressureRailAccent.Ember,
                ),
            ),
            rail.metrics,
        )

        val details = pressureDetailsContent("Devbox", state)
        assertEquals(
            "Fresh pressure. Steady at hot. Causes: disk, load, CPU pressure, I/O pressure.",
            details.summary,
        )
        assertEquals(
            listOf(
                "CPU used",
                "RAM available",
                "swap used",
                "normalized load",
                "disk available",
                "CPU PSI some, 60-second average",
                "memory PSI full, 60-second average",
                "I/O PSI full, 60-second average",
            ),
            details.rows.map(PressureDetailRow::fullLabel),
        )
        assertEquals(
            listOf("34.49%", "42.6%", "0%", "1.26", "4.6%", "21%", "0%", "5.25%"),
            details.rows.map(PressureDetailRow::value),
        )
        assertEquals(
            listOf(
                PressureColorRole.Frost,
                PressureColorRole.Moss,
                PressureColorRole.Frost,
                PressureColorRole.Gold,
                PressureColorRole.Ember,
                PressureColorRole.Gold,
                PressureColorRole.Moss,
                PressureColorRole.Ember,
            ),
            details.rows.map(PressureDetailRow::colorRole),
        )
    }

    @Test
    fun `Darwin substitutes native memory and never invents unsupported rows`() {
        val state = PressureState.Fresh(
            response(
                level = PressureLevel.Warm,
                reasons = listOf(PressureReason.Memory),
                unsupported = darwinUnsupported,
                signals = listOf(
                    measured(PressureMetric.CpuPercent, 9.5, PressureSignalState.Informational),
                    measured(PressureMetric.NormalizedLoad, 0.75, PressureSignalState.Normal),
                    measured(PressureMetric.SwapUsedPercent, 2.0, PressureSignalState.Informational),
                    measured(PressureMetric.DiskAvailablePercent, 55.0, PressureSignalState.Normal),
                    PressureSignal.Measured(
                        PressureValue.MemoryPressure(SystemMemoryPressure.Warning),
                        PressureSignalState.Warm,
                    ),
                ),
            ),
        )

        val rail = pressureRailContent("MacBook", state)
        assertEquals(
            listOf(
                PressureRailMetricContent(
                    PressureMetric.CpuPercent,
                    "CPU",
                    "10%",
                    "i",
                    "Informational",
                    PressureRailAccent.None,
                ),
                PressureRailMetricContent(
                    PressureMetric.MemoryPressure,
                    "MEM",
                    "WARNING",
                    "W",
                    "Warm",
                    PressureRailAccent.Gold,
                ),
                PressureRailMetricContent(
                    PressureMetric.SwapUsedPercent,
                    "SWAP",
                    "2%",
                    "i",
                    "Informational",
                    PressureRailAccent.None,
                ),
                PressureRailMetricContent(
                    PressureMetric.NormalizedLoad,
                    "LOAD",
                    "0.8",
                    "N",
                    "Normal",
                    PressureRailAccent.None,
                ),
                PressureRailMetricContent(
                    PressureMetric.DiskAvailablePercent,
                    "DISK",
                    "55%",
                    "N",
                    "Normal",
                    PressureRailAccent.None,
                ),
            ),
            rail.metrics,
        )
        val details = pressureDetailsContent("MacBook", state)
        assertEquals(
            listOf("CPU used", "system memory pressure", "swap used", "normalized load", "disk available"),
            details.rows.map(PressureDetailRow::fullLabel),
        )
        assertEquals(
            listOf("9.5%", "WARNING", "2%", "0.75", "55%"),
            details.rows.map(PressureDetailRow::value),
        )
        assertFalse(
            "unsupported capability inventory is protocol validation, never product content",
            "$rail$details"
                .contains("unsupported", ignoreCase = true),
        )
    }

    @Test
    fun `rail accessibility names header facts missing detail-only evidence and every metric state`() {
        val state = PressureState.Fresh(
            response(
                level = PressureLevel.Unknown,
                signals = linuxSignals(
                    cpuPsi = PressureSignal.Missing(PressureMetric.CpuPsiSomeAvg60Percent),
                ),
            ),
        )

        assertEquals(
            "Devbox. Fresh pressure. Steady at unknown. " +
                "CPU 18% i, informational; MEM 72% N, normal; SWAP 0% i, informational; " +
                "LOAD 0.4 N, normal; DISK 60% N, normal. CPU PSI NO DATA ?, no data. " +
                "Trend: unknown 5 seconds.",
            pressureRailContent("Devbox", state).accessibilitySummary,
        )
    }

    @Test
    fun `rail semantics compress contiguous history and include the action without color dependence`() {
        val start = Instant.parse("2026-08-27T12:00:00Z")
        val levels = listOf(
            PressureLevel.Normal,
            PressureLevel.Warm,
            PressureLevel.Normal,
            PressureLevel.Warm,
            PressureLevel.Warm,
            PressureLevel.Hot,
        )
        val current = PressureSample(
            sampledAt = start.plusSeconds(30),
            level = PressureLevel.Hot,
            phase = PressurePhase.Steady,
            reasons = listOf(PressureReason.Disk, PressureReason.Load),
            signals = linuxSignals(
                swap = PressureSignal.Missing(PressureMetric.SwapUsedPercent),
                load = measured(PressureMetric.NormalizedLoad, 1.2, PressureSignalState.Warm),
                disk = measured(PressureMetric.DiskAvailablePercent, 4.0, PressureSignalState.Hot),
            ),
        )
        val response = PressureResponse(
            unsupported = linuxUnsupported,
            current = current,
            history = levels.zip(listOf(0L, 5L, 10L, 15L, 25L, 30L)) { level, offset ->
                PressureHistorySample(start.plusSeconds(offset), level)
            },
        )

        val rail = pressureRailContent("Devbox", PressureState.Fresh(response))
        assertEquals(
            "Trend: 2 earlier runs over 10 seconds; normal 5 seconds, then warm 15 seconds, then hot 5 seconds.",
            rail.historySummary,
        )
        assertEquals("Show Devbox pressure details", rail.actionLabel)
        assertEquals(
            PressureRailHeaderContent("Devbox", "HOT · DISK +1", PressureRailAccent.Ember),
            rail.header,
        )
        assertEquals(
            "Devbox. Fresh pressure. Steady at hot. Causes: disk, load. " +
                "CPU 18% i, informational; MEM 72% N, normal; SWAP NO DATA ?, no data; " +
                "LOAD 1.2 W, warm; DISK 4% H, hot. ${rail.historySummary}",
            rail.accessibilitySummary,
        )
        assertFalse(rail.accessibilitySummary.contains("Recent pressure history"))
        assertFalse(rail.accessibilitySummary.contains("up to 15 min"))
    }

    @Test
    fun `reading and unavailable details stay factual without fabricated metrics`() {
        val cases = listOf(
            PressureState.Reading to "Reading pressure.",
            PressureState.Unavailable(GatewayFailure.Transport) to "Pressure unavailable.",
        )

        cases.forEach { (state, expectedSummary) ->
            val rail = pressureRailContent("Devbox", state)
            val details = pressureDetailsContent("Devbox", state)
            val expectedHeader = when (state) {
                PressureState.Reading ->
                    PressureRailHeaderContent("Devbox", "READING", PressureRailAccent.Gold)
                is PressureState.Unavailable -> PressureRailHeaderContent(
                    "Devbox",
                    "PRESSURE UNAVAILABLE",
                    PressureRailAccent.Muted,
                )
                else -> error("header-only cases contain only reading and unavailable")
            }
            assertEquals(expectedHeader, rail.header)
            assertTrue("no accepted snapshot means no fabricated metric values", rail.metrics.isEmpty())
            assertEquals("No pressure samples yet.", rail.historySummary)
            assertEquals(
                "Devbox. ${expectedSummary.removeSuffix(".")}. " +
                    "No pressure samples yet.",
                rail.accessibilitySummary,
            )
            assertEquals("Show Devbox pressure details", rail.actionLabel)
            assertEquals(expectedSummary, details.summary)
            assertTrue("no accepted snapshot means no fabricated detail rows", details.rows.isEmpty())
            assertEquals("Devbox pressure", details.title)
            assertEquals("Dismiss Devbox pressure details", details.dismissLabel)
        }
    }

    private fun response(
        level: PressureLevel,
        phase: PressurePhase = PressurePhase.Steady,
        reasons: List<PressureReason> = emptyList(),
        unsupported: List<PressureMetric> = linuxUnsupported,
        signals: List<PressureSignal>,
    ): PressureResponse {
        val sampledAt = Instant.parse("2026-08-27T12:00:00Z")
        return PressureResponse(
            unsupported = unsupported,
            current = PressureSample(sampledAt, level, phase, reasons, signals),
            history = listOf(PressureHistorySample(sampledAt, level)),
        )
    }

    private fun linuxSignals(
        cpu: PressureSignal = measured(PressureMetric.CpuPercent, 18.0, PressureSignalState.Informational),
        load: PressureSignal = measured(PressureMetric.NormalizedLoad, 0.4, PressureSignalState.Normal),
        memory: PressureSignal = measured(
            PressureMetric.MemoryAvailablePercent,
            72.0,
            PressureSignalState.Normal,
        ),
        swap: PressureSignal = measured(PressureMetric.SwapUsedPercent, 0.0, PressureSignalState.Informational),
        disk: PressureSignal = measured(PressureMetric.DiskAvailablePercent, 60.0, PressureSignalState.Normal),
        cpuPsi: PressureSignal = measured(PressureMetric.CpuPsiSomeAvg60Percent, 0.0, PressureSignalState.Normal),
        memoryPsi: PressureSignal = measured(
            PressureMetric.MemoryPsiFullAvg60Percent,
            0.0,
            PressureSignalState.Normal,
        ),
        ioPsi: PressureSignal = measured(PressureMetric.IoPsiFullAvg60Percent, 0.0, PressureSignalState.Normal),
    ): List<PressureSignal> = listOf(cpu, load, memory, swap, disk, cpuPsi, memoryPsi, ioPsi)

    private fun measured(
        metric: PressureMetric,
        value: Double,
        state: PressureSignalState,
    ): PressureSignal = PressureSignal.Measured(
        when (metric) {
            PressureMetric.CpuPercent -> PressureValue.CpuPercent(value)
            PressureMetric.NormalizedLoad -> PressureValue.NormalizedLoad(value)
            PressureMetric.MemoryAvailablePercent -> PressureValue.MemoryAvailablePercent(value)
            PressureMetric.SwapUsedPercent -> PressureValue.SwapUsedPercent(value)
            PressureMetric.DiskAvailablePercent -> PressureValue.DiskAvailablePercent(value)
            PressureMetric.CpuPsiSomeAvg60Percent -> PressureValue.CpuPsiSomeAvg60Percent(value)
            PressureMetric.MemoryPsiFullAvg60Percent -> PressureValue.MemoryPsiFullAvg60Percent(value)
            PressureMetric.IoPsiFullAvg60Percent -> PressureValue.IoPsiFullAvg60Percent(value)
            PressureMetric.MemoryPressure -> error("memory pressure is categorical")
        },
        state,
    )

    private companion object {
        val linuxUnsupported = listOf(PressureMetric.MemoryPressure)
        val darwinUnsupported = listOf(
            PressureMetric.MemoryAvailablePercent,
            PressureMetric.CpuPsiSomeAvg60Percent,
            PressureMetric.MemoryPsiFullAvg60Percent,
            PressureMetric.IoPsiFullAvg60Percent,
        )
    }
}
