package report

import (
	"fmt"
	"time"

	"http-tester/checker"
	"http-tester/config"
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
	Group       string       `json:"group" yaml:"group"`
	Tags        []string     `json:"tags,omitempty" yaml:"tags,omitempty"`
	Passed      bool         `json:"passed" yaml:"passed"`
	Warning     bool         `json:"warning,omitempty" yaml:"warning,omitempty"`
	Info        bool         `json:"info,omitempty" yaml:"info,omitempty"`
	Error       bool         `json:"error,omitempty" yaml:"error,omitempty"`
	Details     string       `json:"details" yaml:"details"`
	Records     []JSONRecord `json:"records,omitempty" yaml:"records,omitempty"`
	HTTPVersion string       `json:"http_version,omitempty" yaml:"http_version,omitempty"`
	AltSvc      string       `json:"alt_svc,omitempty" yaml:"alt_svc,omitempty"`
	RedirectTo  string       `json:"redirect_to,omitempty" yaml:"redirect_to,omitempty"`
	ServerAddr  string       `json:"server_addr,omitempty" yaml:"server_addr,omitempty"`
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

type Formatter interface {
	Start(cfg *config.DomainConfig)
	Print(stats *checker.Stats, resolver string) int
}

var formatters = map[string]Formatter{
	"text":        &TextFormatter{},
	"json":        &JSONFormatter{Pretty: false},
	"json-pretty": &JSONFormatter{Pretty: true},
	"json_pretty": &JSONFormatter{Pretty: true},
	"json-verbose": &JSONFormatter{Pretty: true},
	"yaml":        &YAMLFormatter{},
	"yml":         &YAMLFormatter{},
}

func Print(format string, stats *checker.Stats, cfg *config.DomainConfig, resolver string) error {
	f, ok := formatters[format]
	if !ok {
		return fmt.Errorf("unknown format: %q (available: text, json, json-pretty, yaml)", format)
	}
	f.Print(stats, resolver)
	return nil
}

func Start(format string, cfg *config.DomainConfig) {
	f, ok := formatters[format]
	if !ok {
		return
	}
	f.Start(cfg)
}

func buildReport(stats *checker.Stats, resolver string) JSONReport {
	report := JSONReport{
		Timestamp: time.Now().Format(time.RFC3339),
		Resolver:  resolver,
	}

	for _, r := range stats.Results {
		domain := JSONDomain{Name: r.Domain}
		for _, res := range r.Results {
			jr := JSONResult{
				Checker:     res.Checker,
				Group:       res.Group,
				Tags:        res.Tags,
				Passed:      res.Passed,
				Warning:     res.Warning,
				Info:        res.Info,
				Error:       res.Error,
				Details:     res.Details,
				HTTPVersion: res.HTTPVersion,
				AltSvc:      res.AltSvc,
				RedirectTo:  res.RedirectTo,
				ServerAddr:  res.ServerAddr,
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
		Total:    stats.Total(),
		Passed:   stats.Passed(),
		Failed:   stats.Failed(),
		Errors:   stats.Errors(),
		Warnings: stats.Warnings(),
		Info:     stats.Info(),
		ExitCode: stats.Code(),
	}

	return report
}
