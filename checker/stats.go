package checker

const (
	OK     = 0
	Config = 1 << 0 // 1
	Fail   = 1 << 1 // 2
	Warn   = 1 << 2 // 4
	Error  = 1 << 3 // 8
)

type Stats struct {
	Results []RunResult
}

func (s *Stats) AddRunResult(rr RunResult) {
	s.Results = append(s.Results, rr)
}

func (s *Stats) Total() int {
	n := 0
	for _, r := range s.Results {
		for _, res := range r.Results {
			if !res.Info && !res.Warning {
				n++
			}
		}
	}
	return n
}

func (s *Stats) Passed() int {
	n := 0
	for _, r := range s.Results {
		for _, res := range r.Results {
			if !res.Info && !res.Warning && res.Passed {
				n++
			}
		}
	}
	return n
}

func (s *Stats) Failed() int {
	n := 0
	for _, r := range s.Results {
		for _, res := range r.Results {
			if !res.Info && !res.Warning && !res.Passed && !res.Error {
				n++
			}
		}
	}
	return n
}

func (s *Stats) Errors() int {
	n := 0
	for _, r := range s.Results {
		for _, res := range r.Results {
			if res.Error {
				n++
			}
		}
	}
	return n
}

func (s *Stats) Warnings() int {
	n := 0
	for _, r := range s.Results {
		for _, res := range r.Results {
			if res.Warning {
				n++
			}
		}
	}
	return n
}

func (s *Stats) Info() int {
	n := 0
	for _, r := range s.Results {
		for _, res := range r.Results {
			if res.Info {
				n++
			}
		}
	}
	return n
}

func (s *Stats) Code() int {
	code := OK
	if s.Failed() > 0 {
		code |= Fail
	}
	if s.Errors() > 0 {
		code |= Error
	}
	if s.Warnings() > 0 {
		code |= Warn
	}
	return code
}

// HasLocalIPv4 reports whether the host has IPv4 connectivity,
// based on the host-ipv4 result stored in stats.
func (s *Stats) HasLocalIPv4() bool {
	return s.hasLocalIP("host-ipv4")
}

// HasLocalIPv6 reports whether the host has IPv6 connectivity,
// based on the host-ipv6 result stored in stats.
func (s *Stats) HasLocalIPv6() bool {
	return s.hasLocalIP("host-ipv6")
}

func (s *Stats) hasLocalIP(checkerName string) bool {
	for _, r := range s.Results {
		for _, res := range r.Results {
			if res.Checker == checkerName {
				return res.Passed
			}
		}
	}
	return false
}
