//go:build darwin

package pressure

import (
	"slices"
	"testing"
)

func TestDarwinPressureUsesNativeMemoryPressureAndNotLinuxMetrics(t *testing.T) {
	policy := darwinPolicy()
	sample := rawSample{diskAvailablePercent: knownMetric(50), loadNormalized: knownMetric(0.5), memoryPressure: knownMemoryPressure(MemoryPressureWarning)}
	if got := classifyOverall(sample, policy); got.status != StatusWarm || !slices.Equal(got.reasons, []Reason{ReasonMemory}) {
		t.Fatalf("Darwin warning classification = %+v", got)
	}
	sample.memoryPressure = knownMemoryPressure(MemoryPressureCritical)
	if got := classifyOverall(sample, policy); got.status != StatusHot {
		t.Fatalf("Darwin critical classification = %+v", got)
	}
	if got := policy.Unsupported(); !slices.Equal(got, []Metric{MetricCPUPressureSomeAvg60, MetricInputOutputPressureFullAvg60, MetricMemoryAvailablePercent, MetricMemoryPressureFullAvg60}) {
		t.Fatalf("Darwin unsupported = %v", got)
	}
}
