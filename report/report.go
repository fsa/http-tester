package report

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"http-tester/checker"
)

const (
	colorGreen = "\033[32m"
	colorRed   = "\033[31m"
	colorYellow = "\033[33m"
	colorReset = "\033[0m"
	colorCyan  = "\033[36m"
)

type JSONReport struct {
	Domains []JSONDomain `json:"domains"`
	Summary JSONSummary  `json:"summary"`
}

type JSONDomain struct {
	Name    string         `json:"name"`
	Results []JSONResult   `json:"results"`
}

type JSONResult struct {
	Checker     string         `json:"checker"`
	Passed      bool           `json:"passed"`
	Warning     bool           `json:"warning,omitempty"`
	Details     string         `json:"details"`
	Records     []JSONRecord   `json:"records,omitempty"`
	HTTPVersion string         `json:"http_version,omitempty"`
	AltSvc      string         `json:"alt_svc,omitempty"`
	RedirectTo  string         `json:"redirect_to,omitempty"`
}

type JSONRecord struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type JSONSummary struct {
	Total    int `json:"total"`
	Passed   int `json:"passed"`
	Failed   int `json:"failed"`
	Warnings int `json:"warnings"`
}

func Print(results []checker.RunResult, format string) int {
	switch format {
	case "json":
		return printJSON(results)
	default:
		return printText(results)
	}
}

func printText(results []checker.RunResult) int {
	total := 0
	passed := 0
	warnings := 0

	for _, r := range results {
		fmt.Fprintf(os.Stdout, "\n%s=== %s ===%s\n", colorCyan, r.Domain, colorReset)
		for _, res := range r.Results {
			var status string
			if res.Warning {
				status = colorYellow + "WARN" + colorReset
				warnings++
			} else {
				total++
				if res.Passed {
					status = colorGreen + "PASS" + colorReset
					passed++
				} else {
					status = colorRed + "FAIL" + colorReset
				}
			}
			fmt.Fprintf(os.Stdout, "  [%s] %s: %s\n", status, res.Checker, res.Details)
			if res.RedirectTo != "" {
				fmt.Fprintf(os.Stdout, "         -> %s\n", res.RedirectTo)
			}
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
	if warnings > 0 {
		fmt.Fprintf(os.Stdout, "%s%d warning(s)%s\n", colorYellow, warnings, colorReset)
	}

	if passed < total {
		return 1
	}
	return 0
}

func printJSON(results []checker.RunResult) int {
	report := JSONReport{}
	total := 0
	passed := 0
	warnings := 0

	for _, r := range results {
		domain := JSONDomain{Name: r.Domain}
		for _, res := range r.Results {
			if res.Warning {
				warnings++
			} else {
				total++
				if res.Passed {
					passed++
				}
			}
			jr := JSONResult{
				Checker:     res.Checker,
				Passed:      res.Passed,
				Warning:     res.Warning,
				Details:     res.Details,
				HTTPVersion: res.HTTPVersion,
				AltSvc:      res.AltSvc,
				RedirectTo:  res.RedirectTo,
			}
			for _, rec := range res.Records {
				jr.Records = append(jr.Records, JSONRecord{
					Type:  rec.Type,
					Value: rec.Value,
				})
			}
			domain.Results = append(domain.Results, jr)
		}
		report.Domains = append(report.Domains, domain)
	}

	report.Summary = JSONSummary{
		Total:    total,
		Passed:   passed,
		Failed:   total - passed,
		Warnings: warnings,
	}

	data, _ := json.MarshalIndent(report, "", "  ")
	fmt.Fprintln(os.Stdout, string(data))

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
			if res.Warning {
				status = "WARN"
			} else if !res.Passed {
				status = "FAIL"
			}
			b.WriteString(fmt.Sprintf("  [%s] %s: %s\n", status, res.Checker, res.Details))
			if res.RedirectTo != "" {
				b.WriteString(fmt.Sprintf("         -> %s\n", res.RedirectTo))
			}
			for _, rec := range res.Records {
				b.WriteString(fmt.Sprintf("         %s %s\n", rec.Type, rec.Value))
			}
		}
	}
	return b.String()
}
