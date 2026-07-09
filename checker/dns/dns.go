package dns

import (
	"context"
	"fmt"
	"net"
	"time"

	"http-tester/checker"
	"http-tester/config"
)

type DNSResult struct {
	HasA    bool
	HasAAAA bool
}

type DNSChecker struct {
	Resolver *net.Resolver
	A        config.DNSRecordCheck
	AAAA     config.DNSRecordCheck
}

func New(resolverAddr string) *DNSChecker {
	r := &net.Resolver{
		PreferGo: true,
	}
	if resolverAddr != "" {
		r.Dial = func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 5 * time.Second}
			return d.DialContext(ctx, "udp", resolverAddr)
		}
	}
	return &DNSChecker{Resolver: r}
}

func (c *DNSChecker) Name() string {
	return "dns"
}

func (c *DNSChecker) Check(domain string) ([]*checker.Result, *DNSResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := &checker.Result{
		Checker: "dns",
		Domain:  domain,
		Passed:  true,
	}

	dnsResult := &DNSResult{}

	addrs, err := c.Resolver.LookupIPAddr(ctx, domain)
	if err != nil {
		result.Passed = false
		result.Details = fmt.Sprintf("resolution failed: %v", err)
		return []*checker.Result{result}, dnsResult, nil
	}

	var ipv4s, ipv6s []string
	for _, addr := range addrs {
		ip := addr.IP
		if ip.To4() != nil {
			ipv4s = append(ipv4s, ip.String())
		} else {
			ipv6s = append(ipv6s, ip.String())
		}
	}

	dnsResult.HasA = len(ipv4s) > 0
	dnsResult.HasAAAA = len(ipv6s) > 0

	// Check A records
	if c.A == config.DNSYes {
		if len(ipv4s) == 0 {
			result.Passed = false
			result.Details = "no A records found (expected)"
			return []*checker.Result{result}, dnsResult, nil
		}
		for _, ip := range ipv4s {
			result.Records = append(result.Records, checker.Record{
				Type:  "A",
				Value: ip,
			})
		}
	} else if c.A == config.DNSNo {
		if len(ipv4s) > 0 {
			result.Passed = false
			result.Details = fmt.Sprintf("A records found but not expected: %v", ipv4s)
			return []*checker.Result{result}, dnsResult, nil
		}
	} else if c.A == config.DNSMaybe {
		for _, ip := range ipv4s {
			result.Records = append(result.Records, checker.Record{
				Type:  "A",
				Value: ip,
			})
		}
		if len(ipv4s) == 0 {
			result.Details = "A records not found (optional)"
		}
	}

	// Check AAAA records
	if c.AAAA == config.DNSYes {
		if len(ipv6s) == 0 {
			result.Passed = false
			result.Details = "no AAAA records found (expected)"
			return []*checker.Result{result}, dnsResult, nil
		}
		for _, ip := range ipv6s {
			result.Records = append(result.Records, checker.Record{
				Type:  "AAAA",
				Value: ip,
			})
		}
	} else if c.AAAA == config.DNSNo {
		if len(ipv6s) > 0 {
			result.Passed = false
			result.Details = fmt.Sprintf("AAAA records found but not expected: %v", ipv6s)
			return []*checker.Result{result}, dnsResult, nil
		}
	} else if c.AAAA == config.DNSMaybe {
		for _, ip := range ipv6s {
			result.Records = append(result.Records, checker.Record{
				Type:  "AAAA",
				Value: ip,
			})
		}
		if len(ipv6s) == 0 {
			result.Details = "AAAA records not found (optional)"
		}
	}

	// Build details message
	parts := []string{}
	if c.A == config.DNSYes || c.A == config.DNSMaybe {
		if len(ipv4s) > 0 {
			parts = append(parts, fmt.Sprintf("A: %v", ipv4s))
		}
	}
	if c.AAAA == config.DNSYes || c.AAAA == config.DNSMaybe {
		if len(ipv6s) > 0 {
			parts = append(parts, fmt.Sprintf("AAAA: %v", ipv6s))
		}
	}
	if len(parts) > 0 {
		result.Details = fmt.Sprintf("%s resolved: %s", domain, joinParts(parts))
	} else if result.Details == "" {
		result.Details = fmt.Sprintf("%s resolved", domain)
	}

	return []*checker.Result{result}, dnsResult, nil
}

func joinParts(parts []string) string {
	s := ""
	for i, p := range parts {
		if i > 0 {
			s += ", "
		}
		s += p
	}
	return s
}
