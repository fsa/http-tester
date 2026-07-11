package main

import (
	"fmt"
	"net"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"http-tester/checker"
	"http-tester/checker/dns"
	"http-tester/checker/teststats"
	httpchecker "http-tester/checker/web"
	"http-tester/config"
	"http-tester/report"

	flag "github.com/spf13/pflag"
	"golang.org/x/text/language"
)

var version = "dev"

func printVersion() {
	fmt.Printf("http-tester %s\n", version)

	if version != "dev" {
		return
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}

	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			commit := setting.Value
			if len(commit) > 12 {
				commit = commit[:12]
			}
			fmt.Printf("commit: %s\n", commit)
		case "vcs.time":
			fmt.Printf("commit time: %s\n", setting.Value)
		case "vcs.modified":
			if setting.Value == "true" {
				fmt.Println("modified: true (uncommitted changes)")
			}
		}
	}
}

func main() {
	var (
		resolver    string
		port        string
		format      string
		configFile  string
		showVersion bool
	)

	flag.StringVarP(&resolver, "resolver", "r", "", "DNS resolver address (e.g. 8.8.8.8 or 2001:4860:4860::8888)")
	flag.StringVarP(&port, "port", "p", "53", "DNS resolver port")
	flag.StringVarP(&format, "format", "f", "text", "output format: text, json, json-pretty, yaml")
	flag.StringVarP(&configFile, "config", "c", "", "config file path")
	flag.BoolVarP(&showVersion, "version", "V", false, "print version and exit")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <domain>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "       %s [options] -c <config.yaml>\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if showVersion {
		printVersion()
		return
	}

	domain := ""
	if flag.NArg() > 0 {
		domain = flag.Arg(0)
	}

	cfg, err := config.Load(domain, configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Build resolver address from host + port
	var resolverAddr string
	if resolver != "" {
		host := strings.Trim(resolver, "[]")
		resolverAddr = net.JoinHostPort(host, port)
	}

	// Check local IPv4/IPv6 connectivity before any tests
	localIPv4, localIPv6 := checkLocalConnectivity()

	// Check resolver reachability if explicitly specified
	if resolverAddr != "" {
		if !probeUDP(resolverAddr) {
			fmt.Fprintf(os.Stderr, "\n\033[31mError:\033[0m specified resolver %s is not reachable\n", resolverAddr)
			os.Exit(1)
		}
	}

	startTime := time.Now()

	// Print plan (only in text mode)
	if format == "text" {
		lang, _ := language.Parse(os.Getenv("LANG"))
		region, _ := lang.Region()
		dateFmt := "02.01.2006 15:04:05 MST"
		if region == language.MustParseRegion("US") || region == language.MustParseRegion("CA") {
			dateFmt = "01/02/2006 15:04:05 MST"
		}
		fmt.Fprintf(os.Stderr, "\n\033[36mStarted\033[0m: %s\n", startTime.Format(dateFmt))
		printPlan(cfg)
	}

	stats := &teststats.Stats{}

	rr := runDomain(cfg.Name, cfg.DNS, cfg.HasDNS, cfg.Web, cfg.HasWeb, resolverAddr, localIPv4, localIPv6)
	stats.AddRunResult(rr)

	for _, alias := range cfg.Aliases {
		arr := runDomain(alias.Name, alias.DNS, alias.HasDNS, alias.Web, alias.HasWeb, resolverAddr, localIPv4, localIPv6)
		stats.AddRunResult(arr)
	}

	exitCode := report.Print(stats, resolverAddr, startTime, format)
	os.Exit(exitCode)
}

func printPlan(cfg *config.DomainConfig) {
	fmt.Fprintf(os.Stderr, "\n\033[36mTesting\033[0m: %s\n", cfg.Name)

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

func runDomain(domain string, dnsChecks *config.DNSChecks, hasDNS bool, webChecks *config.WebChecks, cfgHasWeb bool, resolverAddr string, localIPv4, localIPv6 bool) checker.RunResult {
	rr := checker.RunResult{Domain: domain}

	resolver := dns.NewResolver(resolverAddr)

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
					Error:   true,
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
					Error:   true,
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
				} else if dnsChecks.HTTPS == config.DNSOptional {
					httpsFound := false
					for _, r := range results {
						if r.Passed && len(r.Records) > 0 {
							httpsFound = true
							r.Details = fmt.Sprintf("found %d HTTPS record(s) (optional)", len(r.Records))
						} else if !r.Passed && !r.Error {
							// No records but no error — mark as passed for optional
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
								Error:   true,
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
								Error:   true,
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
		var ipv4s, ipv6s []string
		testAllIPs := false
		if webChecks != nil {
			testAllIPs = webChecks.TestAllIPs
		}
		if dnsResult != nil {
			ipv4s = dnsResult.IPv4s
			ipv6s = dnsResult.IPv6s
		}

		// Warn when multiple IPs exist but test_all_ips is off
		if !testAllIPs {
			if len(ipv4s) > 1 {
				rr.Results = append(rr.Results, &checker.Result{
					Checker: "web-info",
					Domain:  domain,
					Passed:  true,
					Warning: true,
					Details: fmt.Sprintf("Found %d IPv4 addresses, testing only system-selected (%s). Set test_all_ips: true to test all.", len(ipv4s), ipv4s[0]),
				})
			}
			if len(ipv6s) > 1 {
				rr.Results = append(rr.Results, &checker.Result{
					Checker: "web-info",
					Domain:  domain,
					Passed:  true,
					Warning: true,
					Details: fmt.Sprintf("Found %d IPv6 addresses, testing only system-selected (%s). Set test_all_ips: true to test all.", len(ipv6s), ipv6s[0]),
				})
			}
		}

		results := httpchecker.RunAutoChecks(domain, ipv4s, ipv6s, testAllIPs, hasHTTPSCheck, httpsRecordExists, httpsCheckMode, httpMode, httpsMode, localIPv4, localIPv6)

		rr.Results = append(rr.Results, results...)
	}

	return rr
}



func checkLocalConnectivity() (hasIPv4, hasIPv6 bool) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return false, false
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ip, _, err := net.ParseCIDR(addr.String())
			if err != nil {
				continue
			}
			if ip.To4() != nil {
				hasIPv4 = true
			} else if ip.To16() != nil {
				hasIPv6 = true
			}
		}
	}
	return hasIPv4, hasIPv6
}

func probeUDP(addr string) bool {
	conn, err := net.DialTimeout("udp", addr, 500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
