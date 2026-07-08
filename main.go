package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

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

func runDomain(domain string, dnsChecks *config.DNSChecks, httpChecks []config.HTTPCheck, resolver string) checker.RunResult {
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

	// Run HTTP checks
	// First check if HTTP/3 should be tested
	shouldTestH3 := false
	for _, hc := range httpChecks {
		if hc.Protocol == "http3" {
			shouldTestH3 = true
			break
		}
	}

	if shouldTestH3 {
		// Check if domain has HTTPS DNS record
		hasHTTPSRecord := dns.HasHTTPSRecord(domain, resolver)

		// Also check Alt-Svc header from HTTP/2 response
		altSvc := getAltSvcFromHTTP2(domain, resolver)

		if !hasHTTPSRecord && !strings.Contains(altSvc, "h3") {
			// Skip HTTP/3 - no support detected
			for _, hc := range httpChecks {
				if hc.Protocol == "http3" {
					rr.Results = append(rr.Results, &checker.Result{
						Checker: fmt.Sprintf("https-http3"),
						Domain:  domain,
						Passed:  false,
						Details: "skipped: no HTTPS DNS record and no Alt-Svc h3 support",
					})
				}
			}
		}
	}

	for _, hc := range httpChecks {
		// Skip HTTP/3 if no support detected
		if hc.Protocol == "http3" && !shouldTestH3 {
			continue
		}

		c := &httpchecker.HTTPChecker{
			Protocol: hc.Protocol,
			Port:     hc.Port,
			Status:   hc.Status,
		}
		results, err := c.Check(domain)
		if err != nil {
			results = []*checker.Result{{
				Checker: c.Name(),
				Domain:  domain,
				Passed:  false,
				Details: fmt.Sprintf("error: %v", err),
			}}
		}

		// Filter results based on DNS availability
		if dnsResult != nil {
			var filtered []*checker.Result
			for _, r := range results {
				// Check if this result is for a specific IP version
				if strings.Contains(r.Checker, "ipv4") && !dnsResult.HasA {
					// Skip IPv4 checks if no A record (or "no" was set)
					continue
				}
				if strings.Contains(r.Checker, "ipv6") && !dnsResult.HasAAAA {
					// Skip IPv6 checks if no AAAA record (or "no" was set)
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

func getAltSvcFromHTTP2(domain string, resolver string) string {
	c := &httpchecker.HTTPChecker{
		Protocol: "http2",
		Port:     443,
		Status:   []int{200, 301, 302},
	}
	results, _ := c.Check(domain)
	for _, r := range results {
		if r.AltSvc != "" {
			return r.AltSvc
		}
	}
	return ""
}
