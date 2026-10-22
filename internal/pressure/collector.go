package pressure

type cpuCounters struct {
	total uint64
	idle  uint64
}

// collector holds the previous-sample CPU tick state shared by both platform
// collectors; only the raw tick acquisition is platform-specific.
type collector struct {
	previousCPU    cpuCounters
	hasPreviousCPU bool
}

// observeCPU records usage since the previous counters; the first sample and a
// counter reset leave cpu unobserved.
func (collector *collector) observeCPU(signals Signals, current cpuCounters) {
	previous, hadPrevious := collector.previousCPU, collector.hasPreviousCPU
	collector.previousCPU, collector.hasPreviousCPU = current, true
	if !hadPrevious || current.total <= previous.total || current.idle < previous.idle {
		return
	}
	total, idle := current.total-previous.total, current.idle-previous.idle
	if idle > total {
		return
	}
	signals.record(cpuPercent, float64(total-idle)*100/float64(total))
}
