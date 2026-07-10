package dns

import (
	"context"
	"fmt"
	"strings"
	"time"

	"http-tester/checker"

	mdns "github.com/miekg/dns"
)

type HTTPSChecker struct {
	resolver *Resolver
}

func NewHTTPSChecker(resolver *Resolver) *HTTPSChecker {
	return &HTTPSChecker{resolver: resolver}
}

func (c *HTTPSChecker) Name() string {
	return "dns-https"
}

func (c *HTTPSChecker) Check(domain string) ([]*checker.Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := &checker.Result{
		Checker: "dns-https",
		Domain:  domain,
		Passed:  true,
	}

	resp, err := c.resolver.LookupHTTPS(ctx, domain)
	if err != nil {
		result.Passed = false
		result.Details = fmt.Sprintf("HTTPS lookup failed: %v", err)
		return []*checker.Result{result}, nil
	}

	if resp.Rcode != mdns.RcodeSuccess {
		result.Passed = false
		result.Details = "no HTTPS records"
		return []*checker.Result{result}, nil
	}

	for _, rr := range resp.Answer {
		if https, ok := rr.(*mdns.HTTPS); ok {
			svcParams := formatSVCB(https.Priority, https.Target, https.Value)
			result.Records = append(result.Records, checker.Record{
				Type:  "HTTPS",
				Value: svcParams,
			})
		}
	}

	if len(result.Records) == 0 {
		result.Passed = false
		result.Details = "no HTTPS records in response"
		return []*checker.Result{result}, nil
	}

	result.Details = fmt.Sprintf("found %d HTTPS record(s)", len(result.Records))
	return []*checker.Result{result}, nil
}

func formatSVCB(priority uint16, target string, params []mdns.SVCBKeyValue) string {
	s := fmt.Sprintf("priority=%d", priority)
	if target != "" {
		s += fmt.Sprintf(" target=%s", target)
	}
	for _, p := range params {
		s += fmt.Sprintf(" %s=%s", p.Key(), p.String())
	}
	return s
}

// HTTPSRecordInfo holds parsed information about a single HTTPS record
type HTTPSRecordInfo struct {
	Priority    uint16
	Target      string
	IsAliasMode bool // Priority == 0
	Alpn        []string
	HasAlpn     bool
	IPv4Hint    []string
	IPv6Hint    []string
	HasHint4    bool
	HasHint6    bool
	HasParams   bool
}

// parseHTTPSRecord extracts structured info from an HTTPS record
func parseHTTPSRecord(h *mdns.HTTPS) HTTPSRecordInfo {
	info := HTTPSRecordInfo{
		Priority:    h.Priority,
		Target:      h.Target,
		IsAliasMode: h.Priority == 0,
	}

	for _, v := range h.Value {
		info.HasParams = true
		switch key := v.Key(); key {
		case mdns.SVCB_ALPN:
			info.HasAlpn = true
			if alpnVal, ok := v.(*mdns.SVCBAlpn); ok {
				info.Alpn = append(info.Alpn, alpnVal.Alpn...)
			}
		case mdns.SVCB_IPV4HINT:
			info.HasHint4 = true
			if ipVal, ok := v.(*mdns.SVCBIPv4Hint); ok {
				for _, ip := range ipVal.Hint {
					info.IPv4Hint = append(info.IPv4Hint, ip.String())
				}
			}
		case mdns.SVCB_IPV6HINT:
			info.HasHint6 = true
			if ipVal, ok := v.(*mdns.SVCBIPv6Hint); ok {
				for _, ip := range ipVal.Hint {
					info.IPv6Hint = append(info.IPv6Hint, ip.String())
				}
			}
		}
	}
	return info
}

// ConsistencyCheck performs comprehensive HTTPS record validation per RFC 9460
func ConsistencyCheck(domain string, resolver *Resolver) ([]*checker.Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result := &checker.Result{
		Checker: "dns-consistency",
		Domain:  domain,
		Passed:  true,
	}

	// 1. Resolve A/AAAA for the domain
	a4s, a6s, err := resolveIPs(ctx, resolver, domain)
	if err != nil {
		result.Passed = false
		result.Details = fmt.Sprintf("base resolution failed: %v", err)
		return []*checker.Result{result}, nil
	}
	if len(a4s) == 0 && len(a6s) == 0 {
		result.Passed = false
		result.Details = "no A/AAAA records for domain"
		return []*checker.Result{result}, nil
	}

	// 2. Fetch HTTPS records
	resp, err := resolver.LookupHTTPS(ctx, domain)
	if err != nil {
		result.Passed = false
		result.Details = fmt.Sprintf("HTTPS lookup failed: %v", err)
		return []*checker.Result{result}, nil
	}

	if resp.Rcode != mdns.RcodeSuccess {
		result.Details = "no HTTPS records, consistency check skipped"
		return []*checker.Result{result}, nil
	}

	var httpsRecords []*mdns.HTTPS
	for _, rr := range resp.Answer {
		if h, ok := rr.(*mdns.HTTPS); ok {
			httpsRecords = append(httpsRecords, h)
		}
	}

	if len(httpsRecords) == 0 {
		result.Details = "no HTTPS records, consistency check skipped"
		return []*checker.Result{result}, nil
	}

	// 3. Analyze each HTTPS record — return separate results
	var results []*checker.Result

	for i, h := range httpsRecords {
		rec := parseHTTPSRecord(h)

		recResult := &checker.Result{
			Checker: "dns-consistency",
			Domain:  domain,
			Passed:  true,
		}

		if rec.IsAliasMode {
			rr := analyzeAliasMode(domain, &rec, resolver, ctx)
			applyResult(recResult, fmt.Sprintf("record #%d (AliasMode, priority=%d)", i+1, rec.Priority), rr)
		} else {
			rr := analyzeServiceMode(domain, &rec, a4s, a6s)
			applyResult(recResult, fmt.Sprintf("record #%d (ServiceMode, priority=%d)", i+1, rec.Priority), rr)
		}

		results = append(results, recResult)
	}

	return results, nil
}

// applyResult sets the result fields based on analyzeResult
func applyResult(r *checker.Result, prefix string, ar analyzeResult) {
	if len(ar.errors) > 0 {
		r.Passed = false
		r.Details = prefix + ": " + strings.Join(ar.errors, "; ")
	} else if len(ar.warnings) > 0 {
		r.Details = prefix + ": " + strings.Join(ar.warnings, "; ")
	} else if len(ar.info) > 0 {
		r.Details = prefix + ": " + strings.Join(ar.info, "; ")
	} else {
		r.Details = prefix + ": OK"
	}
}

type analyzeResult struct {
	warnings []string
	errors   []string
	info     []string
}

// analyzeAliasMode checks a Priority=0 HTTPS record
func analyzeAliasMode(domain string, rec *HTTPSRecordInfo, resolver *Resolver, ctx context.Context) analyzeResult {
	r := analyzeResult{}

	// Target must be a valid domain, not '.'
	if rec.Target == "" || rec.Target == "." {
		r.errors = append(r.errors, "empty or '.' target in AliasMode")
		return r
	}

	r.info = append(r.info, fmt.Sprintf("target → %s", rec.Target))

	// Parameters should not exist in AliasMode
	if rec.HasParams {
		r.warnings = append(r.warnings, "parameters present — will be ignored by browsers")
	}

	// Follow: query HTTPS record for the Target domain
	targetRecords := queryHTTPS(ctx, resolver, rec.Target)
	if len(targetRecords) == 0 {
		r.warnings = append(r.warnings, fmt.Sprintf("target %s has no HTTPS records", rec.Target))
	} else {
		r.info = append(r.info, fmt.Sprintf("target resolved: %d HTTPS record(s)", len(targetRecords)))
	}

	return r
}

// analyzeServiceMode checks a Priority>0 HTTPS record
func analyzeServiceMode(domain string, rec *HTTPSRecordInfo, a4s, a6s []string) analyzeResult {
	r := analyzeResult{}

	// Target
	if rec.Target == "" || rec.Target == "." {
		r.info = append(r.info, "target: self (same domain)")
	} else {
		r.info = append(r.info, fmt.Sprintf("target: %s", rec.Target))
	}

	// alpn
	if !rec.HasAlpn {
		r.warnings = append(r.warnings, "missing mandatory 'alpn' parameter")
	} else {
		var proto []string
		for _, a := range rec.Alpn {
			if a == "h2" || a == "h3" || strings.HasPrefix(a, "h3-") {
				proto = append(proto, a)
			}
		}
		if len(proto) > 0 {
			r.info = append(r.info, fmt.Sprintf("alpn: %s", strings.Join(proto, ", ")))
		}
	}

	// Hints
	if rec.HasHint4 {
		overlap := hasOverlap(a4s, rec.IPv4Hint) || hasOverlap(a6s, rec.IPv4Hint)
		if overlap {
			r.info = append(r.info, fmt.Sprintf("ipv4hint: %v ✓", rec.IPv4Hint))
		} else {
			r.warnings = append(r.warnings, fmt.Sprintf("ipv4hint %v not found in A/AAAA records", rec.IPv4Hint))
		}
	}
	if rec.HasHint6 {
		overlap := hasOverlap(a6s, rec.IPv6Hint) || hasOverlap(a4s, rec.IPv6Hint)
		if overlap {
			r.info = append(r.info, fmt.Sprintf("ipv6hint: %v ✓", rec.IPv6Hint))
		} else {
			r.warnings = append(r.warnings, fmt.Sprintf("ipv6hint %v not found in A/AAAA records", rec.IPv6Hint))
		}
	}

	return r
}

// queryHTTPS fetches HTTPS records for a domain (used for AliasMode following)
func queryHTTPS(ctx context.Context, resolver *Resolver, domain string) []*mdns.HTTPS {
	resp, err := resolver.LookupHTTPS(ctx, domain)
	if err != nil {
		return nil
	}

	var records []*mdns.HTTPS
	for _, rr := range resp.Answer {
		if h, ok := rr.(*mdns.HTTPS); ok {
			records = append(records, h)
		}
	}
	return records
}

func resolveIPs(ctx context.Context, resolver *Resolver, domain string) (ipv4s, ipv6s []string, err error) {
	addrs, err := resolver.LookupIPAddr(ctx, domain)
	if err != nil {
		return nil, nil, err
	}
	for _, a := range addrs {
		if a.IP.To4() != nil {
			ipv4s = append(ipv4s, a.IP.String())
		} else {
			ipv6s = append(ipv6s, a.IP.String())
		}
	}
	return ipv4s, ipv6s, nil
}

func hasOverlap(a, b []string) bool {
	set := make(map[string]bool, len(a))
	for _, s := range a {
		set[s] = true
	}
	for _, s := range b {
		if set[s] {
			return true
		}
	}
	return false
}

// HasHTTPSRecord checks if domain has HTTPS DNS record.
func HasHTTPSRecord(resolver *Resolver, domain string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := resolver.LookupHTTPS(ctx, domain)
	if err != nil {
		return false
	}

	if resp.Rcode != mdns.RcodeSuccess {
		return false
	}

	for _, rr := range resp.Answer {
		if _, ok := rr.(*mdns.HTTPS); ok {
			return true
		}
	}
	return false
}
