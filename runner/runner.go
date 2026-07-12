package runner

import (
	"fmt"

	"http-tester/checker"
	"http-tester/checker/dns"
	httpchecker "http-tester/checker/web"
	"http-tester/config"
)

// Run executes all configured checks for a domain and stores results in stats.
func Run(cfg *config.DomainConfig, resolverAddr string, stats *checker.Stats) error {
	resolver, err := dns.NewResolver(resolverAddr)
	if err != nil {
		return err
	}

	var dnsResult *dns.DNSResult
	httpsRecordExists := false

	// Run DNS checks
	if cfg.HasDNS && cfg.DNS != nil {
		if cfg.DNS.A != "" || cfg.DNS.AAAA != "" {
			c := dns.New(resolver)
			c.A = cfg.DNS.A
			c.AAAA = cfg.DNS.AAAA
			dnsResult = c.Check(cfg.Name, stats)
		}

		// HTTPS record check
		if cfg.DNS.HTTPS != "" {
			c := dns.NewHTTPSChecker(resolver)
			httpsRecordExists = c.Check(cfg.Name, cfg.DNS.HTTPS, stats)

			if (cfg.DNS.HTTPS == config.DNSOptional || cfg.DNS.HTTPS == config.DNSYes) && httpsRecordExists {
				dns.ConsistencyCheck(cfg.Name, resolver, stats)
			}
		}
	}

	// Run automatic HTTP checks if enabled
	if cfg.Web != nil || cfg.HasWeb {
		hasHTTPSCheck := cfg.DNS != nil && cfg.DNS.HTTPS != ""
		httpsCheckMode := ""
		if cfg.DNS != nil {
			httpsCheckMode = string(cfg.DNS.HTTPS)
		}
		httpMode := "any"
		httpsMode := "any"
		if cfg.Web != nil {
			if cfg.Web.HTTP != "" {
				httpMode = string(cfg.Web.HTTP)
			}
			if cfg.Web.HTTPS != "" {
				httpsMode = string(cfg.Web.HTTPS)
			}
		}
		var ipv4s, ipv6s []string
		testAllIPs := false
		if cfg.Web != nil {
			testAllIPs = cfg.Web.TestAllIPs
		}
		if dnsResult != nil {
			ipv4s = dnsResult.IPv4s
			ipv6s = dnsResult.IPv6s
		}

		// Warn when multiple IPs exist but test_all_ips is off
		if !testAllIPs {
			if len(ipv4s) > 1 {
				stats.AddRunResult(checker.RunResult{Domain: cfg.Name, Results: []*checker.Result{{
					Checker: "web-info",
					Group:   "DNS",
					Domain:  cfg.Name,
					Passed:  true,
					Warning: true,
					Details: fmt.Sprintf("Found %d IPv4 addresses, testing only system-selected (%s). Use --web.test-all-ips to test all.", len(ipv4s), ipv4s[0]),
				}}})
			}
			if len(ipv6s) > 1 {
				stats.AddRunResult(checker.RunResult{Domain: cfg.Name, Results: []*checker.Result{{
					Checker: "web-info",
					Group:   "DNS",
					Domain:  cfg.Name,
					Passed:  true,
					Warning: true,
					Details: fmt.Sprintf("Found %d IPv6 addresses, testing only system-selected (%s). Use --web.test-all-ips to test all.", len(ipv6s), ipv6s[0]),
				}}})
			}
		}

		httpchecker.RunAutoChecks(cfg.Name, ipv4s, ipv6s, testAllIPs, hasHTTPSCheck, httpsRecordExists, httpsCheckMode, httpMode, httpsMode, stats)
	}

	return nil
}
