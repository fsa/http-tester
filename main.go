package main

import (
	"fmt"
	"net"
	"os"
	"runtime/debug"
	"strings"

	"http-tester/checker"
	"http-tester/checker/host"
	"http-tester/config"
	"http-tester/report"
	"http-tester/runner"

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
		resolver     string
		port         string
		format       string
		configFile   string
		showVersion  bool
		dnsA         string
		dnsAAAA      string
		dnsHTTPS     string
		webHTTP      string
		webHTTPS     string
		testAllIPs   bool
		testAllIPsSet bool
	)

	flag.StringVarP(&resolver, "resolver", "r", "", "DNS resolver address")
	flag.StringVarP(&port, "port", "p", "53", "DNS resolver port")
	flag.StringVarP(&format, "format", "f", "text", "output format: text, json, yaml")
	flag.StringVarP(&configFile, "config", "c", "", "config file path")
	flag.BoolVarP(&showVersion, "version", "V", false, "print version and exit")
	flag.StringVar(&dnsA, "dns.a", "", "A record check: yes/no/optional")
	flag.StringVar(&dnsAAAA, "dns.aaaa", "", "AAAA record check: yes/no/optional")
	flag.StringVar(&dnsHTTPS, "dns.https", "", "HTTPS record check: yes/no/optional")
	flag.StringVar(&webHTTP, "web.http", "", "HTTP mode: any/redirect/direct/no")
	flag.StringVar(&webHTTPS, "web.https", "", "HTTPS mode: any/redirect/direct/no")
	flag.BoolVar(&testAllIPs, "web.test-all-ips", false, "test all resolved IPs")

	flag.Visit(func(f *flag.Flag) {
		if f.Name == "web.test-all-ips" {
			testAllIPsSet = true
		}
	})

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <domain>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "       %s [options] -c <config.yaml>\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		fmt.Fprintf(os.Stderr, "  -c, --config <file>        config file path\n")
		fmt.Fprintf(os.Stderr, "  -r, --resolver <addr>      DNS resolver address\n")
		fmt.Fprintf(os.Stderr, "  -p, --port <port>          resolver port (default 53)\n")
		fmt.Fprintf(os.Stderr, "  -f, --format <format>      output format: text, json, yaml\n")
		fmt.Fprintf(os.Stderr, "  -V, --version              print version and exit\n")
		fmt.Fprintf(os.Stderr, "\nConfig overrides (CLI > config file > defaults):\n")
		fmt.Fprintf(os.Stderr, "  --dns.a <value>            yes/no/optional\n")
		fmt.Fprintf(os.Stderr, "  --dns.aaaa <value>         yes/no/optional\n")
		fmt.Fprintf(os.Stderr, "  --dns.https <value>        yes/no/optional\n")
		fmt.Fprintf(os.Stderr, "  --web.http <value>         any/redirect/direct/no\n")
		fmt.Fprintf(os.Stderr, "  --web.https <value>        any/redirect/direct/no\n")
		fmt.Fprintf(os.Stderr, "  --web.test-all-ips         test all resolved IPs\n")
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

	// Apply CLI overrides
	cfg.ApplyCLI(dnsA, dnsAAAA, dnsHTTPS, webHTTP, webHTTPS, testAllIPs, testAllIPsSet)

	// Build resolver address from host + port
	var resolverAddr string
	if resolver != "" {
		host := strings.Trim(resolver, "[]")
		resolverAddr = net.JoinHostPort(host, port)
	}

	// Show pre-test info before running tests
	report.Start(format, cfg)

	stats := &checker.Stats{}

	// Check local IPv4/IPv6 connectivity before any tests
	host.CheckLocalConnectivity(stats)

	// Run all configured checks
	if err := runner.Run(cfg, resolverAddr, stats); err != nil {
		fmt.Fprintf(os.Stderr, "\n\033[31mError:\033[0m %v\n", err)
		os.Exit(1)
	}

	if err := report.Print(format, stats, cfg, resolverAddr); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	os.Exit(stats.Code())
}
