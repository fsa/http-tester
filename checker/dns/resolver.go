package dns

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	mdns "github.com/miekg/dns"
)

// Resolver resolves DNS records.
// Without an address: uses the system resolver (net.Resolver) like any Go program.
// With an address: queries the specified server directly.
type Resolver struct {
	server string // "host:port" or "" for system
	client *mdns.Client
}

// NewResolver creates a resolver.
// addr is an optional DNS server address (IPv4 or IPv6, with or without brackets).
// If a port is included, it's used; otherwise defaults to 53.
func NewResolver(addr string) *Resolver {
	r := &Resolver{
		client: &mdns.Client{Timeout: 5 * time.Second},
	}
	if addr != "" {
		r.server = normalizeAddr(addr)
	}
	return r
}

// normalizeAddr parses a resolver address: strips brackets, adds port 53 if missing.
func normalizeAddr(addr string) string {
	// If it already has a port, return as-is
	if _, _, err := net.SplitHostPort(addr); err == nil {
		return addr
	}
	// No port — add :53
	host := strings.Trim(addr, "[]")
	return net.JoinHostPort(host, "53")
}

// LookupIPAddr resolves A/AAAA records.
// System mode: uses net.Resolver (reads /etc/resolv.conf automatically).
// Server mode: queries the specified server via miekg/dns.
func (r *Resolver) LookupIPAddr(ctx context.Context, domain string) ([]net.IPAddr, error) {
	if r.server == "" {
		return (&net.Resolver{}).LookupIPAddr(ctx, domain)
	}
	return r.lookupIPViaServer(ctx, domain)
}

// lookupIPViaServer resolves A/AAAA via a specific server using miekg/dns.
func (r *Resolver) lookupIPViaServer(ctx context.Context, domain string) ([]net.IPAddr, error) {
	var addrs []net.IPAddr

	for _, qtype := range []uint16{mdns.TypeA, mdns.TypeAAAA} {
		m := new(mdns.Msg)
		m.SetQuestion(mdns.Fqdn(domain), qtype)
		m.RecursionDesired = true

		resp, _, err := r.client.ExchangeContext(ctx, m, r.server)
		if err != nil {
			continue
		}
		for _, rr := range resp.Answer {
			switch v := rr.(type) {
			case *mdns.A:
				addrs = append(addrs, net.IPAddr{IP: v.A})
			case *mdns.AAAA:
				addrs = append(addrs, net.IPAddr{IP: v.AAAA})
			}
		}
	}

	if len(addrs) == 0 {
		return nil, fmt.Errorf("no A/AAAA records for %s", domain)
	}
	return addrs, nil
}

// LookupHTTPS sends a DNS HTTPS (type 65) query.
// System mode: uses the first nameserver from /etc/resolv.conf via miekg/dns
// (Go's net.Resolver does not support type 65).
// Server mode: queries the specified server directly.
func (r *Resolver) LookupHTTPS(ctx context.Context, domain string) (*mdns.Msg, error) {
	m := new(mdns.Msg)
	m.SetQuestion(mdns.Fqdn(domain), mdns.TypeHTTPS)
	m.RecursionDesired = true

	server := r.server
	if server == "" {
		server = systemNameserver()
	}

	resp, _, err := r.client.ExchangeContext(ctx, m, server)
	return resp, err
}

// systemNameserver returns the first nameserver from /etc/resolv.conf.
func systemNameserver() string {
	data, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		return "127.0.0.1:53"
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "nameserver") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		ns := fields[1]
		if strings.Contains(ns, ":") && !strings.HasPrefix(ns, "[") {
			ns = "[" + ns + "]"
		}
		if !strings.Contains(ns, ":") {
			ns = ns + ":53"
		}
		return ns
	}
	return "127.0.0.1:53"
}

// Server returns the DNS server address used for queries.
func (r *Resolver) Server() string {
	if r.server == "" {
		return "system"
	}
	return r.server
}
