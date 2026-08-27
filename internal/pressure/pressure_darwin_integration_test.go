//go:build integration && darwin && cgo

package pressure

import "testing"

func TestDarwinCollectorReportsRequiredNativeSignals(t *testing.T) {
	collector := newCollector()
	sample := collector.collect()
	if !sample.diskAvailablePercent.known || !sample.loadNormalized.known || !sample.memoryPressure.known {
		t.Fatalf("required Darwin signals missing: disk=%t load=%t memoryPressure=%t", sample.diskAvailablePercent.known, sample.loadNormalized.known, sample.memoryPressure.known)
	}
}

func TestDarwinCollectorDoesNotExhaustHostPortRights(t *testing.T) {
	before, ok := hostPortSendRightReferences()
	if !ok {
		t.Fatal("read initial host send-right references")
	}
	collector := newCollector()
	for sample := 0; sample < 128; sample++ {
		collector.collect()
	}
	after, ok := hostPortSendRightReferences()
	if !ok {
		t.Fatal("read final host send-right references")
	}
	if after > before+1 {
		t.Fatalf("host send-right references grew: before=%d after=%d", before, after)
	}
}
