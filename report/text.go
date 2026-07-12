package report

import (
	"fmt"
	"os"
	"strings"
	"time"

	"http-tester/checker"
	"http-tester/config"

	"golang.org/x/text/language"
)

type TextFormatter struct{}

var tagLabels = map[string]string{
	"ipv4": "IPv4",
	"ipv6": "IPv6",
	"http1": "HTTP/1.1",
	"http2": "HTTP/2",
	"http3": "HTTP/3",
}

func displayTag(tag string) string {
	if label, ok := tagLabels[tag]; ok {
		return label
	}
	return tag
}

func (f *TextFormatter) Start(cfg *config.DomainConfig) {
	startTime := time.Now()
	lang, _ := language.Parse(os.Getenv("LANG"))
	region, _ := lang.Region()
	dateFmt := "02.01.2006 15:04:05 MST"
	if region == language.MustParseRegion("US") || region == language.MustParseRegion("CA") {
		dateFmt = "01/02/2006 15:04:05 MST"
	}
	fmt.Fprintf(os.Stderr, "\n\033[36mStarted\033[0m: %s\n", startTime.Format(dateFmt))
	if cfg != nil {
		f.printPlan(cfg)
	}
}

func (f *TextFormatter) printPlan(cfg *config.DomainConfig) {
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

	fmt.Fprintf(os.Stderr, "\n")
}

type testGroup struct {
	name string
}

var groups = []testGroup{
	{"DNS"},
	{"HTTP"},
	{"HTTPS"},
	{"Consistency"},
}

func (f *TextFormatter) Print(stats *checker.Stats, resolver string) int {
	fmt.Fprintf(os.Stdout, "\n%sTest Results%s\n", colorCyan, colorReset)

	if resolver != "" {
		fmt.Fprintf(os.Stdout, "%sResolver%s: %s\n", colorCyan, colorReset, resolver)
	}

	// Collect all results across all RunResults
	var allResults []*checker.Result
	for _, r := range stats.Results {
		allResults = append(allResults, r.Results...)
	}

	for _, g := range groups {
		var groupResults []*checker.Result
		for _, res := range allResults {
			if res.Group == g.name {
				groupResults = append(groupResults, res)
			}
		}
		if len(groupResults) == 0 {
			continue
		}

		fmt.Fprintf(os.Stdout, "\n  %s%s:%s\n", colorCyan, g.name, colorReset)
		for _, res := range groupResults {
			status := f.statusLabel(res)
			if len(res.Tags) > 0 {
				var tags []string
				for _, t := range res.Tags {
					tags = append(tags, displayTag(t))
				}
				fmt.Fprintf(os.Stdout, "    [%s] %s (%s)\n", status, res.Details, strings.Join(tags, ", "))
			} else {
				fmt.Fprintf(os.Stdout, "    [%s] %s\n", status, res.Details)
			}
			if res.RedirectTo != "" {
				fmt.Fprintf(os.Stdout, "           -> %s\n", res.RedirectTo)
			}
			if res.AltSvc != "" {
				fmt.Fprintf(os.Stdout, "           Alt-Svc: %s\n", res.AltSvc)
			}
			if res.ServerAddr != "" {
				fmt.Fprintf(os.Stdout, "           Server: %s\n", res.ServerAddr)
			}
			for _, rec := range res.Records {
				fmt.Fprintf(os.Stdout, "           %s %s\n", rec.Type, rec.Value)
			}
		}
	}

	f.printSummary(stats)
	return stats.Code()
}

func (f *TextFormatter) statusLabel(res *checker.Result) string {
	if res.Info {
		return colorWhite + "INFO" + colorReset
	}
	if res.Warning {
		return colorYellow + "WARN" + colorReset
	}
	if res.Error {
		return colorRed + "ERROR" + colorReset
	}
	if res.Passed {
		return colorGreen + "PASS" + colorReset
	}
	return colorRed + "FAIL" + colorReset
}

func (f *TextFormatter) printSummary(stats *checker.Stats) {
	fmt.Fprintf(os.Stdout, "\n%s--- Summary ---%s\n", colorCyan, colorReset)
	total := stats.Total()
	passed := stats.Passed()
	if total == passed {
		fmt.Fprintf(os.Stdout, "%sAll %d check(s) passed%s\n", colorGreen, total, colorReset)
	} else {
		failed := stats.Failed()
		errors := stats.Errors()
		fmt.Fprintf(os.Stdout, "%s%d passed%s, %s%d failed%s", colorGreen, passed, colorReset, colorRed, failed, colorReset)
		if errors > 0 {
			fmt.Fprintf(os.Stdout, ", %s%d error(s)%s", colorRed, errors, colorReset)
		}
		fmt.Fprintln(os.Stdout)
	}
	if stats.Warnings() > 0 {
		fmt.Fprintf(os.Stdout, "%s%d warning(s)%s\n", colorYellow, stats.Warnings(), colorReset)
	}
	if stats.Info() > 0 {
		fmt.Fprintf(os.Stdout, "%s%d info(s)%s\n", colorWhite, stats.Info(), colorReset)
	}
}
