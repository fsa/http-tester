package report

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"http-tester/checker/teststats"

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
	Timestamp string       `json:"timestamp" yaml:"timestamp"`
	Resolver  string       `json:"resolver,omitempty" yaml:"resolver,omitempty"`
	Domains   []JSONDomain `json:"domains" yaml:"domains"`
	Summary   JSONSummary  `json:"summary" yaml:"summary"`
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
	Error       bool         `json:"error,omitempty" yaml:"error,omitempty"`
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
	Errors   int `json:"errors" yaml:"errors"`
	Warnings int `json:"warnings" yaml:"warnings"`
	Info     int `json:"info" yaml:"info"`
	ExitCode int `json:"exit_code" yaml:"exit_code"`
}

func Print(stats *teststats.Stats, resolver string, startTime time.Time, format string) int {
	switch format {
	case "json":
		return printJSON(stats, resolver, startTime, false)
	case "json-pretty", "json_pretty", "json-verbose":
		return printJSON(stats, resolver, startTime, true)
	case "yaml", "yml":
		return printYAML(stats, resolver, startTime)
	default:
		return printText(stats, resolver)
	}
}

func printText(stats *teststats.Stats, resolver string) int {
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
	if stats.Total == stats.Passed {
		fmt.Fprintf(os.Stdout, "%sAll %d check(s) passed%s\n", colorGreen, stats.Total, colorReset)
	} else {
		fmt.Fprintf(os.Stdout, "%s%d passed%s, %s%d failed%s", colorGreen, stats.Passed, colorReset, colorRed, stats.Failed, colorReset)
		if stats.Errors > 0 {
			fmt.Fprintf(os.Stdout, ", %s%d error(s)%s", colorRed, stats.Errors, colorReset)
		}
		fmt.Fprintln(os.Stdout)
	}
	if stats.Warnings > 0 {
		fmt.Fprintf(os.Stdout, "%s%d warning(s)%s\n", colorYellow, stats.Warnings, colorReset)
	}
	if stats.Info > 0 {
		fmt.Fprintf(os.Stdout, "%s%d info(s)%s\n", colorWhite, stats.Info, colorReset)
	}

	return stats.Code()
}

func buildReport(stats *teststats.Stats, resolver string, startTime time.Time) JSONReport {
	report := JSONReport{
		Timestamp: startTime.Format(time.RFC3339),
		Resolver:  resolver,
	}

	for _, r := range stats.Results {
		domain := JSONDomain{Name: r.Domain}
		for _, res := range r.Results {
			jr := JSONResult{
				Checker:     res.Checker,
				Passed:      res.Passed,
				Warning:     res.Warning,
				Info:        res.Info,
				Error:       res.Error,
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
		Total:    stats.Total,
		Passed:   stats.Passed,
		Failed:   stats.Failed,
		Errors:   stats.Errors,
		Warnings: stats.Warnings,
		Info:     stats.Info,
		ExitCode: stats.Code(),
	}

	return report
}

func printJSON(stats *teststats.Stats, resolver string, startTime time.Time, pretty bool) int {
	report := buildReport(stats, resolver, startTime)

	var data []byte
	if pretty {
		data, _ = json.MarshalIndent(report, "", "  ")
	} else {
		data, _ = json.Marshal(report)
	}
	fmt.Fprintln(os.Stdout, string(data))

	return report.Summary.ExitCode
}

func printYAML(stats *teststats.Stats, resolver string, startTime time.Time) int {
	report := buildReport(stats, resolver, startTime)

	data, err := yaml.Marshal(report)
	if err != nil {
		fmt.Fprintf(os.Stderr, "YAML marshal error: %v\n", err)
		return 1
	}
	fmt.Fprint(os.Stdout, string(data))

	return report.Summary.ExitCode
}
