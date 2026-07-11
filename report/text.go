package report

import (
	"fmt"
	"os"
	"time"

	"http-tester/checker"
)

type TextFormatter struct{}

func (f *TextFormatter) Print(stats *checker.Stats, resolver string, startTime time.Time) int {
	if resolver != "" {
		fmt.Fprintf(os.Stdout, "\n%sResolver%s: %s\n", colorCyan, colorReset, resolver)
	}

	for _, r := range stats.Results {
		fmt.Fprintf(os.Stdout, "\n%s===%s %s %s===%s\n", colorCyan, colorReset, r.Domain, colorCyan, colorReset)
		for _, res := range r.Results {
			var status string
			if res.Info {
				status = colorWhite + "INFO" + colorReset
			} else if res.Warning {
				status = colorYellow + "WARN" + colorReset
			} else if res.Error {
				status = colorRed + "ERROR" + colorReset
			} else if res.Passed {
				status = colorGreen + "PASS" + colorReset
			} else {
				status = colorRed + "FAIL" + colorReset
			}
			fmt.Fprintf(os.Stdout, "  [%s] %s: %s\n", status, res.Checker, res.Details)
			if res.RedirectTo != "" {
				fmt.Fprintf(os.Stdout, "         -> %s\n", res.RedirectTo)
			}
			if res.AltSvc != "" {
				fmt.Fprintf(os.Stdout, "         Alt-Svc header found: %s\n", res.AltSvc)
			}
			for _, rec := range res.Records {
				fmt.Fprintf(os.Stdout, "         %s %s\n", rec.Type, rec.Value)
			}
		}
	}

	fmt.Fprintf(os.Stdout, "\n%s--- Summary ---\n", colorCyan)
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

	return stats.Code()
}
