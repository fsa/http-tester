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
	httpchecker "http-tester/checker/web"
	"http-tester/config"
	"http-tester/report"

	flag "github.com/spf13/pflag"
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

	startTime := time.Now()

	stats := &checker.Stats{}

	if err := runDomain(cfg.Name, cfg.DNS, cfg.HasDNS, cfg.Web, cfg.HasWeb, resolverAddr, localIPv4, localIPv6, stats); err != nil {
		fmt.Fprintf(os.Stderr, "\n\033[31mError:\033[0m %v\n", err)
		os.Exit(1)
	}

	if err := report.Print(format, stats, cfg, resolverAddr, startTime); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	os.Exit(stats.Code())
}

func runDomain(domain string, dnsChecks *config.DNSChecks, hasDNS bool, webChecks *config.WebChecks, cfgHasWeb bool, resolverAddr string, localIPv4, localIPv6 bool, stats *checker.Stats) error {
	resolver, err := dns.NewResolver(resolverAddr)
	if err != nil {
		return err
	}

	var dnsResult *dns.DNSResult
	httpsRecordExists := false

	// Run DNS checks
	if hasDNS && dnsChecks != nil {
		if dnsChecks.A != "" || dnsChecks.AAAA != "" {
			c := dns.New(resolver)
			c.A = dnsChecks.A
			c.AAAA = dnsChecks.AAAA
			dnsResult = c.Check(domain, stats)
		}

		// HTTPS record check
		if dnsChecks.HTTPS != "" {
			c := dns.NewHTTPSChecker(resolver)
			httpsRecordExists = c.Check(domain, dnsChecks.HTTPS, stats)

			if (dnsChecks.HTTPS == config.DNSOptional || dnsChecks.HTTPS == config.DNSYes) && httpsRecordExists {
				dns.ConsistencyCheck(domain, resolver, stats)
			}
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
				stats.AddRunResult(checker.RunResult{Domain: domain, Results: []*checker.Result{{
					Checker: "web-info",
					Group:   "DNS",
					Domain:  domain,
					Passed:  true,
					Warning: true,
					Details: fmt.Sprintf("Found %d IPv4 addresses, testing only system-selected (%s). Set test_all_ips: true to test all.", len(ipv4s), ipv4s[0]),
				}}})
			}
			if len(ipv6s) > 1 {
				stats.AddRunResult(checker.RunResult{Domain: domain, Results: []*checker.Result{{
					Checker: "web-info",
					Group:   "DNS",
					Domain:  domain,
					Passed:  true,
					Warning: true,
					Details: fmt.Sprintf("Found %d IPv6 addresses, testing only system-selected (%s). Set test_all_ips: true to test all.", len(ipv6s), ipv6s[0]),
				}}})
			}
		}

		httpchecker.RunAutoChecks(domain, ipv4s, ipv6s, testAllIPs, hasHTTPSCheck, httpsRecordExists, httpsCheckMode, httpMode, httpsMode, localIPv4, localIPv6, stats)
	}

	return nil
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
