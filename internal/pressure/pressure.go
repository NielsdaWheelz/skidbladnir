package pressure

import (
	"context"
	"sync"
	"time"
)

const (
	SampleInterval     = 5 * time.Second
	WindowDuration     = 15 * time.Minute
	HistorySampleLimit = int(WindowDuration / SampleInterval)
	DeescalationDelay  = 60 * time.Second
	memoryWarmPercent  = 15.0
	memoryHotPercent   = 8.0
	diskWarmPercent    = 15.0
	diskHotPercent     = 5.0
	loadWarm           = 1.0
	loadHot            = 2.0
	cpuPSIWarmPercent  = 20.0
	cpuPSIHotPercent   = 50.0
	fullPSIWarmPercent = 1.0
	fullPSIHotPercent  = 5.0
)

type Status string

const (
	StatusNormal  Status = "Normal"
	StatusWarm    Status = "Warm"
	StatusHot     Status = "Hot"
	StatusUnknown Status = "Unknown"
)

type Reason string

const (
	ReasonMemory    Reason = "Memory"
	ReasonDisk      Reason = "Disk"
	ReasonLoad      Reason = "Load"
	ReasonCPUPSI    Reason = "CpuPsi"
	ReasonMemoryPSI Reason = "MemoryPsi"
	ReasonIOPSI     Reason = "IoPsi"
)

type Signal struct {
	Status Status
	value  float64
	known  bool
}

func (signal Signal) Value() (float64, bool) {
	return signal.value, signal.known
}

type Sample struct {
	ObservedAt                   time.Time
	Status                       Status
	Reasons                      []Reason
	CPUPercent                   Signal
	LoadNormalized               Signal
	MemoryAvailablePercent       Signal
	SwapUsedPercent              Signal
	DiskAvailablePercent         Signal
	CPUPressureSomeAvg60         Signal
	MemoryPressureFullAvg60      Signal
	InputOutputPressureFullAvg60 Signal
	MemoryPressure               MemoryPressureSignal
}

type MemoryPressureSignal struct {
	Status Status
	value  MemoryPressure
	known  bool
}

func (signal MemoryPressureSignal) Value() (MemoryPressure, bool) { return signal.value, signal.known }

type Snapshot struct {
	Current Sample
	Window  []Sample
}

type Monitor struct {
	mutex     sync.RWMutex
	collector collector
	evaluator evaluator
	policy    policy
	current   Sample
	window    []Sample
}

func NewMonitor() *Monitor {
	monitor := &Monitor{collector: newCollector(), evaluator: newEvaluator(), policy: currentPolicy()}
	monitor.sample(time.Now())
	return monitor
}

func (monitor *Monitor) Run(ctx context.Context) {
	// justify-polling: Linux exposes these host-pressure counters as snapshots;
	// five seconds is the product's named freshness/cost tradeoff.
	ticker := time.NewTicker(SampleInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case observedAt := <-ticker.C:
			monitor.sample(observedAt)
		}
	}
}

func (monitor *Monitor) Snapshot() Snapshot {
	monitor.mutex.RLock()
	defer monitor.mutex.RUnlock()
	window := make([]Sample, len(monitor.window))
	for index, sample := range monitor.window {
		window[index] = cloneSample(sample)
	}
	return Snapshot{Current: cloneSample(monitor.current), Window: window}
}

func (monitor *Monitor) Unsupported() []Metric { return monitor.policy.Unsupported() }

func (monitor *Monitor) sample(observedAt time.Time) {
	raw := monitor.collector.collect()
	evaluation := monitor.evaluator.observe(raw, observedAt, monitor.policy)
	sample := Sample{
		ObservedAt:                   observedAt.UTC(),
		Status:                       evaluation.status,
		Reasons:                      evaluation.reasons,
		CPUPercent:                   signal(raw.cpuPercent, classifyInformational),
		LoadNormalized:               signal(raw.loadNormalized, classifyLoad),
		MemoryAvailablePercent:       signal(raw.memoryAvailablePercent, classifyMemory),
		SwapUsedPercent:              signal(raw.swapUsedPercent, classifyInformational),
		DiskAvailablePercent:         signal(raw.diskAvailablePercent, classifyDisk),
		CPUPressureSomeAvg60:         signal(raw.cpuPSISomeAvg60, classifyCPUPSI),
		MemoryPressureFullAvg60:      signal(raw.memoryPSIFullAvg60, classifyFullPSI),
		InputOutputPressureFullAvg60: signal(raw.ioPSIFullAvg60, classifyFullPSI),
		MemoryPressure:               memoryPressureSignal(raw.memoryPressure),
	}

	monitor.mutex.Lock()
	defer monitor.mutex.Unlock()
	monitor.current = sample
	monitor.window = retainHistory(monitor.window, sample)
}

func retainHistory(history []Sample, sample Sample) []Sample {
	cutoff := sample.ObservedAt.Add(-WindowDuration)
	first := 0
	for first < len(history) && history[first].ObservedAt.Before(cutoff) {
		first++
	}
	history = append(history[first:], sample)
	if len(history) > HistorySampleLimit {
		history = history[len(history)-HistorySampleLimit:]
	}
	return history
}

type evaluator struct {
	initialized    bool
	stable         Status
	stableReasons  []Reason
	deescalatingAt time.Time
}

func newEvaluator() evaluator { return evaluator{} }

type evaluation struct {
	status  Status
	reasons []Reason
}

func (evaluator *evaluator) observe(sample rawSample, observedAt time.Time, policy policy) evaluation {
	raw := classifyOverall(sample, policy)
	if raw.status == StatusUnknown {
		evaluator.deescalatingAt = time.Time{}
		return raw
	}
	if !evaluator.initialized {
		evaluator.initialized = true
		evaluator.stable = raw.status
		evaluator.stableReasons = raw.reasons
		return raw
	}
	if severity(raw.status) > severity(evaluator.stable) {
		evaluator.stable = raw.status
		evaluator.stableReasons = raw.reasons
		evaluator.deescalatingAt = time.Time{}
		return raw
	}
	if raw.status == evaluator.stable {
		evaluator.stableReasons = raw.reasons
		evaluator.deescalatingAt = time.Time{}
		return raw
	}
	if evaluator.deescalatingAt.IsZero() {
		evaluator.deescalatingAt = observedAt
		return evaluation{status: evaluator.stable, reasons: cloneReasons(evaluator.stableReasons)}
	}
	if observedAt.Sub(evaluator.deescalatingAt) >= DeescalationDelay {
		evaluator.stable = statusForSeverity(severity(evaluator.stable) - 1)
		if evaluator.stable == StatusNormal {
			evaluator.stableReasons = nil
		} else if raw.status == evaluator.stable {
			evaluator.stableReasons = raw.reasons
		}
		evaluator.deescalatingAt = time.Time{}
	}
	return evaluation{status: evaluator.stable, reasons: cloneReasons(evaluator.stableReasons)}
}

// requiredClassifiers is the closed table of metrics a policy may declare as
// classification inputs; newPolicy admits required metrics only from this
// table, so the required set and the classifier set cannot drift apart.
var requiredClassifiers = map[Metric]struct {
	reason   Reason
	classify func(rawSample) Status
}{
	MetricMemoryAvailablePercent:       {ReasonMemory, func(sample rawSample) Status { return classifyMemory(sample.memoryAvailablePercent) }},
	MetricDiskAvailablePercent:         {ReasonDisk, func(sample rawSample) Status { return classifyDisk(sample.diskAvailablePercent) }},
	MetricLoadNormalized:               {ReasonLoad, func(sample rawSample) Status { return classifyLoad(sample.loadNormalized) }},
	MetricCPUPressureSomeAvg60:         {ReasonCPUPSI, func(sample rawSample) Status { return classifyCPUPSI(sample.cpuPSISomeAvg60) }},
	MetricMemoryPressureFullAvg60:      {ReasonMemoryPSI, func(sample rawSample) Status { return classifyFullPSI(sample.memoryPSIFullAvg60) }},
	MetricInputOutputPressureFullAvg60: {ReasonIOPSI, func(sample rawSample) Status { return classifyFullPSI(sample.ioPSIFullAvg60) }},
	MetricMemoryPressure:               {ReasonMemory, func(sample rawSample) Status { return classifyMemoryPressure(sample.memoryPressure) }},
}

func classifyOverall(sample rawSample, policy policy) evaluation {
	result := evaluation{status: StatusNormal}
	missing := false
	for _, metric := range policy.required {
		classifier := requiredClassifiers[metric]
		switch classifier.classify(sample) {
		case StatusHot:
			result.status = StatusHot
			result.reasons = append(result.reasons, classifier.reason)
		case StatusWarm:
			if result.status != StatusHot {
				result.status = StatusWarm
			}
			result.reasons = append(result.reasons, classifier.reason)
		case StatusUnknown:
			missing = true
		case StatusNormal:
		}
	}
	if missing {
		result.status = StatusUnknown
	}
	return result
}

func memoryPressureSignal(value memoryPressureMetric) MemoryPressureSignal {
	if !value.known {
		return MemoryPressureSignal{Status: StatusUnknown}
	}
	return MemoryPressureSignal{Status: classifyMemoryPressure(value), value: value.value, known: true}
}

func classifyMemoryPressure(value memoryPressureMetric) Status {
	if !value.known {
		return StatusUnknown
	}
	switch value.value {
	case MemoryPressureNormal:
		return StatusNormal
	case MemoryPressureWarning:
		return StatusWarm
	case MemoryPressureCritical:
		return StatusHot
	default:
		panic("invalid memory pressure") // justify-defect: the closed collector value escaped its exhaustive boundary.
	}
}

func signal(value metric, classify func(metric) Status) Signal {
	if !value.known {
		return Signal{Status: StatusUnknown}
	}
	return Signal{Status: classify(value), value: value.value, known: true}
}

func classifyInformational(value metric) Status {
	if !value.known {
		return StatusUnknown
	}
	return StatusNormal
}

func classifyMemory(value metric) Status {
	return classifyLower(value, memoryWarmPercent, memoryHotPercent)
}

func classifyDisk(value metric) Status {
	return classifyLower(value, diskWarmPercent, diskHotPercent)
}

func classifyLoad(value metric) Status {
	return classifyHigher(value, loadWarm, loadHot)
}

func classifyCPUPSI(value metric) Status {
	return classifyHigher(value, cpuPSIWarmPercent, cpuPSIHotPercent)
}

func classifyFullPSI(value metric) Status {
	return classifyHigher(value, fullPSIWarmPercent, fullPSIHotPercent)
}

func classifyLower(value metric, warm, hot float64) Status {
	if !value.known {
		return StatusUnknown
	}
	if value.value <= hot {
		return StatusHot
	}
	if value.value <= warm {
		return StatusWarm
	}
	return StatusNormal
}

func classifyHigher(value metric, warm, hot float64) Status {
	if !value.known {
		return StatusUnknown
	}
	if value.value >= hot {
		return StatusHot
	}
	if value.value >= warm {
		return StatusWarm
	}
	return StatusNormal
}

func severity(status Status) int {
	switch status {
	case StatusNormal:
		return 0
	case StatusWarm:
		return 1
	case StatusHot:
		return 2
	case StatusUnknown:
		panic("UNKNOWN has no pressure severity")
	default:
		panic("invalid pressure status")
	}
}

func statusForSeverity(value int) Status {
	switch value {
	case 0:
		return StatusNormal
	case 1:
		return StatusWarm
	case 2:
		return StatusHot
	default:
		panic("invalid pressure severity")
	}
}

func cloneReasons(reasons []Reason) []Reason {
	if len(reasons) == 0 {
		return nil
	}
	cloned := make([]Reason, len(reasons))
	copy(cloned, reasons)
	return cloned
}

func cloneSample(sample Sample) Sample {
	sample.Reasons = cloneReasons(sample.Reasons)
	return sample
}
