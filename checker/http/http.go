package http

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"http-tester/checker"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
)

type HTTPChecker struct {
	Protocol string // http1, http2, http3
	IP       string // ipv4, ipv6
}

func (c *HTTPChecker) Name() string {
	return fmt.Sprintf("http-%s-%s", c.Protocol, c.IP)
}

func (c *HTTPChecker) Check(domain string) (*checker.Result, error) {
	result := &checker.Result{
		Checker: c.Name(),
		Domain:  domain,
		Passed:  false,
	}

	u := &url.URL{
		Scheme: "https",
		Host:   domain,
		Path:   "/",
	}

	switch c.Protocol {
	case "http1":
		return c.checkHTTP1(domain, u, result)
	case "http2":
		return c.checkHTTP2(domain, u, result)
	case "http3":
		return c.checkHTTP3(domain, u, result)
	default:
		result.Details = fmt.Sprintf("unknown protocol: %s", c.Protocol)
		return result, nil
	}
}

func (c *HTTPChecker) checkHTTP1(domain string, u *url.URL, result *checker.Result) (*checker.Result, error) {
	protocols := &http.Protocols{}
	protocols.SetHTTP1(true)
	protocols.SetHTTP2(false)

	transport := &http.Transport{
		DialContext:            c.dialContext(),
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		Protocols:             protocols,
		ResponseHeaderTimeout: 10 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
	}

	return c.doRequest(transport, u, result)
}

func (c *HTTPChecker) checkHTTP2(domain string, u *url.URL, result *checker.Result) (*checker.Result, error) {
	protocols := &http.Protocols{}
	protocols.SetHTTP1(false)
	protocols.SetHTTP2(true)

	transport := &http.Transport{
		DialContext:            c.dialContext(),
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		Protocols:             protocols,
		ResponseHeaderTimeout: 10 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
	}

	return c.doRequest(transport, u, result)
}

func (c *HTTPChecker) checkHTTP3(domain string, u *url.URL, result *checker.Result) (*checker.Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	resolvedIP := c.resolveIP(domain)
	dialAddr := net.JoinHostPort(resolvedIP, "443")

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
		return result, nil
	}

	resp, err := client.Do(req)
	if err != nil {
		result.Details = fmt.Sprintf("HTTP/3 request failed: %v", err)
		return result, nil
	}
	defer resp.Body.Close()

	result.Passed = resp.StatusCode >= 200 && resp.StatusCode < 400
	result.Details = fmt.Sprintf("http3 https://%s/ -> %d %s", domain, resp.StatusCode, resp.Status)
	return result, nil
}

func (c *HTTPChecker) dialContext() func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}

		resolvedIP := c.resolveIP(host)
		resolvedAddr := net.JoinHostPort(resolvedIP, port)

		dialer := &net.Dialer{Timeout: 10 * time.Second}
		return dialer.DialContext(ctx, "tcp", resolvedAddr)
	}
}

func (c *HTTPChecker) resolveIP(domain string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resolver := &net.Resolver{}
	addrs, err := resolver.LookupIPAddr(ctx, domain)
	if err != nil {
		return domain
	}

	for _, addr := range addrs {
		if c.IP == "ipv4" && addr.IP.To4() != nil {
			return addr.IP.String()
		}
		if c.IP == "ipv6" && addr.IP.To4() == nil {
			return addr.IP.String()
		}
	}

	if len(addrs) > 0 {
		return addrs[0].IP.String()
	}
	return domain
}

func (c *HTTPChecker) doRequest(transport http.RoundTripper, u *url.URL, result *checker.Result) (*checker.Result, error) {
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
		return result, nil
	}

	resp, err := client.Do(req)
	if err != nil {
		result.Details = fmt.Sprintf("request failed: %v", err)
		return result, nil
	}
	defer resp.Body.Close()

	result.Passed = resp.StatusCode >= 200 && resp.StatusCode < 400
	result.Details = fmt.Sprintf("%s %s -> %d %s", c.Protocol, u.String(), resp.StatusCode, resp.Status)
	return result, nil
}
