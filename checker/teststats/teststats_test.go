package teststats

import (
	"http-tester/checker"
	"testing"
)

func makeStats(results ...checker.RunResult) *Stats {
	s := &Stats{}
	for _, r := range results {
		s.AddRunResult(r)
	}
	return s
}

func TestEmpty(t *testing.T) {
	s := &Stats{}
	if s.Total != 0 || s.Passed != 0 || s.Failed != 0 || s.Errors != 0 || s.Warnings != 0 || s.Info != 0 {
		t.Errorf("expected empty stats, got %+v", s)
	}
	if s.Code() != OK {
		t.Errorf("expected OK code, got %d", s.Code())
	}
}

func TestPass(t *testing.T) {
	s := makeStats(checker.RunResult{
		Domain:  "example.com",
		Results: []*checker.Result{{Checker: "http", Passed: true}},
	})
	if s.Total != 1 || s.Passed != 1 || s.Failed != 0 {
		t.Errorf("unexpected stats: %+v", s)
	}
	if s.Code() != OK {
		t.Errorf("expected OK, got %d", s.Code())
	}
}

func TestFail(t *testing.T) {
	s := makeStats(checker.RunResult{
		Domain:  "example.com",
		Results: []*checker.Result{{Checker: "http", Passed: false}},
	})
	if s.Total != 1 || s.Failed != 1 {
		t.Errorf("unexpected stats: %+v", s)
	}
	if s.Code() != Fail {
		t.Errorf("expected Fail(%d), got %d", Fail, s.Code())
	}
}

func TestWarn(t *testing.T) {
	s := makeStats(checker.RunResult{
		Domain:  "example.com",
		Results: []*checker.Result{{Checker: "http", Passed: true, Warning: true}},
	})
	if s.Warnings != 1 || s.Total != 0 {
		t.Errorf("unexpected stats: %+v", s)
	}
	if s.Code() != Warn {
		t.Errorf("expected Warn(%d), got %d", Warn, s.Code())
	}
}

func TestError(t *testing.T) {
	s := makeStats(checker.RunResult{
		Domain:  "example.com",
		Results: []*checker.Result{{Checker: "dns", Error: true}},
	})
	if s.Total != 1 || s.Errors != 1 {
		t.Errorf("unexpected stats: %+v", s)
	}
	if s.Code() != Error {
		t.Errorf("expected Error(%d), got %d", Error, s.Code())
	}
}

func TestInfo(t *testing.T) {
	s := makeStats(checker.RunResult{
		Domain: "example.com",
		Results: []*checker.Result{
			{Checker: "web-info", Info: true},
			{Checker: "http", Passed: true},
		},
	})
	if s.Info != 1 || s.Total != 1 || s.Passed != 1 {
		t.Errorf("unexpected stats: %+v", s)
	}
	if s.Code() != OK {
		t.Errorf("expected OK, got %d", s.Code())
	}
}

func TestCombination(t *testing.T) {
	s := makeStats(checker.RunResult{
		Domain: "example.com",
		Results: []*checker.Result{
			{Checker: "http", Passed: false},
			{Checker: "https", Passed: true, Warning: true},
			{Checker: "web-info", Info: true},
		},
	})
	if s.Total != 1 || s.Failed != 1 || s.Warnings != 1 || s.Info != 1 {
		t.Errorf("unexpected stats: %+v", s)
	}
	expected := Fail | Warn
	if s.Code() != expected {
		t.Errorf("expected %d, got %d", expected, s.Code())
	}
}

func TestMultipleDomains(t *testing.T) {
	s := makeStats(
		checker.RunResult{
			Domain:  "example.com",
			Results: []*checker.Result{{Checker: "http", Passed: true}},
		},
		checker.RunResult{
			Domain:  "example.org",
			Results: []*checker.Result{{Checker: "http", Passed: false}},
		},
	)
	if s.Total != 2 || s.Passed != 1 || s.Failed != 1 {
		t.Errorf("unexpected stats: %+v", s)
	}
	if len(s.Results) != 2 {
		t.Errorf("Results count = %d, want 2", len(s.Results))
	}
}
