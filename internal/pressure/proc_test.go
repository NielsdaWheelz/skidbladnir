//go:build linux

package pressure

import "testing"

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
