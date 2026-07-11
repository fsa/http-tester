package dns

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	mdns "github.com/miekg/dns"
)

// Resolver resolves DNS records via miekg/dns.
// Without an address: queries the first nameserver from /etc/resolv.conf.
// With an address: queries the specified server directly.
type Resolver struct {
	server string // "host:port"
	client *mdns.Client
}

// NewResolver creates a resolver.
// addr is an optional DNS server address (IPv4 or IPv6, with or without brackets).
// If a port is included, it's used; otherwise defaults to 53.
// If addr is empty, the first nameserver from /etc/resolv.conf is used.
func NewResolver(addr string) *Resolver {
	r := &Resolver{
		client: &mdns.Client{Timeout: 5 * time.Second},
	}
	if addr != "" {
		r.server = normalizeAddr(addr)
	} else {
		r.server = systemNameserver()
	}
	return r
}

// normalizeAddr parses a resolver address: strips brackets, adds port 53 if missing.
func normalizeAddr(addr string) string {
	if _, _, err := net.SplitHostPort(addr); err == nil {
		return addr
	}
	host := strings.Trim(addr, "[]")
	return net.JoinHostPort(host, "53")
}

// systemNameserver returns the first nameserver from /etc/resolv.conf via miekg/dns.
func systemNameserver() string {
	cfg, err := mdns.ClientConfigFromFile("/etc/resolv.conf")
	if err != nil || len(cfg.Servers) == 0 {
		return "127.0.0.1:53"
	}
	return net.JoinHostPort(cfg.Servers[0], cfg.Port)
}

// LookupIPAddr resolves A/AAAA records.
func (r *Resolver) LookupIPAddr(ctx context.Context, domain string) ([]net.IPAddr, error) {
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
func (r *Resolver) LookupHTTPS(ctx context.Context, domain string) (*mdns.Msg, error) {
	m := new(mdns.Msg)
	m.SetQuestion(mdns.Fqdn(domain), mdns.TypeHTTPS)
	m.RecursionDesired = true

	resp, _, err := r.client.ExchangeContext(ctx, m, r.server)
	return resp, err
}

// Server returns the DNS server address used for queries.
func (r *Resolver) Server() string {
	return r.server
}
