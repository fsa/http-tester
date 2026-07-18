package web

import (
	"testing"

	"http-tester/checker"
)

func TestParseStatusType(t *testing.T) {
	tests := []struct {
		name     string
		details  string
		expected string
	}{
		{
			name:     "200 OK",
			details:  "http://example.com/ -> 200 OK",
			expected: "200",
		},
		{
			name:     "301 redirect",
			details:  "http://example.com/ -> 301 Moved Permanently",
			expected: "301",
		},
		{
			name:     "302 redirect",
			details:  "http://example.com/ -> 302 Found",
			expected: "302",
		},
		{
			name:     "404 not found",
			details:  "http://example.com/ -> 404 Not Found",
			expected: "404",
		},
		{
			name:     "no arrow",
			details:  "some error message",
			expected: "",
		},
		{
			name:     "short status",
			details:  "http://example.com/ -> 2",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseStatusType(tt.details)
			if result != tt.expected {
				t.Errorf("parseStatusType(%q) = %q, want %q", tt.details, result, tt.expected)
			}
		})
	}
}

func TestCheckStatusConsistency(t *testing.T) {
	tests := []struct {
		name            string
		results         []*checker.Result
		expectedWarning bool
	}{
		{
			name:            "no results",
			results:         []*checker.Result{},
			expectedWarning: false,
		},
		{
			name: "all same HTTP status",
			results: []*checker.Result{
				{Checker: "http-ipv4", Passed: true, Details: "http://example.com/ -> 200 OK"},
				{Checker: "http-ipv6", Passed: true, Details: "http://example.com/ -> 200 OK"},
			},
			expectedWarning: false,
		},
		{
			name: "inconsistent HTTP status",
			results: []*checker.Result{
				{Checker: "http-ipv4", Passed: true, Details: "http://example.com/ -> 200 OK"},
				{Checker: "http-ipv6", Passed: true, Details: "http://example.com/ -> 301 Moved Permanently"},
			},
			expectedWarning: true,
		},
		{
			name: "all same HTTPS status",
			results: []*checker.Result{
				{Checker: "https-http2-ipv4", Passed: true, Details: "https://example.com/ -> 200 OK"},
				{Checker: "https-http2-ipv6", Passed: true, Details: "https://example.com/ -> 200 OK"},
			},
			expectedWarning: false,
		},
		{
			name: "inconsistent HTTPS status",
			results: []*checker.Result{
				{Checker: "https-http2-ipv4", Passed: true, Details: "https://example.com/ -> 200 OK"},
				{Checker: "https-http3-ipv4", Passed: true, Details: "https://example.com/ -> 301 Moved Permanently"},
			},
			expectedWarning: true,
		},
		{
			name: "failed results ignored",
			results: []*checker.Result{
				{Checker: "http-ipv4", Passed: false, Details: "request failed"},
				{Checker: "http-ipv6", Passed: true, Details: "http://example.com/ -> 200 OK"},
			},
			expectedWarning: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warnings := CheckStatusConsistency(tt.results)
			hasWarning := len(warnings) > 0
			if hasWarning != tt.expectedWarning {
				t.Errorf("CheckStatusConsistency() returned %d warnings, expected warning=%v", len(warnings), tt.expectedWarning)
			}
		})
	}
}
