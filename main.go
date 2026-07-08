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

	// Run DNS checks
	if dnsChecks != nil {
		if dnsChecks.A || dnsChecks.AAAA {
			c := dns.New(resolver)
			c.CheckA = dnsChecks.A
			c.CheckAAAA = dnsChecks.AAAA
			results, err := c.Check(domain)
			if err != nil {
				results = []*checker.Result{{
					Checker: c.Name(),
					Domain:  domain,
					Passed:  false,
					Details: fmt.Sprintf("error: %v", err),
				}}
			}
			rr.Results = append(rr.Results, results...)
		}

		if dnsChecks.HTTPS {
			c := dns.NewHTTPSChecker(resolver)
			results, err := c.Check(domain)
			if err != nil {
				results = []*checker.Result{{
					Checker: c.Name(),
					Domain:  domain,
					Passed:  false,
					Details: fmt.Sprintf("error: %v", err),
				}}
			}
			rr.Results = append(rr.Results, results...)
		}

		if dnsChecks.Consistency {
			results, err := dns.ConsistencyCheck(domain, resolver)
			if err != nil {
				results = []*checker.Result{{
					Checker: "dns-consistency",
					Domain:  domain,
					Passed:  false,
					Details: fmt.Sprintf("error: %v", err),
				}}
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
		rr.Results = append(rr.Results, results...)
	}

	return rr
}

func getAltSvcFromHTTP2(domain string, resolver string) string {
	// Quick HTTP/2 request to get Alt-Svc header
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
