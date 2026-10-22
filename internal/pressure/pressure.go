// Package pressure samples host pressure, holds its level with hysteresis and
// owns the GET /v1/pressure wire: json.Marshal(Snapshot) is the response body.
package pressure

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"sync"
	"time"
)

// Metric names one observation by its wire key.
type Metric string

const (
	cpuPercent                Metric = "cpuPercent"
	normalizedLoad            Metric = "normalizedLoad"
	memoryAvailablePercent    Metric = "memoryAvailablePercent"
	swapUsedPercent           Metric = "swapUsedPercent"
	diskAvailablePercent      Metric = "diskAvailablePercent"
	cpuPSISomeAvg60Percent    Metric = "cpuPsiSomeAvg60Percent"
	memoryPSIFullAvg60Percent Metric = "memoryPsiFullAvg60Percent"
	ioPSIFullAvg60Percent     Metric = "ioPsiFullAvg60Percent"
	memoryPressure            Metric = "memoryPressure"
)

// metrics lists every metric in wire order.
var metrics = []Metric{cpuPercent, normalizedLoad, memoryAvailablePercent, swapUsedPercent,
	diskAvailablePercent, cpuPSISomeAvg60Percent, memoryPSIFullAvg60Percent, ioPSIFullAvg60Percent, memoryPressure}

// Level is a sample's level or one signal's state. Only a sample is Unknown and
// only a signal is Informational.
type Level string

const (
	normal        Level = "Normal"
	warm          Level = "Warm"
	hot           Level = "Hot"
	unknown       Level = "Unknown"
	informational Level = "Informational"
)

var rank = map[Level]int{normal: 0, warm: 1, hot: 2}

var below = map[Level]Level{hot: warm, warm: normal}

// Phase is Recovering while the held level is above the sampled one.
type Phase string

const (
	steady     Phase = "Steady"
	recovering Phase = "Recovering"
)

// Reason names a required metric that is Warm or Hot.
type Reason string

// policy is one platform's capability: the metrics that set the level, in reason
// (wire) order, and the metrics it never observes, sorted bytewise.
type policy struct {
	required    []requirement
	unsupported []Metric
}

type requirement struct {
	metric Metric
	reason Reason
}

var darwinPolicy = policy{
	required:    []requirement{{diskAvailablePercent, "Disk"}, {normalizedLoad, "Load"}, {memoryPressure, "Memory"}},
	unsupported: []Metric{cpuPSISomeAvg60Percent, ioPSIFullAvg60Percent, memoryAvailablePercent, memoryPSIFullAvg60Percent},
}

var linuxPolicy = policy{
	required: []requirement{{memoryAvailablePercent, "Memory"}, {diskAvailablePercent, "Disk"}, {normalizedLoad, "Load"},
		{cpuPSISomeAvg60Percent, "CpuPsi"}, {memoryPSIFullAvg60Percent, "MemoryPsi"}, {ioPSIFullAvg60Percent, "IoPsi"}},
	unsupported: []Metric{memoryPressure},
}

// bounds are each classified float metric's inclusive warm and hot thresholds;
// a falling metric is worse when lower. cpu and swap are display-only.
var bounds = map[Metric]struct {
	warm, hot float64
	falling   bool
}{
	memoryAvailablePercent:    {15, 8, true},
	diskAvailablePercent:      {15, 5, true},
	normalizedLoad:            {1, 2, false},
	cpuPSISomeAvg60Percent:    {20, 50, false},
	memoryPSIFullAvg60Percent: {1, 5, false},
	ioPSIFullAvg60Percent:     {1, 5, false},
}

type Snapshot struct {
	Unsupported []Metric `json:"unsupported"` // sorted, constant per process
	Current     Sample   `json:"current"`
	History     []Point  `json:"history"` // chronological, ends with Current
}

type Sample struct {
	SampledAt time.Time `json:"sampledAt"` // UTC
	Level     Level     `json:"level"`
	Phase     Phase     `json:"phase"`
	Reasons   []Reason  `json:"reasons"` // never nil
	Signals   Signals   `json:"signals"`
	Missing   []Metric  `json:"missing"` // supported but unobserved, sorted; never nil
}

type Point struct {
	SampledAt time.Time `json:"sampledAt"`
	Level     Level     `json:"level"`
}

// Signals holds one sample's observed metrics; an unobserved metric has no entry.
type Signals map[Metric]Signal

// Signal is one observed metric. Value is a float64, except memoryPressure's
// "Normal", "Warning" or "Critical"; State is its classification.
type Signal struct {
	Value any   `json:"value"`
	State Level `json:"state"`
}

// record stores an observed float metric, classified against its bounds.
func (signals Signals) record(metric Metric, value float64) {
	state := informational
	if bound, classified := bounds[metric]; classified {
		hotter, warmer := value >= bound.hot, value >= bound.warm
		if bound.falling {
			hotter, warmer = value <= bound.hot, value <= bound.warm
		}
		switch {
		case hotter:
			state = hot
		case warmer:
			state = warm
		default:
			state = normal
		}
	}
	signals[metric] = Signal{Value: value, State: state}
}

// MarshalJSON writes the observed signals in wire order; encoding/json would sort map keys.
func (signals Signals) MarshalJSON() ([]byte, error) {
	members := []string{}
	for _, metric := range metrics {
		if signal, observed := signals[metric]; observed {
			member, err := json.Marshal(signal)
			if err != nil { // only a non-finite value
				return nil, err
			}
			members = append(members, `"`+string(metric)+`":`+string(member))
		}
	}
	return []byte("{" + strings.Join(members, ",") + "}"), nil
}

type Monitor struct {
	collector collector // NewMonitor, then only Run's goroutine
	evaluator evaluator // likewise
	policy    policy
	mutex     sync.RWMutex // guards current and history
	current   Sample
	history   []Point
}

func NewMonitor() *Monitor {
	monitor := &Monitor{policy: platformPolicy}
	monitor.sample(monitor.collector.collect(), time.Now())
	return monitor
}

// Run samples every five seconds, the freshness/cost trade-off: linux exposes
// these counters only as snapshots.
func (monitor *Monitor) Run(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case at := <-ticker.C:
			monitor.sample(monitor.collector.collect(), at)
		}
	}
}

func (monitor *Monitor) Snapshot() Snapshot {
	monitor.mutex.RLock()
	defer monitor.mutex.RUnlock()
	// a published Sample is never written again; the history array is, so it is copied
	return Snapshot{Unsupported: monitor.policy.unsupported, Current: monitor.current, History: slices.Clone(monitor.history)}
}

// sample classifies one collection against the policy and publishes it.
func (monitor *Monitor) sample(signals Signals, at time.Time) {
	level, reasons, unobserved := normal, []Reason{}, false
	for _, required := range monitor.policy.required {
		signal, observed := signals[required.metric]
		switch {
		case !observed:
			unobserved = true // reasons keep collecting
		case signal.State == warm || signal.State == hot:
			reasons = append(reasons, required.reason)
			if rank[signal.State] > rank[level] {
				level = signal.State
			}
		}
	}
	if unobserved {
		level = unknown
	}
	missing := []Metric{}
	for _, metric := range metrics {
		if _, observed := signals[metric]; !observed && !slices.Contains(monitor.policy.unsupported, metric) {
			missing = append(missing, metric)
		}
	}
	slices.Sort(missing)
	held, phase, heldReasons := monitor.evaluator.observe(level, reasons, at)
	current := Sample{SampledAt: at.UTC(), Level: held, Phase: phase, Reasons: heldReasons, Signals: signals, Missing: missing}

	monitor.mutex.Lock()
	defer monitor.mutex.Unlock()
	monitor.current = current
	// keep 15 minutes, at most 180 points: one per five-second sample
	cutoff := current.SampledAt.Add(-15 * time.Minute)
	first := 0
	for first < len(monitor.history) && monitor.history[first].SampledAt.Before(cutoff) {
		first++
	}
	monitor.history = append(monitor.history[first:], Point{SampledAt: current.SampledAt, Level: current.Level})
	if len(monitor.history) > 180 {
		monitor.history = monitor.history[len(monitor.history)-180:]
	}
}

// evaluator holds the reported level: it escalates at once and steps down one
// level per minute spent below the held level.
type evaluator struct {
	held          Level // "" until the first known sample
	heldReasons   []Reason
	steppingSince time.Time // zero when no step-down is armed
}

func (evaluator *evaluator) observe(level Level, reasons []Reason, at time.Time) (Level, Phase, []Reason) {
	switch {
	case level == unknown:
		evaluator.steppingSince = time.Time{}
		return level, steady, reasons
	case evaluator.held == "" || rank[level] >= rank[evaluator.held]:
		evaluator.held, evaluator.heldReasons, evaluator.steppingSince = level, reasons, time.Time{}
		return level, steady, reasons
	case evaluator.steppingSince.IsZero():
		evaluator.steppingSince = at
	case at.Sub(evaluator.steppingSince) >= time.Minute:
		evaluator.held, evaluator.steppingSince = below[evaluator.held], time.Time{}
		// a step onto the sampled level takes its reasons, which are none at Normal;
		// a step above it keeps the reasons it held
		if evaluator.held == level {
			evaluator.heldReasons = reasons
		}
	}
	phase := steady
	if rank[evaluator.held] > rank[level] {
		phase = recovering
	}
	return evaluator.held, phase, evaluator.heldReasons
}
