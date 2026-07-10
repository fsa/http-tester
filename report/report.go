package report

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"http-tester/checker"

	"gopkg.in/yaml.v3"
)

const (
	colorGreen  = "\033[32m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorReset  = "\033[0m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
)

type JSONReport struct {
	Domains []JSONDomain `json:"domains" yaml:"domains"`
	Summary JSONSummary  `json:"summary" yaml:"summary"`
}

type JSONDomain struct {
	Name    string       `json:"name" yaml:"name"`
	Results []JSONResult `json:"results" yaml:"results"`
}

type JSONResult struct {
	Checker     string       `json:"checker" yaml:"checker"`
	Passed      bool         `json:"passed" yaml:"passed"`
	Warning     bool         `json:"warning,omitempty" yaml:"warning,omitempty"`
	Info        bool         `json:"info,omitempty" yaml:"info,omitempty"`
	Details     string       `json:"details" yaml:"details"`
	Records     []JSONRecord `json:"records,omitempty" yaml:"records,omitempty"`
	HTTPVersion string       `json:"http_version,omitempty" yaml:"http_version,omitempty"`
	AltSvc      string       `json:"alt_svc,omitempty" yaml:"alt_svc,omitempty"`
	RedirectTo  string       `json:"redirect_to,omitempty" yaml:"redirect_to,omitempty"`
}

type JSONRecord struct {
	Type  string `json:"type" yaml:"type"`
	Value string `json:"value" yaml:"value"`
}

type JSONSummary struct {
	Total    int `json:"total" yaml:"total"`
	Passed   int `json:"passed" yaml:"passed"`
	Failed   int `json:"failed" yaml:"failed"`
	Warnings int `json:"warnings" yaml:"warnings"`
	Info     int `json:"info" yaml:"info"`
}

func Print(results []checker.RunResult, format string) int {
	switch format {
	case "json":
		return printJSON(results, false)
	case "json-pretty", "json_pretty", "json-verbose":
		return printJSON(results, true)
	case "yaml", "yml":
		return printYAML(results)
	default:
		return printText(results)
	}
}

func printText(results []checker.RunResult) int {
	total := 0
	passed := 0
	warnings := 0
	infos := 0

	for _, r := range results {
		fmt.Fprintf(os.Stdout, "\n%s=== %s ===%s\n", colorCyan, r.Domain, colorReset)
		for _, res := range r.Results {
			var status string
			if res.Info {
				status = colorWhite + "INFO" + colorReset
				infos++
			} else if res.Warning {
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
			if res.AltSvc != "" {
				fmt.Fprintf(os.Stdout, "         Alt-Svc header found: %s\n", res.AltSvc)
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
	if infos > 0 {
		fmt.Fprintf(os.Stdout, "%s%d info(s)%s\n", colorWhite, infos, colorReset)
	}

	if passed < total {
		return 1
	}
	return 0
}

func buildReport(results []checker.RunResult) JSONReport {
	report := JSONReport{}
	total := 0
	passed := 0
	warnings := 0
	infos := 0

	for _, r := range results {
		domain := JSONDomain{Name: r.Domain}
		for _, res := range r.Results {
			if res.Info {
				infos++
			} else if res.Warning {
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
				Info:        res.Info,
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
		Info:     infos,
	}

	return report
}

func printJSON(results []checker.RunResult, pretty bool) int {
	report := buildReport(results)

	var data []byte
	if pretty {
		data, _ = json.MarshalIndent(report, "", "  ")
	} else {
		data, _ = json.Marshal(report)
	}
	fmt.Fprintln(os.Stdout, string(data))

	if report.Summary.Passed < report.Summary.Total {
		return 1
	}
	return 0
}

func printYAML(results []checker.RunResult) int {
	report := buildReport(results)

	data, err := yaml.Marshal(report)
	if err != nil {
		fmt.Fprintf(os.Stderr, "YAML marshal error: %v\n", err)
		return 1
	}
	fmt.Fprint(os.Stdout, string(data))

	if report.Summary.Passed < report.Summary.Total {
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
			if res.Info {
				status = "INFO"
			} else if res.Warning {
				status = "WARN"
			} else if !res.Passed {
				status = "FAIL"
			}
			b.WriteString(fmt.Sprintf("  [%s] %s: %s\n", status, res.Checker, res.Details))
			if res.RedirectTo != "" {
				b.WriteString(fmt.Sprintf("         -> %s\n", res.RedirectTo))
			}
			if res.AltSvc != "" {
				b.WriteString(fmt.Sprintf("         Alt-Svc header found: %s\n", res.AltSvc))
			}
			for _, rec := range res.Records {
				b.WriteString(fmt.Sprintf("         %s %s\n", rec.Type, rec.Value))
			}
		}
	}
	return b.String()
}
