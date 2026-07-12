package host

import (
	"http-tester/checker"
	"testing"
)

func TestCheckLocalConnectivity(t *testing.T) {
	stats := &checker.Stats{}
	hasIPv4, hasIPv6 := CheckLocalConnectivity(stats)

	// On any modern system, at least one should be true
	if !hasIPv4 && !hasIPv6 {
		t.Error("expected at least one of IPv4 or IPv6 to be available")
	}

	// Should have added 2 results to stats
	if len(stats.Results) != 1 {
		t.Fatalf("expected 1 RunResult, got %d", len(stats.Results))
	}
	if len(stats.Results[0].Results) != 2 {
		t.Fatalf("expected 2 Results, got %d", len(stats.Results[0].Results))
	}

	// Verify result structure
	for _, res := range stats.Results[0].Results {
		if res.Group != "Host" {
			t.Errorf("expected group 'Host', got %q", res.Group)
		}
		if res.Checker != "host-ipv4" && res.Checker != "host-ipv6" {
			t.Errorf("unexpected checker name: %q", res.Checker)
		}
		if res.Details == "" {
			t.Error("expected non-empty details")
		}
	}
}

func TestHasLocalIPv4(t *testing.T) {
	stats := &checker.Stats{}

	// Before any check, should return false
	if stats.HasLocalIPv4() {
		t.Error("expected false before check")
	}

	// Add a passing result
	stats.AddRunResult(checker.RunResult{
		Results: []*checker.Result{
			{Checker: "host-ipv4", Group: "Host", Passed: true},
		},
	})

	if !stats.HasLocalIPv4() {
		t.Error("expected true after adding passing result")
	}
}

func TestHasLocalIPv6(t *testing.T) {
	stats := &checker.Stats{}

	// Before any check, should return false
	if stats.HasLocalIPv6() {
		t.Error("expected false before check")
	}

	// Add a passing result
	stats.AddRunResult(checker.RunResult{
		Results: []*checker.Result{
			{Checker: "host-ipv6", Group: "Host", Passed: true},
		},
	})

	if !stats.HasLocalIPv6() {
		t.Error("expected true after adding passing result")
	}
}
