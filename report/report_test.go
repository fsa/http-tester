package report

import (
	"encoding/json"
	"testing"

	"http-tester/checker"
)

func TestPrintText(t *testing.T) {
	results := []checker.RunResult{
		{
			Domain: "example.com",
			Results: []*checker.Result{
				{
					Checker: "dns",
					Domain:  "example.com",
					Passed:  true,
					Details: "resolved",
				},
				{
					Checker: "http-ipv4",
					Domain:  "example.com",
					Passed:  false,
					Details: "failed",
				},
			},
		},
	}

	exitCode := Print(results, "text")
	if exitCode != 1 {
		t.Errorf("exitCode = %d, want 1 (has failures)", exitCode)
	}
}

func TestPrintJSON(t *testing.T) {
	results := []checker.RunResult{
		{
			Domain: "example.com",
			Results: []*checker.Result{
				{
					Checker: "dns",
					Domain:  "example.com",
					Passed:  true,
					Details: "resolved",
					Records: []checker.Record{
						{Type: "A", Value: "1.2.3.4"},
					},
				},
			},
		},
	}

	// Test compact JSON
	exitCode := Print(results, "json")
	if exitCode != 0 {
		t.Errorf("exitCode = %d, want 0", exitCode)
	}

	// Test pretty JSON
	exitCode = Print(results, "json-pretty")
	if exitCode != 0 {
		t.Errorf("exitCode = %d, want 0", exitCode)
	}
}

func TestPrintJSON_Structure(t *testing.T) {
	results := []checker.RunResult{
		{
			Domain: "example.com",
			Results: []*checker.Result{
				{
					Checker:     "https-http2-ipv4",
					Domain:      "example.com",
					Passed:      true,
					Details:     "200 OK",
					HTTPVersion: "HTTP/2",
					AltSvc:      "h3=\":443\"",
					RedirectTo:  "https://example.com/",
				},
				{
					Checker: "dns-https-info",
					Domain:  "example.com",
					Passed:  true,
					Info:    true,
					Details: "info message",
				},
				{
					Checker: "http-alt-svc-warn",
					Domain:  "example.com",
					Passed:  false,
					Warning: true,
					Details: "warn message",
				},
			},
		},
	}

	report := buildReport(results)

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

	// Check JSON serialization
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
	results := []checker.RunResult{
		{
			Domain: "example.com",
			Results: []*checker.Result{
				{
					Checker: "dns",
					Passed:  true,
					Details: "resolved",
				},
			},
		},
	}

	exitCode := Print(results, "yaml")
	if exitCode != 0 {
		t.Errorf("exitCode = %d, want 0", exitCode)
	}
}
