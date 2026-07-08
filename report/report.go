package report

import (
	"fmt"
	"os"
	"strings"

	"http-tester/checker"
)

const (
	colorGreen = "\033[32m"
	colorRed   = "\033[31m"
	colorReset = "\033[0m"
	colorCyan  = "\033[36m"
)

func Print(results []checker.RunResult) int {
	total := 0
	passed := 0

	for _, r := range results {
		fmt.Fprintf(os.Stdout, "\n%s=== %s ===%s\n", colorCyan, r.Domain, colorReset)
		for _, res := range r.Results {
			total++
			status := colorGreen + "PASS" + colorReset
			if !res.Passed {
				status = colorRed + "FAIL" + colorReset
			} else {
				passed++
			}
			fmt.Fprintf(os.Stdout, "  [%s] %s: %s\n", status, res.Checker, res.Details)
			for _, rec := range res.Records {
				fmt.Fprintf(os.Stdout, "         %s %s\n", rec.Type, rec.Value)
			}
		}
	}

	fmt.Fprintf(os.Stdout, "\n%s--- Summary ---\n", colorCyan)
	if total == passed {
		fmt.Fprintf(os.Stdout, "%sAll %d check(s) passed%s\n", colorGreen, total, colorReset)
	} else {
		failed := total - passed
		fmt.Fprintf(os.Stdout, "%s%d passed%s, %s%d failed%s\n", colorGreen, passed, colorReset, colorRed, failed, colorReset)
	}

	if passed < total {
		return 1
	}
	return 0
}

func FormatText(results []checker.RunResult) string {
	var b strings.Builder
	for _, r := range results {
		b.WriteString(fmt.Sprintf("\n=== %s ===\n", r.Domain))
		for _, res := range r.Results {
			status := "PASS"
			if !res.Passed {
				status = "FAIL"
			}
			b.WriteString(fmt.Sprintf("  [%s] %s: %s\n", status, res.Checker, res.Details))
			for _, rec := range res.Records {
				b.WriteString(fmt.Sprintf("         %s %s\n", rec.Type, rec.Value))
			}
		}
	}
	return b.String()
}
