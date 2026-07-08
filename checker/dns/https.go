package dns

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"http-tester/checker"

	mdns "github.com/miekg/dns"
)

type HTTPSChecker struct {
	Server string // "8.8.8.8:53" etc
}

func NewHTTPSChecker(server string) *HTTPSChecker {
	if server == "" {
		server = "8.8.8.8:53"
	}
	return &HTTPSChecker{Server: server}
}

func (c *HTTPSChecker) Name() string {
	return "dns-https"
}

func (c *HTTPSChecker) Check(domain string) ([]*checker.Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := &checker.Result{
		Checker: "dns-https",
		Domain:  domain,
		Passed:  true,
	}

	m := new(mdns.Msg)
	m.SetQuestion(mdns.Fqdn(domain), mdns.TypeHTTPS)
	m.RecursionDesired = true

	client := new(mdns.Client)
	resp, _, err := client.ExchangeContext(ctx, m, c.Server)
	if err != nil {
		result.Passed = false
		result.Details = fmt.Sprintf("HTTPS lookup failed: %v", err)
		return []*checker.Result{result}, nil
	}

	if resp.Rcode != mdns.RcodeSuccess {
		result.Details = "no HTTPS records"
		return []*checker.Result{result}, nil
	}

	for _, rr := range resp.Answer {
		if https, ok := rr.(*mdns.HTTPS); ok {
			svcParams := formatSVCB(https.Priority, https.Target, https.Value)
			result.Records = append(result.Records, checker.Record{
				Type:  "HTTPS",
				Value: svcParams,
			})
		}
	}

	if len(result.Records) == 0 {
		result.Details = "no HTTPS records in response"
		return []*checker.Result{result}, nil
	}

	result.Details = fmt.Sprintf("found %d HTTPS record(s)", len(result.Records))
	return []*checker.Result{result}, nil
}

func formatSVCB(priority uint16, target string, params []mdns.SVCBKeyValue) string {
	s := fmt.Sprintf("priority=%d", priority)
	if target != "" {
		s += fmt.Sprintf(" target=%s", target)
	}
	for _, p := range params {
		s += fmt.Sprintf(" %s=%s", p.Key(), p.String())
	}
	return s
}

// ConsistencyCheck verifies that HTTPS records point to targets resolvable via A/AAAA.
func ConsistencyCheck(domain string, server string) ([]*checker.Result, error) {
	if server == "" {
		server = "8.8.8.8:53"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result := &checker.Result{
		Checker: "dns-consistency",
		Domain:  domain,
		Passed:  true,
	}

	a4s, a6s, err := resolveIPs(ctx, domain, server)
	if err != nil {
		result.Passed = false
		result.Details = fmt.Sprintf("base resolution failed: %v", err)
		return []*checker.Result{result}, nil
	}
	if len(a4s) == 0 && len(a6s) == 0 {
		result.Passed = false
		result.Details = "no A/AAAA records for domain"
		return []*checker.Result{result}, nil
	}

	m := new(mdns.Msg)
	m.SetQuestion(mdns.Fqdn(domain), mdns.TypeHTTPS)
	m.RecursionDesired = true
	client := new(mdns.Client)
	resp, _, err := client.ExchangeContext(ctx, m, server)
	if err != nil {
		result.Passed = false
		result.Details = fmt.Sprintf("HTTPS lookup failed: %v", err)
		return []*checker.Result{result}, nil
	}

	if resp.Rcode != mdns.RcodeSuccess {
		result.Details = "no HTTPS records, consistency check skipped"
		return []*checker.Result{result}, nil
	}

	var httpsRecords []*mdns.HTTPS
	for _, rr := range resp.Answer {
		if h, ok := rr.(*mdns.HTTPS); ok {
			httpsRecords = append(httpsRecords, h)
		}
	}

	if len(httpsRecords) == 0 {
		result.Details = "no HTTPS records, consistency check skipped"
		return []*checker.Result{result}, nil
	}

	inconsistencies := []string{}
	for _, h := range httpsRecords {
		target := h.Target
		if target == "." || target == "" {
			continue
		}
		t4s, t6s, err := resolveIPs(ctx, target, server)
		if err != nil {
			inconsistencies = append(inconsistencies,
				fmt.Sprintf("HTTPS target %s unresolvable: %v", target, err))
			continue
		}
		if len(t4s) == 0 && len(t6s) == 0 {
			inconsistencies = append(inconsistencies,
				fmt.Sprintf("HTTPS target %s has no A/AAAA records", target))
			continue
		}
		overlap := hasOverlap(a4s, t4s) || hasOverlap(a6s, t6s) || hasOverlap(a4s, t6s) || hasOverlap(a6s, t4s)
		if !overlap {
			inconsistencies = append(inconsistencies,
				fmt.Sprintf("HTTPS target %s resolves to %v/%v, domain resolves to %v/%v — no overlap",
					target, t4s, t6s, a4s, a6s))
		}
	}

	if len(inconsistencies) > 0 {
		result.Passed = false
		result.Details = strings.Join(inconsistencies, "; ")
	} else {
		result.Details = fmt.Sprintf("consistent: %d HTTPS record(s) match A/AAAA", len(httpsRecords))
	}

	return []*checker.Result{result}, nil
}

func resolveIPs(ctx context.Context, domain, server string) (ipv4s, ipv6s []string, err error) {
	r := &net.Resolver{
		PreferGo: true,
	}
	if server != "" {
		r.Dial = func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 5 * time.Second}
			return d.DialContext(ctx, "udp", server)
		}
	}

	addrs, err := r.LookupIPAddr(ctx, domain)
	if err != nil {
		return nil, nil, err
	}
	for _, a := range addrs {
		if a.IP.To4() != nil {
			ipv4s = append(ipv4s, a.IP.String())
		} else {
			ipv6s = append(ipv6s, a.IP.String())
		}
	}
	return ipv4s, ipv6s, nil
}

func hasOverlap(a, b []string) bool {
	set := make(map[string]bool, len(a))
	for _, s := range a {
		set[s] = true
	}
	for _, s := range b {
		if set[s] {
			return true
		}
	}
	return false
}

// HasHTTPSRecord checks if domain has HTTPS DNS record.
func HasHTTPSRecord(domain string, server string) bool {
	if server == "" {
		server = "8.8.8.8:53"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	m := new(mdns.Msg)
	m.SetQuestion(mdns.Fqdn(domain), mdns.TypeHTTPS)
	m.RecursionDesired = true

	client := new(mdns.Client)
	resp, _, err := client.ExchangeContext(ctx, m, server)
	if err != nil {
		return false
	}

	if resp.Rcode != mdns.RcodeSuccess {
		return false
	}

	for _, rr := range resp.Answer {
		if _, ok := rr.(*mdns.HTTPS); ok {
			return true
		}
	}
	return false
}
