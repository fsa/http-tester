package checker

type Record struct {
	Type  string
	Value string
	TTL   uint32
}

type Result struct {
	Checker     string
	Group       string   // "DNS", "HTTP", "HTTPS", "Consistency"
	Tags        []string // e.g. ["http2", "ipv4"] or ["ipv4", "1.2.3.4"]
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
	ServerAddr  string // actual server IP used for the request
	Body        []byte // normalized response body for consistency check
}


type RunResult struct {
	Domain  string
	Results []*Result
}
