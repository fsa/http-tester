package checker

type Record struct {
	Type  string
	Value string
	TTL   uint32
}

type Result struct {
	Checker     string
	Domain      string
	Passed      bool
	Warning     bool
	Info        bool
	Error       bool // test could not be completed (resolver unreachable, timeout)
	Details     string
	Records     []Record
	HTTPVersion string
	AltSvc      string
	RedirectTo  string
	Body        []byte // normalized response body for consistency check
}

type Checker interface {
	Name() string
	Check(domain string) ([]*Result, error)
}

type RunResult struct {
	Domain  string
	Results []*Result
}
