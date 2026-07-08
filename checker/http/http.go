package http

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

type HTTPChecker struct {
	Protocol string // http1, http2, http3
	Port     int    // 80, 443
	Status   []int  // expected status codes
}

func (c *HTTPChecker) Name() string {
	scheme := "https"
	if c.Port == 80 {
		scheme = "http"
	}
	return fmt.Sprintf("%s-%s", scheme, c.Protocol)
}

func (c *HTTPChecker) Check(domain string) ([]*checker.Result, error) {
	scheme := "https"
	port := c.Port
	if port == 0 {
		port = 443
	}
	if port == 80 {
		scheme = "http"
	}

	u := &url.URL{
		Scheme: scheme,
		Host:   domain,
		Path:   "/",
	}

	// Resolve both IPv4 and IPv6
	ipv4, ipv6 := c.resolveBoth(domain)

	var results []*checker.Result

	if ipv4 != "" {
		res := c.checkSingle(domain, u, port, "ipv4", ipv4)
		results = append(results, res)
	}

	if ipv6 != "" {
		res := c.checkSingle(domain, u, port, "ipv6", ipv6)
		results = append(results, res)
	}

	if len(results) == 0 {
		results = append(results, &checker.Result{
			Checker: c.Name(),
			Domain:  domain,
			Passed:  false,
			Details: "no reachable IP found",
		})
	}

	return results, nil
}

func (c *HTTPChecker) checkSingle(domain string, u *url.URL, port int, ipVer, ip string) *checker.Result {
	result := &checker.Result{
		Checker: fmt.Sprintf("%s-%s", c.Name(), ipVer),
		Domain:  domain,
		Passed:  false,
	}

	switch c.Protocol {
	case "http1":
		return c.checkHTTP1(domain, u, port, ip, result)
	case "http2":
		return c.checkHTTP2(domain, u, port, ip, result)
	case "http3":
		return c.checkHTTP3(domain, u, ip, result)
	default:
		result.Details = fmt.Sprintf("unknown protocol: %s", c.Protocol)
		return result
	}
}

func (c *HTTPChecker) checkHTTP1(domain string, u *url.URL, port int, ip string, result *checker.Result) *checker.Result {
	protocols := &http.Protocols{}
	protocols.SetHTTP1(true)
	protocols.SetHTTP2(false)

	transport := &http.Transport{
		DialContext: c.dialContext(port, ip),
		Protocols:  protocols,
		ResponseHeaderTimeout: 10 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
	}

	if port != 80 {
		transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}

	return c.doRequest(transport, u, result)
}

func (c *HTTPChecker) checkHTTP2(domain string, u *url.URL, port int, ip string, result *checker.Result) *checker.Result {
	protocols := &http.Protocols{}
	protocols.SetHTTP1(false)
	protocols.SetHTTP2(true)

	transport := &http.Transport{
		DialContext: c.dialContext(port, ip),
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		Protocols:             protocols,
		ResponseHeaderTimeout: 10 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
	}

	return c.doRequest(transport, u, result)
}

func (c *HTTPChecker) checkHTTP3(domain string, u *url.URL, ip string, result *checker.Result) *checker.Result {
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

	result.Passed = c.checkStatus(resp.StatusCode)
	result.Details = fmt.Sprintf("%s %s -> %d %s", c.Name(), u.String(), resp.StatusCode, resp.Status)
	return result
}

func (c *HTTPChecker) dialContext(port int, ip string) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		resolvedAddr := net.JoinHostPort(ip, fmt.Sprintf("%d", port))
		dialer := &net.Dialer{Timeout: 10 * time.Second}
		return dialer.DialContext(ctx, "tcp", resolvedAddr)
	}
}

func (c *HTTPChecker) resolveBoth(domain string) (ipv4, ipv6 string) {
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

func (c *HTTPChecker) checkStatus(code int) bool {
	if len(c.Status) == 0 {
		return code >= 200 && code < 400
	}
	for _, s := range c.Status {
		if code == s {
			return true
		}
	}
	return false
}

func (c *HTTPChecker) doRequest(transport http.RoundTripper, u *url.URL, result *checker.Result) *checker.Result {
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

	result.Passed = c.checkStatus(resp.StatusCode)
	result.Details = fmt.Sprintf("%s %s -> %d %s", c.Name(), u.String(), resp.StatusCode, resp.Status)
	result.HTTPVersion = resp.Proto
	result.AltSvc = resp.Header.Get("Alt-Svc")
	return result
}

// SupportsH3 checks if the domain supports HTTP/3 based on DNS HTTPS record or Alt-Svc header.
func SupportsH3(domain string, altSvc string) bool {
	// Check Alt-Svc header from HTTP/2 response
	if strings.Contains(altSvc, "h3") {
		return true
	}

	// Check DNS HTTPS record
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	m := &net.Resolver{}
	_ = ctx
	_ = m

	// Use miekg/dns to check HTTPS record
	return checkDNSHTTPS(domain)
}

func checkDNSHTTPS(domain string) bool {
	// Simple check - just return false for now, will be implemented properly
	return false
}
