package teststats

import "http-tester/checker"

const (
	OK     = 0
	Config = 1 << 0 // 1
	Fail   = 1 << 1 // 2
	Warn   = 1 << 2 // 4
	Error  = 1 << 3 // 8
)

type Stats struct {
	Results  []checker.RunResult
	Total    int
	Passed   int
	Failed   int
	Errors   int
	Warnings int
	Info     int
}

func (s *Stats) AddRunResult(rr checker.RunResult) {
	s.Results = append(s.Results, rr)
	for _, res := range rr.Results {
		s.Add(res)
	}
}

func (s *Stats) Add(res *checker.Result) {
	if res.Info {
		s.Info++
		return
	}
	if res.Warning {
		s.Warnings++
		return
	}
	s.Total++
	if res.Error {
		s.Errors++
	} else if res.Passed {
		s.Passed++
	} else {
		s.Failed++
	}
}

func (s *Stats) Code() int {
	code := OK
	if s.Failed > 0 {
		code |= Fail
	}
	if s.Errors > 0 {
		code |= Error
	}
	if s.Warnings > 0 {
		code |= Warn
	}
	return code
}
