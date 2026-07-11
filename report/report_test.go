package report

import (
	"encoding/json"
	"testing"
	"time"

	"http-tester/checker"
	"http-tester/checker/teststats"
)

func makeStats(results ...checker.RunResult) *teststats.Stats {
	s := &teststats.Stats{}
	for _, r := range results {
		s.AddRunResult(r)
	}
	return s
}

func TestPrintText(t *testing.T) {
	stats := makeStats(checker.RunResult{
		Domain: "example.com",
		Results: []*checker.Result{
			{Checker: "dns", Domain: "example.com", Passed: true, Details: "resolved"},
			{Checker: "http-ipv4", Domain: "example.com", Passed: false, Details: "failed"},
		},
	})

	exitCode := Print(stats, "8.8.8.8:53", time.Now(), "text")
	if exitCode != teststats.Fail {
		t.Errorf("exitCode = %d, want %d (has failures)", exitCode, teststats.Fail)
	}
}

func TestPrintJSON(t *testing.T) {
	stats := makeStats(checker.RunResult{
		Domain: "example.com",
		Results: []*checker.Result{
			{
				Checker: "dns", Domain: "example.com", Passed: true, Details: "resolved",
				Records: []checker.Record{{Type: "A", Value: "1.2.3.4"}},
			},
		},
	})

	exitCode := Print(stats, "8.8.8.8:53", time.Now(), "json")
	if exitCode != 0 {
		t.Errorf("exitCode = %d, want 0", exitCode)
	}

	exitCode = Print(stats, "8.8.8.8:53", time.Now(), "json-pretty")
	if exitCode != 0 {
		t.Errorf("exitCode = %d, want 0", exitCode)
	}
}

func TestPrintJSON_Structure(t *testing.T) {
	stats := makeStats(checker.RunResult{
		Domain: "example.com",
		Results: []*checker.Result{
			{
				Checker: "https-http2-ipv4", Domain: "example.com", Passed: true,
				Details: "200 OK", HTTPVersion: "HTTP/2", AltSvc: "h3=\":443\"",
				RedirectTo: "https://example.com/",
			},
			{Checker: "dns-https-info", Domain: "example.com", Passed: true, Info: true, Details: "info message"},
			{Checker: "http-alt-svc-warn", Domain: "example.com", Passed: false, Warning: true, Details: "warn message"},
		},
	})

	report := buildReport(stats, "8.8.8.8:53", time.Now())

	if report.Summary.Total != 1 {
		t.Errorf("Total = %d, want 1", report.Summary.Total)
	}
	if report.Summary.Passed != 1 {
		t.Errorf("Passed = %d, want 1", report.Summary.Passed)
	}
	if report.Summary.Failed != 0 {
		t.Errorf("Failed = %d, want 0", report.Summary.Failed)
	}
	if report.Summary.Warnings != 1 {
		t.Errorf("Warnings = %d, want 1", report.Summary.Warnings)
	}
	if report.Summary.Info != 1 {
		t.Errorf("Info = %d, want 1", report.Summary.Info)
	}

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var parsed JSONReport
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if len(parsed.Domains) != 1 {
		t.Errorf("Domains count = %d, want 1", len(parsed.Domains))
	}
	if len(parsed.Domains[0].Results) != 3 {
		t.Errorf("Results count = %d, want 3", len(parsed.Domains[0].Results))
	}
}

func TestPrintYAML(t *testing.T) {
	stats := makeStats(checker.RunResult{
		Domain: "example.com",
		Results: []*checker.Result{
			{Checker: "dns", Passed: true, Details: "resolved"},
		},
	})

	exitCode := Print(stats, "8.8.8.8:53", time.Now(), "yaml")
	if exitCode != 0 {
		t.Errorf("exitCode = %d, want 0", exitCode)
	}
}

func TestPrintText_WithError(t *testing.T) {
	stats := makeStats(checker.RunResult{
		Domain: "example.com",
		Results: []*checker.Result{
			{Checker: "dns", Domain: "example.com", Passed: false, Error: true, Details: "resolver unreachable"},
			{Checker: "dns-https", Domain: "example.com", Passed: false, Error: true, Details: "HTTPS lookup failed"},
		},
	})

	exitCode := Print(stats, "127.0.0.2:53", time.Now(), "text")
	if exitCode != teststats.Error {
		t.Errorf("exitCode = %d, want %d (errors)", exitCode, teststats.Error)
	}
}

func TestPrintJSON_WithError(t *testing.T) {
	stats := makeStats(checker.RunResult{
		Domain: "example.com",
		Results: []*checker.Result{
			{Checker: "dns", Domain: "example.com", Passed: false, Error: true, Details: "resolver unreachable"},
		},
	})

	report := buildReport(stats, "127.0.0.2:53", time.Now())

	if report.Summary.Total != 1 {
		t.Errorf("Total = %d, want 1", report.Summary.Total)
	}
	if report.Summary.Errors != 1 {
		t.Errorf("Errors = %d, want 1", report.Summary.Errors)
	}
	if report.Summary.Passed != 0 {
		t.Errorf("Passed = %d, want 0", report.Summary.Passed)
	}
	if report.Summary.Failed != 0 {
		t.Errorf("Failed = %d, want 0", report.Summary.Failed)
	}
}
