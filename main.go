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
		rr := checker.RunResult{Domain: d.Name}

		if d.Checks.DNS != nil {
			if d.Checks.DNS.A || d.Checks.DNS.AAAA {
				c := dns.New(*resolver)
				c.CheckA = d.Checks.DNS.A
				c.CheckAAAA = d.Checks.DNS.AAAA
				res, err := c.Check(d.Name)
				if err != nil {
					res = &checker.Result{
						Checker: c.Name(),
						Domain:  d.Name,
						Passed:  false,
						Details: fmt.Sprintf("error: %v", err),
					}
				}
				rr.Results = append(rr.Results, res)
			}

			if d.Checks.DNS.HTTPS {
				c := dns.NewHTTPSChecker(*resolver)
				res, err := c.Check(d.Name)
				if err != nil {
					res = &checker.Result{
						Checker: c.Name(),
						Domain:  d.Name,
						Passed:  false,
						Details: fmt.Sprintf("error: %v", err),
					}
				}
				rr.Results = append(rr.Results, res)
			}

			if d.Checks.DNS.Consistency {
				res, err := dns.ConsistencyCheck(d.Name, *resolver)
				if err != nil {
					res = &checker.Result{
						Checker: "dns-consistency",
						Domain:  d.Name,
						Passed:  false,
						Details: fmt.Sprintf("error: %v", err),
					}
				}
				rr.Results = append(rr.Results, res)
			}
		}

		if d.Checks.HTTP != nil {
			for _, hc := range d.Checks.HTTP.Checks {
				c := &httpchecker.HTTPChecker{
					Protocol: hc.Protocol,
					IP:       hc.IP,
				}
				res, err := c.Check(d.Name)
				if err != nil {
					res = &checker.Result{
						Checker: c.Name(),
						Domain:  d.Name,
						Passed:  false,
						Details: fmt.Sprintf("error: %v", err),
					}
				}
				rr.Results = append(rr.Results, res)
			}
		}

		allResults = append(allResults, rr)
	}

	exitCode := report.Print(allResults)
	os.Exit(exitCode)
}
