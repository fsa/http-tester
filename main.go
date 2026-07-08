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

	exitCode := report.Print(allResults)
	os.Exit(exitCode)
}

func runDomain(domain string, dnsChecks *config.DNSChecks, httpChecks []config.HTTPCheck, resolver string) checker.RunResult {
	rr := checker.RunResult{Domain: domain}

	if dnsChecks != nil {
		if dnsChecks.A || dnsChecks.AAAA {
			c := dns.New(resolver)
			c.CheckA = dnsChecks.A
			c.CheckAAAA = dnsChecks.AAAA
			res, err := c.Check(domain)
			if err != nil {
				res = &checker.Result{
					Checker: c.Name(),
					Domain:  domain,
					Passed:  false,
					Details: fmt.Sprintf("error: %v", err),
				}
			}
			rr.Results = append(rr.Results, res)
		}

		if dnsChecks.HTTPS {
			c := dns.NewHTTPSChecker(resolver)
			res, err := c.Check(domain)
			if err != nil {
				res = &checker.Result{
					Checker: c.Name(),
					Domain:  domain,
					Passed:  false,
					Details: fmt.Sprintf("error: %v", err),
				}
			}
			rr.Results = append(rr.Results, res)
		}

		if dnsChecks.Consistency {
			res, err := dns.ConsistencyCheck(domain, resolver)
			if err != nil {
				res = &checker.Result{
					Checker: "dns-consistency",
					Domain:  domain,
					Passed:  false,
					Details: fmt.Sprintf("error: %v", err),
				}
			}
			rr.Results = append(rr.Results, res)
		}
	}

	for _, hc := range httpChecks {
		c := &httpchecker.HTTPChecker{
			Protocol: hc.Protocol,
			IP:       hc.IP,
			Port:     hc.Port,
			Status:   hc.Status,
		}
		res, err := c.Check(domain)
		if err != nil {
			res = &checker.Result{
				Checker: c.Name(),
				Domain:  domain,
				Passed:  false,
				Details: fmt.Sprintf("error: %v", err),
			}
		}
		rr.Results = append(rr.Results, res)
	}

	return rr
}
