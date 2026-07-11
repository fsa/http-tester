package report

import (
	"fmt"
	"os"
	"strings"
	"time"

	"http-tester/checker"
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

type testGroup struct {
	name string
}

var groups = []testGroup{
	{"DNS"},
	{"HTTP"},
	{"HTTPS"},
	{"Consistency"},
}

func (f *TextFormatter) Print(stats *checker.Stats, resolver string, startTime time.Time) int {
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
