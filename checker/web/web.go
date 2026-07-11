package web

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"http-tester/checker"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
)

const maxBodySize = 512 * 1024 // 512 KB max body size for consistency check

// RunAutoChecks performs automatic web checks:
// - Port 80: HTTP/1.1 with httpMode
// - Port 443: HTTP/2 with httpsMode (only if httpsMode != "no")
// - HTTP/3: only if Alt-Svc h3 detected and httpsMode != "no"
// When testAllIPs is true, checks are run for every IP; otherwise only the first of each type.
func RunAutoChecks(domain string, ipv4s, ipv6s []string, testAllIPs bool, hasHTTPSCheck bool, httpsRecordExists bool, httpsCheckMode string, httpMode string, httpsMode string, localIPv4, localIPv6 bool) []*checker.Result {
	var results []*checker.Result

	// Check local connectivity and add INFO to this domain's results
	if len(ipv4s) > 0 && !localIPv4 {
		results = append(results, &checker.Result{
			Checker: "web-info",
			Domain:  domain,
			Passed:  true,
			Info:    true,
			Details: "IPv4 not available on this host — skipping IPv4 tests",
		})
		ipv4s = nil
	}
	if len(ipv6s) > 0 && !localIPv6 {
		results = append(results, &checker.Result{
			Checker: "web-info",
			Domain:  domain,
			Passed:  true,
			Info:    true,
			Details: "IPv6 not available on this host — skipping IPv6 tests",
		})
		ipv6s = nil
	}

	// Select IPs to test
	var testIPv4s, testIPv6s []string
	if testAllIPs {
		testIPv4s = ipv4s
		testIPv6s = ipv6s
	} else {
		if len(ipv4s) > 0 {
			testIPv4s = []string{ipv4s[0]}
		}
		if len(ipv6s) > 0 {
			testIPv6s = []string{ipv6s[0]}
		}
	}

	// Port 80 - HTTP/1.1 check (skip if mode is "no")
	if httpMode != "no" {
		for _, ip := range testIPv4s {
			results = append(results, checkPort(domain, ipLabel("ipv4", ip, testAllIPs, len(testIPv4s)), ip, 80, "http", "http1", httpMode))
		}
		for _, ip := range testIPv6s {
			results = append(results, checkPort(domain, ipLabel("ipv6", ip, testAllIPs, len(testIPv6s)), ip, 80, "http", "http1", httpMode))
		}
	}

	// Port 443 - HTTPS check (skip if mode is "no")
	var altSvc string
	if httpsMode != "no" {
		for _, ip := range testIPv4s {
			res := checkPort(domain, ipLabel("ipv4", ip, testAllIPs, len(testIPv4s)), ip, 443, "https", "http2", httpsMode)
			results = append(results, res)
			if res.AltSvc != "" {
				altSvc = res.AltSvc
			}
		}
		for _, ip := range testIPv6s {
			res := checkPort(domain, ipLabel("ipv6", ip, testAllIPs, len(testIPv6s)), ip, 443, "https", "http2", httpsMode)
			results = append(results, res)
			if res.AltSvc != "" && altSvc == "" {
				altSvc = res.AltSvc
			}
		}
	}

	// HTTP/3 check - only if Alt-Svc h3 detected and httpsMode != "no"
	if httpsMode != "no" && strings.Contains(altSvc, "h3") {
		for _, ip := range testIPv4s {
			results = append(results, checkHTTP3(domain, ipLabel("ipv4", ip, testAllIPs, len(testIPv4s)), ip))
		}
		for _, ip := range testIPv6s {
			results = append(results, checkHTTP3(domain, ipLabel("ipv6", ip, testAllIPs, len(testIPv6s)), ip))
		}
	}

	hasH3 := strings.Contains(altSvc, "h3")

	// Info: HTTP/3 supported, HTTPS check is optional (optional), but record doesn't exist
	if httpsMode != "no" && hasH3 && httpsCheckMode == "optional" && !httpsRecordExists {
		results = append(results, &checker.Result{
			Checker: "dns-https-info",
			Domain:  domain,
			Passed:  true,
			Info:    true,
			Details: "HTTP/3 supported but no HTTPS DNS record — create HTTPS record for better performance",
		})
	}

	// Warn: HTTPS DNS record exists but server doesn't advertise Alt-Svc
	if httpsMode != "no" && httpsRecordExists && !hasH3 {
		results = append(results, &checker.Result{
			Checker: "http-alt-svc-warn",
			Domain:  domain,
			Passed:  false,
			Warning: true,
			Details: "HTTPS DNS record exists but server does not advertise Alt-Svc header",
		})
	}

	// Check consistency of responses across different protocols
	consistencyWarnings := CheckConsistency(results)
	results = append(results, consistencyWarnings...)

	// Check status code consistency across protocols
	statusWarnings := CheckStatusConsistency(results)
	results = append(results, statusWarnings...)

	if len(results) == 0 {
		results = append(results, &checker.Result{
			Checker: "http",
			Domain:  domain,
			Passed:  false,
			Details: "no reachable IP found",
		})
	}

	return results
}

// checkPort performs a single HTTP/HTTPS check with mode validation
func checkPort(domain, ipVer, ip string, port int, scheme, protocol, mode string) *checker.Result {
	checkerName := fmt.Sprintf("%s-%s-%s", scheme, protocol, ipVer)
	if scheme == "http" {
		checkerName = fmt.Sprintf("%s-%s", scheme, ipVer)
	}

	result := &checker.Result{
		Checker: checkerName,
		Domain:  domain,
		Passed:  false,
	}

	u := &url.URL{
		Scheme: scheme,
		Host:   domain,
		Path:   "/",
	}

	protocols := &http.Protocols{}
	if protocol == "http1" {
		protocols.SetHTTP1(true)
		protocols.SetHTTP2(false)
	} else {
		protocols.SetHTTP1(false)
		protocols.SetHTTP2(true)
	}

	transport := &http.Transport{
		DialContext:           dialContext(port, ip),
		Protocols:             protocols,
		ResponseHeaderTimeout: 10 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
	}

	if scheme == "https" {
		transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client := &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		result.Details = fmt.Sprintf("request error: %v", err)
		return result
	}

	resp, err := client.Do(req)
	if err != nil {
		result.Details = fmt.Sprintf("request failed: %v", err)
		return result
	}
	defer resp.Body.Close()

	isRedirect := resp.StatusCode == 301 || resp.StatusCode == 302
	isOK := resp.StatusCode >= 200 && resp.StatusCode < 400

	switch mode {
	case "redirect":
		result.Passed = isRedirect
	case "direct":
		result.Passed = isOK
	case "any":
		result.Passed = isOK
	default:
		result.Passed = isOK
	}

	result.Details = fmt.Sprintf("%s %s -> %s", result.Checker, u.String(), resp.Status)
	result.HTTPVersion = resp.Proto
	if loc := resp.Header.Get("Location"); loc != "" {
		result.RedirectTo = loc
	}
	if altSvc := resp.Header.Get("Alt-Svc"); altSvc != "" {
		result.AltSvc = altSvc
	}
	// Store body for consistency check (only for 200 OK)
	if resp.StatusCode == 200 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxBodySize))
		result.Body = body
	}
	return result
}

func checkHTTP3(domain, ipVer, ip string) *checker.Result {
	result := &checker.Result{
		Checker: fmt.Sprintf("https-http3-%s", ipVer),
		Domain:  domain,
		Passed:  false,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	dialAddr := net.JoinHostPort(ip, "443")

	transport := &http3.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			NextProtos: []string{"h3"},
		},
		Dial: func(ctx context.Context, addr string, tlsCfg *tls.Config, cfg *quic.Config) (*quic.Conn, error) {
			return quic.DialAddr(ctx, dialAddr, tlsCfg, cfg)
		},
	}
	defer transport.CloseIdleConnections()

	client := &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	u := &url.URL{
		Scheme: "https",
		Host:   domain,
		Path:   "/",
	}

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		result.Details = fmt.Sprintf("request error: %v", err)
		return result
	}

	resp, err := client.Do(req)
	if err != nil {
		result.Details = fmt.Sprintf("HTTP/3 request failed: %v", err)
		return result
	}
	defer resp.Body.Close()

	result.Passed = resp.StatusCode >= 200 && resp.StatusCode < 400
	result.Details = fmt.Sprintf("%s %s -> %s", result.Checker, u.String(), resp.Status)
	result.HTTPVersion = resp.Proto
	if loc := resp.Header.Get("Location"); loc != "" {
		result.RedirectTo = loc
	}
	if altSvc := resp.Header.Get("Alt-Svc"); altSvc != "" {
		result.AltSvc = altSvc
	}
	// Store body for consistency check (only for 200 OK)
	if resp.StatusCode == 200 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxBodySize))
		result.Body = body
	}
	return result
}

func dialContext(port int, ip string) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		resolvedAddr := net.JoinHostPort(ip, fmt.Sprintf("%d", port))
		dialer := &net.Dialer{Timeout: 10 * time.Second}
		return dialer.DialContext(ctx, "tcp", resolvedAddr)
	}
}

// ipLabel returns "ipv4" or "ipv4:1.2.3.4" when multiple IPs are tested.
func ipLabel(base, ip string, testAllIPs bool, count int) string {
	if testAllIPs && count > 1 {
		return base + ":" + ip
	}
	return base
}

// wordFreq builds a frequency map of words from input bytes
func wordFreq(data []byte) map[string]int {
	freq := make(map[string]int)
	words := strings.FieldsFunc(string(data), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	for _, w := range words {
		w = strings.ToLower(w)
		if len(w) >= 3 {
			freq[w]++
		}
	}
	return freq
}

// compareWordMaps compares two word frequency maps and returns the ratio of matching words
func compareWordMaps(a, b map[string]int) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0
	}
	// Union of all unique words
	union := make(map[string]bool)
	for w := range a {
		union[w] = true
	}
	for w := range b {
		union[w] = true
	}
	// Count words present in both maps
	match := 0
	for w := range union {
		if a[w] > 0 && b[w] > 0 {
			match++
		}
	}
	return float64(match) / float64(len(union))
}

// CheckConsistency compares response bodies from different protocols and warns on significant differences
func CheckConsistency(results []*checker.Result) []*checker.Result {
	// Collect results with 200 OK status and body
	type entry struct {
		checker string
		body    []byte
	}
	var entries []entry
	for _, r := range results {
		if r.Passed && r.Body != nil && len(r.Body) > 1024 { // skip small responses (under 1KB)
			entries = append(entries, entry{checker: r.Checker, body: r.Body})
		}
	}

	if len(entries) < 2 {
		return nil
	}

	// Compare all pairs
	var warnings []*checker.Result
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			freqA := wordFreq(entries[i].body)
			freqB := wordFreq(entries[j].body)
			similarity := compareWordMaps(freqA, freqB)

			// Check size ratio
			sizeA := len(entries[i].body)
			sizeB := len(entries[j].body)
			sizeRatio := 1.0
			if sizeA > 0 && sizeB > 0 {
				if sizeA > sizeB {
					sizeRatio = float64(sizeA) / float64(sizeB)
				} else {
					sizeRatio = float64(sizeB) / float64(sizeA)
				}
			}

			// Warn if similarity is low or size differs significantly
			// Thresholds are intentionally relaxed to avoid false positives
			// from sites with A/B testing, localization, or minor content variations
			if similarity < 0.60 || sizeRatio > 3.0 {
				warnings = append(warnings, &checker.Result{
					Checker: "consistency",
					Domain:  entries[i].checker,
					Passed:  false,
					Warning: true,
					Details: fmt.Sprintf("different content detected between %s and %s (word overlap: %.0f%%, size ratio: %.1fx)",
						entries[i].checker, entries[j].checker, similarity*100, sizeRatio),
				})
			}
		}
	}

	return warnings
}

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
	statusStr := details[idx+4:]
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
