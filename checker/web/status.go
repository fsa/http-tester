package web

import (
	"fmt"
	"strings"

	"http-tester/checker"
)

// CheckStatusConsistency checks if all HTTP/HTTPS responses have consistent status types.
// HTTP and HTTPS are checked separately.
// For HTTP: all responses should have the same status (e.g., all 200 or all 301).
// For HTTPS: all responses should have the same status.
// Failed responses are ignored (already reported as FAIL).
func CheckStatusConsistency(results []*checker.Result) []*checker.Result {
	// Separate HTTP and HTTPS checks
	httpStatuses := make(map[string][]string) // status -> list of checkers
	httpsStatuses := make(map[string][]string)

	for _, r := range results {
		if !r.Passed {
			continue // skip failed responses
		}
		status := parseStatusType(r.Details)
		if status == "" {
			continue
		}

		checkerName := r.Checker
		if strings.HasPrefix(checkerName, "http-ipv") {
			// HTTP check (port 80): http-ipv4, http-ipv6
			httpStatuses[status] = append(httpStatuses[status], checkerName)
		} else if strings.HasPrefix(checkerName, "https-") {
			// HTTPS check (port 443): https-http2-ipv4, https-http2-ipv6, https-http3-ipv4, https-http3-ipv6
			httpsStatuses[status] = append(httpsStatuses[status], checkerName)
		}
	}

	var warnings []*checker.Result

	// Check HTTP consistency
	if len(httpStatuses) > 1 {
		var parts []string
		for status, checkers := range httpStatuses {
			parts = append(parts, fmt.Sprintf("%s (%s)", status, strings.Join(checkers, ", ")))
		}
		warnings = append(warnings, &checker.Result{
			Checker: "http-status-consistency",
			Group:   "Consistency",
			Domain:  "",
			Passed:  false,
			Warning: true,
			Details: fmt.Sprintf("inconsistent HTTP responses: %s",
				strings.Join(parts, " vs ")),
		})
	}

	// Check HTTPS consistency
	if len(httpsStatuses) > 1 {
		var parts []string
		for status, checkers := range httpsStatuses {
			parts = append(parts, fmt.Sprintf("%s (%s)", status, strings.Join(checkers, ", ")))
		}
		warnings = append(warnings, &checker.Result{
			Checker: "https-status-consistency",
			Group:   "Consistency",
			Domain:  "",
			Passed:  false,
			Warning: true,
			Details: fmt.Sprintf("inconsistent HTTPS responses: %s",
				strings.Join(parts, " vs ")),
		})
	}

	return warnings
}

// parseStatusType extracts a normalized status type from Details string
// e.g. "http-ipv4 http://example.com/ -> 200 OK" returns "200"
func parseStatusType(details string) string {
	// Find "-> XXX " pattern
	idx := strings.Index(details, "-> ")
	if idx < 0 {
		return ""
	}
	statusStr := details[idx+3:]
	// Extract status code (first 3 digits)
	if len(statusStr) < 3 {
		return ""
	}
	code := statusStr[:3]
	switch code {
	case "200":
		return "200"
	case "301":
		return "301"
	case "302":
		return "302"
	default:
		return code
	}
}
