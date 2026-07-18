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

	"http-tester/checker"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
)

// RunAutoChecks performs automatic web checks:
// - Port 80: HTTP/1.1 with httpMode
// - Port 443: HTTP/2 with httpsMode (only if httpsMode != "no")
// - HTTP/3: only if Alt-Svc h3 detected and httpsMode != "no"
// When testAllIPs is true, checks are run for every IP; otherwise only the first of each type.
func RunAutoChecks(domain string, ipv4s, ipv6s []string, testAllIPs bool, hasHTTPSCheck bool, httpsRecordExists bool, httpsCheckMode string, httpMode string, httpsMode string, stats *checker.Stats) {
	var results []*checker.Result

	// Filter IPs based on local connectivity from host check
	if len(ipv4s) > 0 && !stats.HasLocalIPv4() {
		if httpMode != "no" {
			results = append(results, &checker.Result{
				Checker: "web-info",
				Group:   "HTTP",
				Domain:  domain,
				Passed:  true,
				Info:    true,
				Details: "IPv4 not available on this host — skipping IPv4 tests",
			})
		}
		if httpsMode != "no" {
			results = append(results, &checker.Result{
				Checker: "web-info",
				Group:   "HTTPS",
				Domain:  domain,
				Passed:  true,
				Info:    true,
				Details: "IPv4 not available on this host — skipping IPv4 tests",
			})
		}
		ipv4s = nil
	}
	if len(ipv6s) > 0 && !stats.HasLocalIPv6() {
		if httpMode != "no" {
			results = append(results, &checker.Result{
				Checker: "web-info",
				Group:   "HTTP",
				Domain:  domain,
				Passed:  true,
				Info:    true,
				Details: "IPv6 not available on this host — skipping IPv6 tests",
			})
		}
		if httpsMode != "no" {
			results = append(results, &checker.Result{
				Checker: "web-info",
				Group:   "HTTPS",
				Domain:  domain,
				Passed:  true,
				Info:    true,
				Details: "IPv6 not available on this host — skipping IPv6 tests",
			})
		}
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
			Group:   "DNS",
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
			Group:   "DNS",
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
			Group:   "HTTP",
			Domain:  domain,
			Passed:  false,
			Details: "no reachable IP found",
		})
	}

	stats.AddRunResult(checker.RunResult{Domain: domain, Results: results})
}

// checkPort performs a single HTTP/HTTPS check with mode validation
func checkPort(domain, ipVer, ip string, port int, scheme, protocol, mode string) *checker.Result {
	checkerName := fmt.Sprintf("%s-%s-%s", scheme, protocol, ipVer)
	if scheme == "http" {
		checkerName = fmt.Sprintf("%s-%s", scheme, ipVer)
	}

	group := "HTTP"
	if scheme == "https" {
		group = "HTTPS"
	}

	ipTag := "ipv4"
	if strings.Contains(ipVer, "ipv6") {
		ipTag = "ipv6"
	}
	tags := []string{ipTag}
	if scheme == "https" {
		tags = []string{protocol, ipTag}
	}

	result := &checker.Result{
		Checker: checkerName,
		Group:   group,
		Tags:    tags,
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
		ResponseHeaderTimeout: checker.DefaultResponseTimeout,
		TLSHandshakeTimeout:   checker.DefaultTLSHandshake,
	}

	if scheme == "https" {
		transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}

	ctx, cancel := context.WithTimeout(context.Background(), checker.DefaultRequestTimeout)
	defer cancel()

	client := &http.Client{
		Transport: transport,
		Timeout:   checker.DefaultRequestTimeout,
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

	result.Details = fmt.Sprintf("%s -> %s", u.String(), resp.Status)
	result.ServerAddr = ip
	result.HTTPVersion = resp.Proto
	if loc := resp.Header.Get("Location"); loc != "" {
		result.RedirectTo = loc
	}
	if altSvc := resp.Header.Get("Alt-Svc"); altSvc != "" {
		result.AltSvc = altSvc
	}
	// Store body for consistency check (only for 200 OK)
	if resp.StatusCode == 200 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, checker.MaxBodySizeForCheck))
		result.Body = body
	}
	return result
}

func checkHTTP3(domain, ipVer, ip string) *checker.Result {
	result := &checker.Result{
		Checker: fmt.Sprintf("https-http3-%s", ipVer),
		Group:   "HTTPS",
		Tags:    []string{"http3", ipVer},
		Domain:  domain,
		Passed:  false,
	}

	ctx, cancel := context.WithTimeout(context.Background(), checker.DefaultRequestTimeout)
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
		Timeout:   checker.DefaultRequestTimeout,
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
	result.Details = fmt.Sprintf("%s -> %s", u.String(), resp.Status)
	result.ServerAddr = ip
	result.HTTPVersion = resp.Proto
	if loc := resp.Header.Get("Location"); loc != "" {
		result.RedirectTo = loc
	}
	if altSvc := resp.Header.Get("Alt-Svc"); altSvc != "" {
		result.AltSvc = altSvc
	}
	// Store body for consistency check (only for 200 OK)
	if resp.StatusCode == 200 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, checker.MaxBodySizeForCheck))
		result.Body = body
	}
	return result
}

func dialContext(port int, ip string) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		resolvedAddr := net.JoinHostPort(ip, fmt.Sprintf("%d", port))
		dialer := &net.Dialer{Timeout: checker.DefaultDialTimeout}
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
