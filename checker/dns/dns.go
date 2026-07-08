package dns

import (
	"context"
	"fmt"
	"net"
	"time"

	"http-tester/checker"
)

type DNSChecker struct {
	Resolver  *net.Resolver
	CheckA    bool
	CheckAAAA bool
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

func (c *DNSChecker) Check(domain string) ([]*checker.Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := &checker.Result{
		Checker: "dns",
		Domain:  domain,
		Passed:  true,
	}

	addrs, err := c.Resolver.LookupIPAddr(ctx, domain)
	if err != nil {
		result.Passed = false
		result.Details = fmt.Sprintf("resolution failed: %v", err)
		return []*checker.Result{result}, nil
	}

	var ipv4s, ipv6s []string
	for _, addr := range addrs {
		ip := addr.IP
		if ip.To4() != nil {
			if c.CheckA {
				ipv4s = append(ipv4s, ip.String())
				result.Records = append(result.Records, checker.Record{
					Type:  "A",
					Value: ip.String(),
				})
			}
		} else {
			if c.CheckAAAA {
				ipv6s = append(ipv6s, ip.String())
				result.Records = append(result.Records, checker.Record{
					Type:  "AAAA",
					Value: ip.String(),
				})
			}
		}
	}

	if c.CheckA && len(ipv4s) == 0 {
		result.Passed = false
		result.Details = "no A records found"
		return []*checker.Result{result}, nil
	}
	if c.CheckAAAA && len(ipv6s) == 0 {
		result.Passed = false
		result.Details = "no AAAA records found"
		return []*checker.Result{result}, nil
	}

	parts := []string{}
	if c.CheckA {
		parts = append(parts, fmt.Sprintf("A: %v", ipv4s))
	}
	if c.CheckAAAA {
		parts = append(parts, fmt.Sprintf("AAAA: %v", ipv6s))
	}
	result.Details = fmt.Sprintf("%s resolved", domain)
	if len(parts) > 0 {
		result.Details += ": " + joinParts(parts)
	}

	return []*checker.Result{result}, nil
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
