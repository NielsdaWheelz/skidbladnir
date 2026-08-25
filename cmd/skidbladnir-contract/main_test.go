package main

import "testing"

func TestAcceptanceTargetClosure(t *testing.T) {
	for _, target := range []string{"p0", "core", "upgrade"} {
		if !acceptanceTargets[target] {
			t.Fatalf("acceptance target %q is missing", target)
		}
	}
	for _, target := range []string{"", "all", "release", "P0"} {
		if acceptanceTargets[target] {
			t.Fatalf("unexpected acceptance target %q", target)
		}
	}
}
