package report

import (
	"fmt"
	"os"

	"http-tester/checker"
	"http-tester/config"

	"gopkg.in/yaml.v3"
)

type YAMLFormatter struct{}

func (f *YAMLFormatter) Start(cfg *config.DomainConfig) {}

func (f *YAMLFormatter) Print(stats *checker.Stats, resolver string) int {
	report := buildReport(stats, resolver)

	data, err := yaml.Marshal(report)
	if err != nil {
		fmt.Fprintf(os.Stderr, "YAML marshal error: %v\n", err)
		return 1
	}
	fmt.Fprint(os.Stdout, string(data))

	return stats.Code()
}
