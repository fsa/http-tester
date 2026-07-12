package dns

import (
	"net"
	"testing"
	"time"

	"http-tester/checker"
	"http-tester/config"

	mdns "github.com/miekg/dns"
)

func startTestDNSServer(t *testing.T, zones map[string]fakeZone) (addr string, stop func()) {
	t.Helper()

	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	server := &mdns.Server{
		PacketConn: pc,
		Handler:    &fakeDNSHandler{zones: zones},
	}

	go server.ActivateAndServe()

	return pc.LocalAddr().String(), func() { server.Shutdown() }
}

func TestDNSChecker_Check(t *testing.T) {
	zones := map[string]fakeZone{
		"tavda.info": {
			A:    []string{"185.199.108.153", "185.199.109.153"},
			AAAA: []string{"2606:50c0:8000::153"},
		},
		"tavda.net": {
			A:    []string{"185.199.108.153"},
			AAAA: []string{"2606:50c0:8000::153", "2606:50c0:8001::153"},
		},
		"tavda.org": {
			A:    []string{"185.199.108.153"},
			AAAA: []string{"2606:50c0:8000::153"},
			HTTPS: []fakeHTTPSRecord{
				{Priority: 1, Target: ".", ALPN: []string{"h2", "h3"}},
			},
		},
	}

	addr, stop := startTestDNSServer(t, zones)
	defer stop()
	time.Sleep(10 * time.Millisecond)

	r := &Resolver{server: addr, client: &mdns.Client{Timeout: 5 * time.Second}}

	tests := []struct {
		name     string
		domain   string
		modeA    config.DNSRecordCheck
		modeAAAA config.DNSRecordCheck
		wantIPv4 int
		wantIPv6 int
	}{
		{"tavda.info optional", "tavda.info", config.DNSOptional, config.DNSOptional, 2, 1},
		{"tavda.net yes", "tavda.net", config.DNSYes, config.DNSYes, 1, 2},
		{"tavda.org yes", "tavda.org", config.DNSYes, config.DNSYes, 1, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := &checker.Stats{}
			c := &DNSChecker{resolver: r, A: tt.modeA, AAAA: tt.modeAAAA}
			result := c.Check(tt.domain, stats)

			if len(result.IPv4s) != tt.wantIPv4 {
				t.Errorf("len(IPv4s) = %d, want %d", len(result.IPv4s), tt.wantIPv4)
			}
			if len(result.IPv6s) != tt.wantIPv6 {
				t.Errorf("len(IPv6s) = %d, want %d", len(result.IPv6s), tt.wantIPv6)
			}
		})
	}
}

func TestHTTPSChecker_Check(t *testing.T) {
	zones := map[string]fakeZone{
		"tavda.info": {
			A:    []string{"185.199.108.153"},
			AAAA: []string{"2606:50c0:8000::153"},
		},
		"tavda.org": {
			A:    []string{"185.199.108.153"},
			AAAA: []string{"2606:50c0:8000::153"},
			HTTPS: []fakeHTTPSRecord{
				{Priority: 1, Target: ".", ALPN: []string{"h2", "h3"}},
			},
		},
	}

	addr, stop := startTestDNSServer(t, zones)
	defer stop()
	time.Sleep(10 * time.Millisecond)

	r := &Resolver{server: addr, client: &mdns.Client{Timeout: 5 * time.Second}}

	tests := []struct {
		name       string
		domain     string
		mode       config.DNSRecordCheck
		wantExists bool
	}{
		{"tavda.info optional", "tavda.info", config.DNSOptional, false},
		{"tavda.info yes", "tavda.info", config.DNSYes, false},
		{"tavda.org optional", "tavda.org", config.DNSOptional, true},
		{"tavda.org yes", "tavda.org", config.DNSYes, true},
		{"tavda.org no", "tavda.org", config.DNSNo, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := &checker.Stats{}
			c := &HTTPSChecker{resolver: r}
			exists := c.Check(tt.domain, tt.mode, stats)
			if exists != tt.wantExists {
				t.Errorf("exists = %v, want %v", exists, tt.wantExists)
			}
		})
	}
}

func TestConsistencyCheck(t *testing.T) {
	zones := map[string]fakeZone{
		"tavda.info": {
			A:    []string{"185.199.108.153"},
			AAAA: []string{"2606:50c0:8000::153"},
		},
		"tavda.org": {
			A:    []string{"185.199.108.153"},
			AAAA: []string{"2606:50c0:8000::153"},
			HTTPS: []fakeHTTPSRecord{
				{Priority: 1, Target: ".", ALPN: []string{"h2", "h3"}, IPv4Hint: []string{"185.199.108.153"}, IPv6Hint: []string{"2606:50c0:8000::153"}},
			},
		},
	}

	addr, stop := startTestDNSServer(t, zones)
	defer stop()
	time.Sleep(10 * time.Millisecond)

	r := &Resolver{server: addr, client: &mdns.Client{Timeout: 5 * time.Second}}

	tests := []struct {
		name    string
		domain  string
		wantMin int
	}{
		{"tavda.info no HTTPS", "tavda.info", 1},
		{"tavda.org has HTTPS", "tavda.org", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := &checker.Stats{}
			ConsistencyCheck(tt.domain, r, stats)
			if len(stats.Results) < tt.wantMin {
				t.Errorf("len(results) = %d, want >= %d", len(stats.Results), tt.wantMin)
			}
			for _, rr := range stats.Results {
				for _, res := range rr.Results {
					if res.Group != "DNS" {
						t.Errorf("Group = %q, want %q", res.Group, "DNS")
					}
				}
			}
		})
	}
}

func TestFullCheckFlow(t *testing.T) {
	zones := map[string]fakeZone{
		"tavda.info": {
			A:    []string{"185.199.108.153", "185.199.109.153"},
			AAAA: []string{"2606:50c0:8000::153"},
		},
	}

	addr, stop := startTestDNSServer(t, zones)
	defer stop()
	time.Sleep(10 * time.Millisecond)

	r := &Resolver{server: addr, client: &mdns.Client{Timeout: 5 * time.Second}}
	stats := &checker.Stats{}

	// DNS check
	dnsChecker := &DNSChecker{resolver: r, A: config.DNSOptional, AAAA: config.DNSOptional}
	dnsResult := dnsChecker.Check("tavda.info", stats)

	if !dnsResult.HasA {
		t.Error("expected HasA = true")
	}
	if !dnsResult.HasAAAA {
		t.Error("expected HasAAAA = true")
	}
	if len(dnsResult.IPv4s) != 2 {
		t.Errorf("len(IPv4s) = %d, want 2", len(dnsResult.IPv4s))
	}

	// HTTPS check
	httpsChecker := &HTTPSChecker{resolver: r}
	httpsExists := httpsChecker.Check("tavda.info", config.DNSOptional, stats)
	if httpsExists {
		t.Error("expected httpsExists = false")
	}

	// Consistency check
	ConsistencyCheck("tavda.info", r, stats)

	if len(stats.Results) == 0 {
		t.Error("expected at least one result")
	}

	for _, rr := range stats.Results {
		for _, res := range rr.Results {
			if res.Group != "DNS" {
				t.Errorf("Group = %q, want %q for %s", res.Group, "DNS", res.Checker)
			}
		}
	}
}
