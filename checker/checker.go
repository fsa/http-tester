package checker

type Record struct {
	Type  string
	Value string
	TTL   uint32
}

type Result struct {
	Checker string
	Domain  string
	Passed  bool
	Details string
	Records []Record
}

type Checker interface {
	Name() string
	Check(domain string) (*Result, error)
}

type RunResult struct {
	Domain  string
	Results []*Result
}
