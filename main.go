package main

import (
	"flag"
	"fmt"
	"os"

	"http-tester/checker"
	"http-tester/checker/dns"
	httpchecker "http-tester/checker/http"
	"http-tester/config"
	"http-tester/report"
)

func main() {
	resolver := flag.String("resolver", "", "DNS resolver address (e.g. 8.8.8.8:53)")
	format := flag.String("format", "text", "output format: text, json")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <config.yaml>\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	configFile := flag.Arg(0)

	cfg, err := config.LoadDomain(configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	var allResults []checker.RunResult

	rr := runDomain(cfg.Name, cfg.DNS, cfg.HTTP, *resolver)
	allResults = append(allResults, rr)

	for _, alias := range cfg.Aliases {
		arr := runDomain(alias.Name, alias.DNS, alias.HTTP, *resolver)
		allResults = append(allResults, arr)
	}

	exitCode := report.Print(allResults, *format)
	os.Exit(exitCode)
}

func runDomain(domain string, dnsChecks *config.DNSChecks, enableHTTP bool, resolver string) checker.RunResult {
	rr := checker.RunResult{Domain: domain}

	var dnsResult *dns.DNSResult

	// Run DNS checks
	if dnsChecks != nil {
		if dnsChecks.A != "" || dnsChecks.AAAA != "" {
			c := dns.New(resolver)
			c.A = dnsChecks.A
			c.AAAA = dnsChecks.AAAA
			results, result, err := c.Check(domain)
			if err != nil {
				results = []*checker.Result{{
					Checker: c.Name(),
					Domain:  domain,
					Passed:  false,
					Details: fmt.Sprintf("error: %v", err),
				}}
			} else {
				dnsResult = result
			}
			rr.Results = append(rr.Results, results...)
		}

		// HTTPS record check
		if dnsChecks.HTTPS != "" {
			c := dns.NewHTTPSChecker(resolver)
			results, err := c.Check(domain)
			if err != nil {
				results = []*checker.Result{{
					Checker: c.Name(),
					Domain:  domain,
					Passed:  false,
					Details: fmt.Sprintf("error: %v", err),
				}}
			} else {
				// Handle "yes" and "no" cases
				if dnsChecks.HTTPS == config.DNSNo {
					// Expect NO HTTPS records
					for _, r := range results {
						if r.Passed && len(r.Records) > 0 {
							// Records found but not expected
							r.Passed = false
							r.Details = fmt.Sprintf("HTTPS records found but not expected: %d record(s)", len(r.Records))
						} else if !r.Passed && len(r.Records) == 0 {
							// No records found - that's expected for "no"
							r.Passed = true
							r.Details = "no HTTPS records (expected)"
						}
					}
				} else {
					// Expect HTTPS records ("yes")
					httpsPassed := false
					for _, r := range results {
						if r.Passed {
							httpsPassed = true
							break
						}
					}
					if httpsPassed {
						consResults, err := dns.ConsistencyCheck(domain, resolver)
						if err != nil {
							consResults = []*checker.Result{{
								Checker: "dns-consistency",
								Domain:  domain,
								Passed:  false,
								Details: fmt.Sprintf("error: %v", err),
							}}
						}
						results = append(results, consResults...)
					}
				}
			}
			rr.Results = append(rr.Results, results...)
		}
	}

	// Run automatic HTTP checks if enabled
	if enableHTTP {
		hasHTTPSCheck := dnsChecks != nil && dnsChecks.HTTPS != ""
		results := httpchecker.RunAutoChecks(domain, hasHTTPSCheck)

		// Filter results based on DNS availability
		if dnsResult != nil {
			var filtered []*checker.Result
			for _, r := range results {
				// Skip IPv4 checks if no A record
				if contains(r.Checker, "ipv4") && !dnsResult.HasA {
					continue
				}
				// Skip IPv6 checks if no AAAA record
				if contains(r.Checker, "ipv6") && !dnsResult.HasAAAA {
					continue
				}
				filtered = append(filtered, r)
			}
			results = filtered
		}

		rr.Results = append(rr.Results, results...)
	}

	return rr
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
