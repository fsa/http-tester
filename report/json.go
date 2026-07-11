package report

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"http-tester/checker"
)

type JSONFormatter struct {
	Pretty bool
}

func (f *JSONFormatter) Print(stats *checker.Stats, resolver string, startTime time.Time) int {
	report := buildReport(stats, resolver, startTime)

	var data []byte
	if f.Pretty {
		data, _ = json.MarshalIndent(report, "", "  ")
	} else {
		data, _ = json.Marshal(report)
	}
	fmt.Fprintln(os.Stdout, string(data))

	return stats.Code()
}
