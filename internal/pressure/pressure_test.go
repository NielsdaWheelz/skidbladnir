package pressure

import (
	"slices"
	"testing"
	"time"
)

func TestPressureThresholdsAndHysteresis(t *testing.T) {
	now := time.Unix(1_000, 0)
	evaluator := newEvaluator()
	policy := linuxPolicy()

	normal := completeRawSample(50, 50, 0.5, 10, 0.5, 0.5)
	if got := evaluator.observe(normal, now, policy); got.status != StatusNormal || got.phase != PhaseSteady {
		t.Fatalf("normal evaluation = %+v, want Normal/Steady", got)
	}

	warmBoundary := completeRawSample(15, 15, 1, 20, 1, 1)
	if got := evaluator.observe(warmBoundary, now.Add(time.Second), policy); got.status != StatusWarm {
		t.Fatalf("warm-boundary status = %q, want %q", got.status, StatusWarm)
	}

	hotBoundary := completeRawSample(8, 5, 2, 50, 5, 5)
	hot := evaluator.observe(hotBoundary, now.Add(2*time.Second), policy)
	if hot.status != StatusHot {
		t.Fatalf("hot-boundary status = %q, want %q", hot.status, StatusHot)
	}
	wantHotReasons := []Reason{ReasonMemory, ReasonDisk, ReasonLoad, ReasonCPUPSI, ReasonMemoryPSI, ReasonIOPSI}
	if !slices.Equal(hot.reasons, wantHotReasons) {
		t.Fatalf("hot-boundary reasons = %v, want %v", hot.reasons, wantHotReasons)
	}

	if got := evaluator.observe(normal, now.Add(61*time.Second), policy); got.status != StatusHot || got.phase != PhaseRecovering || !slices.Equal(got.reasons, wantHotReasons) {
		t.Fatalf("evaluation before continuous de-escalation delay = %+v, want held Hot/Recovering", got)
	}
	if got := evaluator.observe(normal, now.Add(121*time.Second), policy); got.status != StatusWarm || got.phase != PhaseRecovering || !slices.Equal(got.reasons, wantHotReasons) {
		t.Fatalf("evaluation after first de-escalation delay = %+v, want held Warm/Recovering", got)
	}
	if got := evaluator.observe(normal, now.Add(122*time.Second), policy); got.status != StatusWarm {
		t.Fatalf("second de-escalation started at %q, want %q", got.status, StatusWarm)
	}
	if got := evaluator.observe(normal, now.Add(182*time.Second), policy); got.status != StatusNormal || got.phase != PhaseSteady {
		t.Fatalf("evaluation after second de-escalation delay = %+v, want Normal/Steady", got)
	}

	if got := evaluator.observe(hotBoundary, now.Add(183*time.Second), policy); got.status != StatusHot {
		t.Fatalf("immediate escalation status = %q, want %q", got.status, StatusHot)
	}
	if got := evaluator.observe(unknownRawSample(), now.Add(184*time.Second), policy); got.status != StatusUnknown || got.phase != PhaseSteady {
		t.Fatalf("missing-metric evaluation = %+v, want Unknown/Steady", got)
	}
	if got := evaluator.observe(normal, now.Add(185*time.Second), policy); got.status != StatusHot {
		t.Fatalf("recovered measurement bypassed de-escalation delay: got %q, want %q", got.status, StatusHot)
	}

	hotWithLaterMissing := completeRawSample(8, 50, 0.5, 10, 0.5, 0.5)
	hotWithLaterMissing.ioPSIFullAvg60 = metric{}
	missingEvaluator := newEvaluator()
	if got := missingEvaluator.observe(hotWithLaterMissing, now, policy); got.status != StatusUnknown {
		t.Fatalf("known hot metric hid a later missing threshold input: got %q, want %q", got.status, StatusUnknown)
	}
}

func TestPressureSignalStatusesRemainIndependentOfTheAggregateVerdict(t *testing.T) {
	sample := completeRawSample(50, 5, 1, 10, 0.5, 0.5)
	sample.cpuPercent = knownMetric(91)
	sample.swapUsedPercent = knownMetric(72)

	if got := informationalSignal(sample.cpuPercent).Status; got != SignalStatusInformational {
		t.Fatalf("CPU signal status = %q, want Informational", got)
	}
	if got := informationalSignal(sample.swapUsedPercent).Status; got != SignalStatusInformational {
		t.Fatalf("swap signal status = %q, want Informational", got)
	}
	if got := signal(sample.loadNormalized, classifyLoad).Status; got != SignalStatusWarm {
		t.Fatalf("load signal status = %q, want Warm", got)
	}
	if got := signal(sample.diskAvailablePercent, classifyDisk).Status; got != SignalStatusHot {
		t.Fatalf("disk signal status = %q, want Hot", got)
	}

	aggregate := classifyOverall(sample, linuxPolicy())
	if aggregate.status != StatusHot || !slices.Equal(aggregate.reasons, []Reason{ReasonDisk, ReasonLoad}) {
		t.Fatalf("aggregate verdict = %+v, want Hot from disk and load only", aggregate)
	}
}

func TestDarwinPolicyUsesNativeMemoryPressureAndNotLinuxMetrics(t *testing.T) {
	policy := darwinPolicy()
	sample := rawSample{diskAvailablePercent: knownMetric(50), loadNormalized: knownMetric(0.5), memoryPressure: knownMemoryPressure(MemoryPressureWarning)}
	if got := classifyOverall(sample, policy); got.status != StatusWarm || !slices.Equal(got.reasons, []Reason{ReasonMemory}) {
		t.Fatalf("Darwin warning classification = %+v, want %q with reasons %v", got, StatusWarm, []Reason{ReasonMemory})
	}
	sample.memoryPressure = knownMemoryPressure(MemoryPressureCritical)
	if got := classifyOverall(sample, policy); got.status != StatusHot {
		t.Fatalf("Darwin critical classification = %+v, want %q", got, StatusHot)
	}
	want := []Metric{MetricCPUPressureSomeAvg60, MetricInputOutputPressureFullAvg60, MetricMemoryAvailablePercent, MetricMemoryPressureFullAvg60}
	if got := policy.Unsupported(); !slices.Equal(got, want) {
		t.Fatalf("Darwin unsupported = %v, want %v", got, want)
	}
}

func TestLinuxPolicyDeclaresTheContractedCapabilityPartition(t *testing.T) {
	policy := linuxPolicy()
	wantUnsupported := []Metric{MetricMemoryPressure}
	if got := policy.Unsupported(); !slices.Equal(got, wantUnsupported) {
		t.Fatalf("Linux unsupported = %v, want %v", got, wantUnsupported)
	}
	wantRequired := []Metric{MetricMemoryAvailablePercent, MetricDiskAvailablePercent, MetricLoadNormalized, MetricCPUPressureSomeAvg60, MetricMemoryPressureFullAvg60, MetricInputOutputPressureFullAvg60}
	if !slices.Equal(policy.required, wantRequired) {
		t.Fatalf("Linux required classification inputs = %v, want %v", policy.required, wantRequired)
	}
}

func TestPolicyConstructionOwnsTheCapabilityInvariants(t *testing.T) {
	unordered := newPolicy(nil, []Metric{MetricMemoryPressure, MetricDiskAvailablePercent})
	if got, want := unordered.Unsupported(), []Metric{MetricDiskAvailablePercent, MetricMemoryPressure}; !slices.Equal(got, want) {
		t.Fatalf("unsupported set was not canonicalized at construction: got %v, want %v", got, want)
	}

	assertPolicyDefect(t, "duplicate unsupported metric", func() {
		newPolicy(nil, []Metric{MetricMemoryPressure, MetricMemoryPressure})
	})
	assertPolicyDefect(t, "required metric listed as unsupported", func() {
		newPolicy([]Metric{MetricLoadNormalized}, []Metric{MetricLoadNormalized})
	})
	assertPolicyDefect(t, "required metric without a classifier", func() {
		newPolicy([]Metric{MetricCPUPercent}, nil)
	})
}

func assertPolicyDefect(t *testing.T, name string, construct func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatalf("policy construction accepted a %s", name)
		}
	}()
	construct()
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
