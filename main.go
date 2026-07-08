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
	testsFile := flag.String("tests", "", "path to tests YAML config file")
	resolver := flag.String("resolver", "", "DNS resolver address (e.g. 8.8.8.8:53). If empty, system default is used.")
	flag.Parse()

	if *testsFile == "" {
		fmt.Fprintln(os.Stderr, "Usage: http-tester -tests <config.yaml> [-resolver <addr>]")
		flag.PrintDefaults()
		os.Exit(1)
	}

	cfg, err := config.LoadTests(*testsFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	var allResults []checker.RunResult

	for _, d := range cfg.Domains {
		rr := runDomain(d.Name, d.Checks, *resolver)
		allResults = append(allResults, rr)

		for _, alias := range d.Aliases {
			arr := runDomain(alias.Name, alias.Checks, *resolver)
			allResults = append(allResults, arr)
		}
	}

	exitCode := report.Print(allResults)
	os.Exit(exitCode)
}

func runDomain(domain string, checks config.Checks, resolver string) checker.RunResult {
	rr := checker.RunResult{Domain: domain}

	if checks.DNS != nil {
		if checks.DNS.A || checks.DNS.AAAA {
			c := dns.New(resolver)
			c.CheckA = checks.DNS.A
			c.CheckAAAA = checks.DNS.AAAA
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

		if checks.DNS.HTTPS {
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

		if checks.DNS.Consistency {
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

	if checks.HTTP != nil {
		for _, hc := range checks.HTTP.Checks {
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
	}

	return rr
}
