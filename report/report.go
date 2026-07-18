package report

import (
	"fmt"

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
