package pressure

import (
	"slices"
	"testing"
	"time"
)

func TestPressureThresholdsAndHysteresis(t *testing.T) {
	now := time.Unix(1_000, 0)
	evaluator := newEvaluator()

	normal := completeRawSample(50, 50, 0.5, 10, 0.5, 0.5)
	if got := evaluator.observe(normal, now); got.status != StatusNormal {
		t.Fatalf("normal status = %q, want %q", got.status, StatusNormal)
	}

	warmBoundary := completeRawSample(15, 15, 1, 20, 1, 1)
	if got := evaluator.observe(warmBoundary, now.Add(time.Second)); got.status != StatusWarm {
		t.Fatalf("warm-boundary status = %q, want %q", got.status, StatusWarm)
	}

	hotBoundary := completeRawSample(8, 5, 2, 50, 5, 5)
	hot := evaluator.observe(hotBoundary, now.Add(2*time.Second))
	if hot.status != StatusHot {
		t.Fatalf("hot-boundary status = %q, want %q", hot.status, StatusHot)
	}
	wantHotReasons := []Reason{ReasonMemory, ReasonDisk, ReasonLoad, ReasonCPUPSI, ReasonMemoryPSI, ReasonIOPSI}
	if !slices.Equal(hot.reasons, wantHotReasons) {
		t.Fatalf("hot-boundary reasons = %v, want %v", hot.reasons, wantHotReasons)
	}

	if got := evaluator.observe(normal, now.Add(61*time.Second)); got.status != StatusHot || !slices.Equal(got.reasons, wantHotReasons) {
		t.Fatalf("status before continuous de-escalation delay = %q, want %q", got.status, StatusHot)
	}
	if got := evaluator.observe(normal, now.Add(121*time.Second)); got.status != StatusWarm || !slices.Equal(got.reasons, wantHotReasons) {
		t.Fatalf("status after first de-escalation delay = %q, want %q", got.status, StatusWarm)
	}
	if got := evaluator.observe(normal, now.Add(122*time.Second)); got.status != StatusWarm {
		t.Fatalf("second de-escalation started at %q, want %q", got.status, StatusWarm)
	}
	if got := evaluator.observe(normal, now.Add(182*time.Second)); got.status != StatusNormal {
		t.Fatalf("status after second de-escalation delay = %q, want %q", got.status, StatusNormal)
	}

	if got := evaluator.observe(hotBoundary, now.Add(183*time.Second)); got.status != StatusHot {
		t.Fatalf("immediate escalation status = %q, want %q", got.status, StatusHot)
	}
	if got := evaluator.observe(unknownRawSample(), now.Add(184*time.Second)); got.status != StatusUnknown {
		t.Fatalf("missing-metric status = %q, want %q", got.status, StatusUnknown)
	}
	if got := evaluator.observe(normal, now.Add(185*time.Second)); got.status != StatusHot {
		t.Fatalf("recovered measurement bypassed de-escalation delay: got %q, want %q", got.status, StatusHot)
	}

	hotWithLaterMissing := completeRawSample(8, 50, 0.5, 10, 0.5, 0.5)
	hotWithLaterMissing.ioPSIFullAvg60 = metric{}
	missingEvaluator := newEvaluator()
	if got := missingEvaluator.observe(hotWithLaterMissing, now); got.status != StatusUnknown {
		t.Fatalf("known hot metric hid a later missing threshold input: got %q, want %q", got.status, StatusUnknown)
	}
}

func TestPressureHistoryIsChronologicalAndCappedAt180Samples(t *testing.T) {
	start := time.Unix(2_000, 0).UTC()
	history := make([]Sample, 0, HistorySampleLimit+1)
	for index := 0; index <= HistorySampleLimit; index++ {
		history = retainHistory(history, Sample{ObservedAt: start.Add(time.Duration(index) * SampleInterval)})
	}
	if len(history) != HistorySampleLimit {
		t.Fatalf("history length = %d, want %d", len(history), HistorySampleLimit)
	}
	if got, want := history[0].ObservedAt, start.Add(SampleInterval); !got.Equal(want) {
		t.Fatalf("history first sample = %s, want %s", got, want)
	}
	if got, want := history[len(history)-1].ObservedAt, start.Add(time.Duration(HistorySampleLimit)*SampleInterval); !got.Equal(want) {
		t.Fatalf("history final sample = %s, want current %s", got, want)
	}
}

func TestPressureSnapshotDoesNotExposeMonitorReasonStorage(t *testing.T) {
	monitor := &Monitor{
		current: Sample{Reasons: []Reason{ReasonMemory}},
		window:  []Sample{{Reasons: []Reason{ReasonDisk}}},
	}
	first := monitor.Snapshot()
	first.Current.Reasons[0] = ReasonLoad
	first.Window[0].Reasons[0] = ReasonIOPSI

	second := monitor.Snapshot()
	if got := second.Current.Reasons[0]; got != ReasonMemory {
		t.Fatalf("current reasons were mutable through Snapshot: got %q", got)
	}
	if got := second.Window[0].Reasons[0]; got != ReasonDisk {
		t.Fatalf("history reasons were mutable through Snapshot: got %q", got)
	}
}

func TestCPUCounterAnomalyIsUnknownInsteadOfUnderflowing(t *testing.T) {
	if got := cpuUsage(cpuCounters{total: 100, idle: 40}, cpuCounters{total: 110, idle: 60}); got.known {
		t.Fatalf("idle delta larger than total delta produced a known value: %+v", got)
	}
	got := cpuUsage(cpuCounters{total: 100, idle: 40}, cpuCounters{total: 120, idle: 45})
	if !got.known || got.value != 75 {
		t.Fatalf("valid CPU counters produced %+v, want 75%%", got)
	}
}

func TestProcParsersRejectMalformedRequiredMetrics(t *testing.T) {
	total, available, swapTotal, swapFree := parseMemory([]byte("MemTotal: 100 kB\nMemAvailable: 40 kB\nSwapTotal: 20 kB\nSwapFree: 5 kB\n"))
	for name, metric := range map[string]metric{
		"MemTotal": total, "MemAvailable": available, "SwapTotal": swapTotal, "SwapFree": swapFree,
	} {
		if !metric.known {
			t.Fatalf("valid %s metric was not parsed", name)
		}
	}

	total, available, _, _ = parseMemory([]byte("MemTotal: 100 pages\nMemAvailable: 40\n"))
	if total.known || available.known {
		t.Fatalf("meminfo values without the kernel kB unit were accepted: total=%+v available=%+v", total, available)
	}
	decimal, _, _, _ := parseMemory([]byte("MemTotal: 100.5 kB\n"))
	if decimal.known {
		t.Fatalf("non-integer meminfo value was accepted: %+v", decimal)
	}

	psi := parsePSI([]byte("some avg10=0.00 avg60=12.50 avg300=0.00 total=1\n"), "some")
	if !psi.known || psi.value != 12.5 {
		t.Fatalf("valid PSI avg60 produced %+v, want 12.5", psi)
	}
	if got := parsePSI([]byte("full avg60=101.00\n"), "full"); got.known {
		t.Fatalf("out-of-range PSI percentage was accepted: %+v", got)
	}
}

func completeRawSample(memory, disk, load, cpuPSI, memoryPSI, ioPSI float64) rawSample {
	return rawSample{
		memoryAvailablePercent: knownMetric(memory),
		diskAvailablePercent:   knownMetric(disk),
		loadNormalized:         knownMetric(load),
		cpuPSISomeAvg60:        knownMetric(cpuPSI),
		memoryPSIFullAvg60:     knownMetric(memoryPSI),
		ioPSIFullAvg60:         knownMetric(ioPSI),
	}
}

func unknownRawSample() rawSample {
	sample := completeRawSample(50, 50, 0.5, 10, 0.5, 0.5)
	sample.memoryPSIFullAvg60 = metric{}
	return sample
}
