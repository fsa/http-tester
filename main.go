package main

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"http-tester/checker"
	"http-tester/checker/dns"
	httpchecker "http-tester/checker/web"
	"http-tester/config"
	"http-tester/report"

	flag "github.com/spf13/pflag"
)

func main() {
	resolver := flag.StringP("resolver", "r", "", "DNS resolver address (e.g. 8.8.8.8 or 2001:4860:4860::8888)")
	port := flag.StringP("port", "p", "53", "DNS resolver port")
	format := flag.StringP("format", "f", "text", "output format: text, json, json-pretty, yaml")
	configFile := flag.StringP("config", "c", "", "config file path")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <domain>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "       %s [options] -c <config.yaml>\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	var cfg *config.DomainConfig

	if *configFile != "" {
		// Config mode: load from file
		var err error
		cfg, err = config.LoadDomain(*configFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}
	} else if flag.NArg() > 0 {
		// Quick mode: test domain with default config
		cfg = &config.DomainConfig{
			Name:   flag.Arg(0),
			HasDNS: true,
			DNS: &config.DNSChecks{
				A:     config.DNSMaybe,
				AAAA:  config.DNSMaybe,
				HTTPS: config.DNSMaybe,
			},
			HasWeb: true,
			Web: &config.WebChecks{
				HTTP:  config.HTTPAny,
				HTTPS: config.HTTPAny,
			},
		}
	} else {
		flag.Usage()
		os.Exit(1)
	}

	// Build resolver address from host + port
	var resolverAddr string
	if *resolver != "" {
		host := strings.Trim(*resolver, "[]")
		resolverAddr = net.JoinHostPort(host, *port)
	}

	startTime := time.Now()

	// Print plan (only in text mode)
	if *format == "text" || *format == "" {
		fmt.Fprintf(os.Stderr, "\nStarted: %s\n", startTime.Format("02.01.2006 15:04:05 MST"))
		printPlan(cfg)
	}

	var allResults []checker.RunResult

	rr := runDomain(cfg.Name, cfg.DNS, cfg.HasDNS, cfg.Web, cfg.HasWeb, resolverAddr)
	allResults = append(allResults, rr)

	for _, alias := range cfg.Aliases {
		arr := runDomain(alias.Name, alias.DNS, alias.HasDNS, alias.Web, alias.HasWeb, resolverAddr)
		allResults = append(allResults, arr)
	}

	exitCode := report.Print(allResults, resolverAddr, startTime, *format)
	os.Exit(exitCode)
}

func printPlan(cfg *config.DomainConfig) {
	fmt.Fprintf(os.Stderr, "\n\033[36mTesting: %s\033[0m\n", cfg.Name)

	if cfg.HasDNS {
		parts := []string{}
		if cfg.DNS != nil {
			if cfg.DNS.A != "" {
				parts = append(parts, fmt.Sprintf("A(%s)", cfg.DNS.A))
			}
			if cfg.DNS.AAAA != "" {
				parts = append(parts, fmt.Sprintf("AAAA(%s)", cfg.DNS.AAAA))
			}
			if cfg.DNS.HTTPS != "" {
				parts = append(parts, fmt.Sprintf("HTTPS(%s)", cfg.DNS.HTTPS))
			}
		}
		if len(parts) > 0 {
			fmt.Fprintf(os.Stderr, "  DNS: %s\n", strings.Join(parts, ", "))
		}
	}

	if cfg.HasWeb {
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
		fmt.Fprintf(os.Stderr, "  Web: HTTP(%s), HTTPS(%s)\n", httpMode, httpsMode)
	}

	for _, alias := range cfg.Aliases {
		fmt.Fprintf(os.Stderr, "\n\033[36mAlias: %s\033[0m\n", alias.Name)
		if alias.HasDNS {
			parts := []string{}
			if alias.DNS != nil {
				if alias.DNS.A != "" {
					parts = append(parts, fmt.Sprintf("A(%s)", alias.DNS.A))
				}
				if alias.DNS.AAAA != "" {
					parts = append(parts, fmt.Sprintf("AAAA(%s)", alias.DNS.AAAA))
				}
				if alias.DNS.HTTPS != "" {
					parts = append(parts, fmt.Sprintf("HTTPS(%s)", alias.DNS.HTTPS))
				}
			}
			if len(parts) > 0 {
				fmt.Fprintf(os.Stderr, "  DNS: %s\n", strings.Join(parts, ", "))
			}
		}
		if alias.HasWeb {
			httpMode := "any"
			httpsMode := "any"
			if alias.Web != nil {
				if alias.Web.HTTP != "" {
					httpMode = string(alias.Web.HTTP)
				}
				if alias.Web.HTTPS != "" {
					httpsMode = string(alias.Web.HTTPS)
				}
			}
			fmt.Fprintf(os.Stderr, "  Web: HTTP(%s), HTTPS(%s)\n", httpMode, httpsMode)
		}
	}

	fmt.Fprintf(os.Stderr, "\n")
}

func runDomain(domain string, dnsChecks *config.DNSChecks, hasDNS bool, webChecks *config.WebChecks, cfgHasWeb bool, resolver string) checker.RunResult {
	rr := checker.RunResult{Domain: domain}

	var dnsResult *dns.DNSResult
	httpsRecordExists := false

	// Run DNS checks
	if hasDNS && dnsChecks != nil {
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
				// Check if HTTPS records were found
				for _, r := range results {
					if len(r.Records) > 0 {
						httpsRecordExists = true
						break
					}
				}

				if dnsChecks.HTTPS == config.DNSNo {
					for _, r := range results {
						if r.Passed && len(r.Records) > 0 {
							r.Passed = false
							r.Details = fmt.Sprintf("HTTPS records found but not expected: %d record(s)", len(r.Records))
						} else if !r.Passed && len(r.Records) == 0 {
							r.Passed = true
							r.Details = "no HTTPS records (expected)"
						}
					}
				} else if dnsChecks.HTTPS == config.DNSMaybe {
					httpsFound := false
					for _, r := range results {
						if r.Passed && len(r.Records) > 0 {
							httpsFound = true
							r.Details = fmt.Sprintf("found %d HTTPS record(s) (optional)", len(r.Records))
						} else if !r.Passed {
							r.Passed = true
							r.Details = "no HTTPS records (optional)"
						}
					}
					if httpsFound {
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
				} else {
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
	// webChecks can be nil if web: section is empty/null, apply defaults
	if webChecks != nil || cfgHasWeb {
		hasHTTPSCheck := dnsChecks != nil && dnsChecks.HTTPS != ""
		httpsCheckMode := ""
		if dnsChecks != nil {
			httpsCheckMode = string(dnsChecks.HTTPS)
		}
		httpMode := "any"
		httpsMode := "any"
		if webChecks != nil {
			if webChecks.HTTP != "" {
				httpMode = string(webChecks.HTTP)
			}
			if webChecks.HTTPS != "" {
				httpsMode = string(webChecks.HTTPS)
			}
		}
		results := httpchecker.RunAutoChecks(domain, hasHTTPSCheck, httpsRecordExists, httpsCheckMode, httpMode, httpsMode)

		if dnsResult != nil {
			var filtered []*checker.Result
			for _, r := range results {
				if contains(r.Checker, "ipv4") && !dnsResult.HasA {
					continue
				}
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
