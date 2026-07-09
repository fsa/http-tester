package web

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"http-tester/checker"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
)

// RunAutoChecks performs automatic HTTP checks:
// - Port 80: HTTP/1.1, expect redirect (301/302) or direct (200) based on mode
// - Port 443: HTTP/2, expect 200 (only if enableHTTPS is true)
// - HTTP/3: only if Alt-Svc h3 detected in HTTP/2 response (only if enableHTTPS is true)
// hasHTTPSCheck: dns.https config is set (yes/no/maybe)
// httpsRecordExists: HTTPS DNS record actually exists in DNS
// httpsCheckMode: "yes", "no", "maybe", or "" (not set)
// httpMode: "redirect" (default) or "direct" for port 80 behavior
// enableHTTPS: whether to check HTTPS on port 443
func RunAutoChecks(domain string, hasHTTPSCheck bool, httpsRecordExists bool, httpsCheckMode string, httpMode string, enableHTTPS bool) []*checker.Result {
	var results []*checker.Result

	ipv4, ipv6 := resolveBoth(domain)

	// Port 80 - HTTP/1.1 check
	expectRedirect := httpMode != "direct"
	if ipv4 != "" {
		results = append(results, checkHTTPPort80(domain, "ipv4", ipv4, expectRedirect))
	}
	if ipv6 != "" {
		results = append(results, checkHTTPPort80(domain, "ipv6", ipv6, expectRedirect))
	}

	// Port 443 - HTTPS HTTP/2 check (only if enabled)
	var altSvc string
	if enableHTTPS {
		if ipv4 != "" {
			res := checkHTTPSH2(domain, "ipv4", ipv4)
			results = append(results, res)
			if res.AltSvc != "" {
				altSvc = res.AltSvc
			}
		}
		if ipv6 != "" {
			res := checkHTTPSH2(domain, "ipv6", ipv6)
			results = append(results, res)
			if res.AltSvc != "" && altSvc == "" {
				altSvc = res.AltSvc
			}
		}
	}

	// HTTP/3 check - only if Alt-Svc h3 detected and HTTPS enabled
	if enableHTTPS && strings.Contains(altSvc, "h3") {
		if ipv4 != "" {
			results = append(results, checkHTTP3(domain, "ipv4", ipv4))
		}
		if ipv6 != "" {
			results = append(results, checkHTTP3(domain, "ipv6", ipv6))
		}
	}

	hasH3 := strings.Contains(altSvc, "h3")

	// Info: HTTP/3 supported, HTTPS check is optional (maybe), but record doesn't exist
	if enableHTTPS && hasH3 && httpsCheckMode == "maybe" && !httpsRecordExists {
		results = append(results, &checker.Result{
			Checker: "dns-https-info",
			Domain:  domain,
			Passed:  true,
			Info:    true,
			Details: "HTTP/3 supported but no HTTPS DNS record — consider adding https: yes",
		})
	}

	// Warn: HTTPS DNS record exists but server doesn't advertise Alt-Svc
	if enableHTTPS && httpsRecordExists && !hasH3 {
		results = append(results, &checker.Result{
			Checker: "http-alt-svc-warn",
			Domain:  domain,
			Passed:  false,
			Warning: true,
			Details: "HTTPS DNS record exists but server does not advertise Alt-Svc header",
		})
	}

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

func checkHTTPPort80(domain, ipVer, ip string, expectRedirect bool) *checker.Result {
	result := &checker.Result{
		Checker: fmt.Sprintf("http-%s", ipVer),
		Domain:  domain,
		Passed:  false,
	}

	u := &url.URL{
		Scheme: "http",
		Host:   domain,
		Path:   "/",
	}

	protocols := &http.Protocols{}
	protocols.SetHTTP1(true)
	protocols.SetHTTP2(false)

	transport := &http.Transport{
		DialContext: dialContext(80, ip),
		Protocols:  protocols,
		ResponseHeaderTimeout: 10 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
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

	if expectRedirect {
		result.Passed = resp.StatusCode == 301 || resp.StatusCode == 302
	} else {
		result.Passed = resp.StatusCode >= 200 && resp.StatusCode < 400
	}
	result.Details = fmt.Sprintf("%s %s -> %s", result.Checker, u.String(), resp.Status)
	result.HTTPVersion = resp.Proto
	if loc := resp.Header.Get("Location"); loc != "" {
		result.RedirectTo = loc
	}
	return result
}

func checkHTTPSH2(domain, ipVer, ip string) *checker.Result {
	result := &checker.Result{
		Checker: fmt.Sprintf("https-http2-%s", ipVer),
		Domain:  domain,
		Passed:  false,
	}

	u := &url.URL{
		Scheme: "https",
		Host:   domain,
		Path:   "/",
	}

	protocols := &http.Protocols{}
	protocols.SetHTTP1(false)
	protocols.SetHTTP2(true)

	transport := &http.Transport{
		DialContext: dialContext(443, ip),
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		Protocols:             protocols,
		ResponseHeaderTimeout: 10 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
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

	result.Passed = resp.StatusCode >= 200 && resp.StatusCode < 400
	result.Details = fmt.Sprintf("%s %s -> %s", result.Checker, u.String(), resp.Status)
	result.HTTPVersion = resp.Proto
	result.AltSvc = resp.Header.Get("Alt-Svc")
	if loc := resp.Header.Get("Location"); loc != "" {
		result.RedirectTo = loc
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
