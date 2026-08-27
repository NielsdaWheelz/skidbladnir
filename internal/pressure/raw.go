package pressure

import "slices"

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

// policy is the single owner of a platform's pressure capability: which
// metrics classify overall status and which the platform never observes.
// newPolicy establishes the invariants once, at construction.
type policy struct {
	required    []Metric
	unsupported []Metric
}

func (value policy) Unsupported() []Metric { return append([]Metric(nil), value.unsupported...) }

func newPolicy(required, unsupported []Metric) policy {
	sorted := append([]Metric(nil), unsupported...)
	slices.Sort(sorted)
	for index := 1; index < len(sorted); index++ {
		if sorted[index-1] == sorted[index] {
			panic("duplicate unsupported pressure metric") // justify-defect: a platform policy literal is wrong; the canonical unsupported set is unique by contract.
		}
	}
	for _, metric := range required {
		if _, classifiable := requiredClassifiers[metric]; !classifiable {
			panic("required pressure metric has no classifier") // justify-defect: a platform policy literal names a metric outside the closed classification table.
		}
		if slices.Contains(sorted, metric) {
			panic("required pressure metric is unsupported") // justify-defect: one metric cannot be both a classification input and unsupported; that would make Unknown permanent.
		}
	}
	return policy{required: append([]Metric(nil), required...), unsupported: sorted}
}

func linuxPolicy() policy {
	return newPolicy(
		[]Metric{MetricMemoryAvailablePercent, MetricDiskAvailablePercent, MetricLoadNormalized, MetricCPUPressureSomeAvg60, MetricMemoryPressureFullAvg60, MetricInputOutputPressureFullAvg60},
		[]Metric{MetricMemoryPressure},
	)
}

func darwinPolicy() policy {
	return newPolicy(
		[]Metric{MetricDiskAvailablePercent, MetricLoadNormalized, MetricMemoryPressure},
		[]Metric{MetricCPUPressureSomeAvg60, MetricInputOutputPressureFullAvg60, MetricMemoryAvailablePercent, MetricMemoryPressureFullAvg60},
	)
}
