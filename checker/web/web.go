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
func RunAutoChecks(domain string, hasHTTPSCheck bool, httpsRecordExists bool, httpsCheckMode string, httpMode string, httpsMode string) []*checker.Result {
	var results []*checker.Result

	ipv4, ipv6 := resolveBoth(domain)

	// Check local IPv4/IPv6 connectivity
	localIPv4, localIPv6 := checkLocalConnectivity()

	// Add INFO messages if testing is not possible
	if ipv4 != "" && !localIPv4 {
		results = append(results, &checker.Result{
			Checker: "web-info",
			Domain:  domain,
			Passed:  true,
			Info:    true,
			Details: "IPv4 not available on this host — skipping IPv4 tests",
		})
		ipv4 = ""
	}
	if ipv6 != "" && !localIPv6 {
		results = append(results, &checker.Result{
			Checker: "web-info",
			Domain:  domain,
			Passed:  true,
			Info:    true,
			Details: "IPv6 not available on this host — skipping IPv6 tests",
		})
		ipv6 = ""
	}

	// Port 80 - HTTP/1.1 check (skip if mode is "no")
	if httpMode != "no" {
		if ipv4 != "" {
			results = append(results, checkPort(domain, "ipv4", ipv4, 80, "http", "http1", httpMode))
		}
		if ipv6 != "" {
			results = append(results, checkPort(domain, "ipv6", ipv6, 80, "http", "http1", httpMode))
		}
	}

	// Port 443 - HTTPS check (skip if mode is "no")
	var altSvc string
	if httpsMode != "no" {
		if ipv4 != "" {
			res := checkPort(domain, "ipv4", ipv4, 443, "https", "http2", httpsMode)
			results = append(results, res)
			if res.AltSvc != "" {
				altSvc = res.AltSvc
			}
		}
		if ipv6 != "" {
			res := checkPort(domain, "ipv6", ipv6, 443, "https", "http2", httpsMode)
			results = append(results, res)
			if res.AltSvc != "" && altSvc == "" {
				altSvc = res.AltSvc
			}
		}
	}

	// HTTP/3 check - only if Alt-Svc h3 detected and httpsMode != "no"
	if httpsMode != "no" && strings.Contains(altSvc, "h3") {
		if ipv4 != "" {
			results = append(results, checkHTTP3(domain, "ipv4", ipv4))
		}
		if ipv6 != "" {
			results = append(results, checkHTTP3(domain, "ipv6", ipv6))
		}
	}

	hasH3 := strings.Contains(altSvc, "h3")

	// Info: HTTP/3 supported, HTTPS check is optional (maybe), but record doesn't exist
	if httpsMode != "no" && hasH3 && httpsCheckMode == "maybe" && !httpsRecordExists {
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
		DialContext:            dialContext(port, ip),
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

func resolveBoth(domain string) (ipv4, ipv6 string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resolver := &net.Resolver{}
	addrs, err := resolver.LookupIPAddr(ctx, domain)
	if err != nil {
		return "", ""
	}

	for _, addr := range addrs {
		if addr.IP.To4() != nil && ipv4 == "" {
			ipv4 = addr.IP.String()
		} else if addr.IP.To4() == nil && ipv6 == "" {
			ipv6 = addr.IP.String()
		}
	}
	return ipv4, ipv6
}

// checkLocalConnectivity checks if the machine has IPv4 and IPv6 network interfaces
func checkLocalConnectivity() (hasIPv4, hasIPv6 bool) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return false, false
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ip, _, err := net.ParseCIDR(addr.String())
			if err != nil {
				continue
			}
			if ip.To4() != nil {
				hasIPv4 = true
			} else if ip.To16() != nil {
				hasIPv6 = true
			}
		}
	}

	return hasIPv4, hasIPv6
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
		if r.Passed && r.Body != nil && len(r.Body) > 0 {
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
			if similarity < 0.85 || sizeRatio > 1.5 {
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
