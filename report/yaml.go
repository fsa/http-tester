package report

import (
	"fmt"
	"os"
	"time"

	"http-tester/checker"
	"http-tester/config"

	"gopkg.in/yaml.v3"
)

type YAMLFormatter struct{}

func (f *YAMLFormatter) Start(cfg *config.DomainConfig, startTime time.Time) {}

func (f *YAMLFormatter) Print(stats *checker.Stats, resolver string, startTime time.Time) int {
	report := buildReport(stats, resolver, startTime)

	data, err := yaml.Marshal(report)
	if err != nil {
		fmt.Fprintf(os.Stderr, "YAML marshal error: %v\n", err)
		return 1
	}
	fmt.Fprint(os.Stdout, string(data))

	return stats.Code()
}
