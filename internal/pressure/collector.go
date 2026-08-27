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

func newCollector() collector { return collector{} }

func (collector *collector) cpuPercent(current cpuCounters) metric {
	usage := metric{}
	if collector.hasPreviousCPU {
		usage = cpuUsage(collector.previousCPU, current)
	}
	collector.previousCPU, collector.hasPreviousCPU = current, true
	return usage
}

func cpuUsage(previous, current cpuCounters) metric {
	if current.total <= previous.total || current.idle < previous.idle {
		return metric{}
	}
	total, idle := current.total-previous.total, current.idle-previous.idle
	if idle > total {
		return metric{}
	}
	return knownMetric(float64(total-idle) * 100 / float64(total))
}
