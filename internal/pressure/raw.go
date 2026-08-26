package pressure

type metric struct {
	value float64
	known bool
}

func knownMetric(value float64) metric { return metric{value: value, known: true} }

type MemoryPressure string

const (
	MemoryPressureNormal   MemoryPressure = "Normal"
	MemoryPressureWarning  MemoryPressure = "Warning"
	MemoryPressureCritical MemoryPressure = "Critical"
)

type memoryPressureMetric struct {
	value MemoryPressure
	known bool
}

func knownMemoryPressure(value MemoryPressure) memoryPressureMetric {
	return memoryPressureMetric{value: value, known: true}
}

type rawSample struct {
	cpuPercent             metric
	loadNormalized         metric
	memoryAvailablePercent metric
	swapUsedPercent        metric
	diskAvailablePercent   metric
	cpuPSISomeAvg60        metric
	memoryPSIFullAvg60     metric
	ioPSIFullAvg60         metric
	memoryPressure         memoryPressureMetric
}

type Metric string

const (
	MetricCPUPercent                   Metric = "cpuPercent"
	MetricLoadNormalized               Metric = "normalizedLoad"
	MetricMemoryAvailablePercent       Metric = "memoryAvailablePercent"
	MetricSwapUsedPercent              Metric = "swapUsedPercent"
	MetricDiskAvailablePercent         Metric = "diskAvailablePercent"
	MetricCPUPressureSomeAvg60         Metric = "cpuPsiSomeAvg60Percent"
	MetricMemoryPressureFullAvg60      Metric = "memoryPsiFullAvg60Percent"
	MetricInputOutputPressureFullAvg60 Metric = "ioPsiFullAvg60Percent"
	MetricMemoryPressure               Metric = "memoryPressure"
)

type policy struct {
	unsupported          []Metric
	nativeMemoryPressure bool
}

func (value policy) Unsupported() []Metric { return append([]Metric(nil), value.unsupported...) }
func linuxPolicy() policy                  { return policy{unsupported: []Metric{MetricMemoryPressure}} }
func darwinPolicy() policy {
	return policy{nativeMemoryPressure: true, unsupported: []Metric{MetricCPUPressureSomeAvg60, MetricInputOutputPressureFullAvg60, MetricMemoryAvailablePercent, MetricMemoryPressureFullAvg60}}
}
