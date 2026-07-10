package dns

import (
	"context"
	"net"
	"os"
	"strings"
	"time"

	mdns "github.com/miekg/dns"
)

// Resolver is a DNS resolution service.
// By default it uses Go standard library functions (net.Resolver).
// When an explicit resolver address is provided, it uses that server.
type Resolver struct {
	// system is the standard Go resolver (uses OS DNS config).
	system *net.Resolver

	// server is the explicit DNS server address for miekg/dns queries.
	// Empty when using system resolver.
	server string

	// client is the miekg/dns client, reused across queries.
	client *mdns.Client
}

// NewResolver creates a resolver.
// If addr is empty, uses system resolver for A/AAAA and detects the system
// nameserver for HTTPS (type 65) queries that require miekg/dns.
// If addr is provided (e.g. "8.8.8.8:53"), uses that server for all queries.
func NewResolver(addr string) *Resolver {
	r := &Resolver{
		system: &net.Resolver{PreferGo: true},
		client: &mdns.Client{Timeout: 5 * time.Second},
	}

	if addr != "" {
		r.server = addr
	} else {
		r.server = detectSystemNameserver()
	}

	return r
}

// detectSystemNameserver finds the first nameserver from /etc/resolv.conf,
// falling back to 8.8.8.8:53 if unavailable.
func detectSystemNameserver() string {
	data, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		return "8.8.8.8:53"
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "nameserver") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				ns := fields[1]
				if strings.Contains(ns, ":") && !strings.HasPrefix(ns, "[") {
					ns = "[" + ns + "]"
				}
				if !strings.Contains(ns, ":") {
					ns = ns + ":53"
				}
				return ns
			}
		}
	}
	return "8.8.8.8:53"
}

// LookupIPAddr resolves A/AAAA records using Go standard library.
func (r *Resolver) LookupIPAddr(ctx context.Context, domain string) ([]net.IPAddr, error) {
	return r.system.LookupIPAddr(ctx, domain)
}

// LookupHTTPS sends a DNS HTTPS (type 65) query via miekg/dns.
func (r *Resolver) LookupHTTPS(ctx context.Context, domain string) (*mdns.Msg, error) {
	m := new(mdns.Msg)
	m.SetQuestion(mdns.Fqdn(domain), mdns.TypeHTTPS)
	m.RecursionDesired = true
	resp, _, err := r.client.ExchangeContext(ctx, m, r.server)
	return resp, err
}

// Server returns the DNS server address used for miekg/dns queries.
func (r *Resolver) Server() string {
	return r.server
}
