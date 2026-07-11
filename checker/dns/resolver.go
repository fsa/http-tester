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

// NewResolver creates a resolver and verifies it is reachable.
// addr is an optional DNS server address (IPv4 or IPv6, with or without brackets).
// If a port is included, it's used; otherwise defaults to 53.
// If addr is empty, the first nameserver from /etc/resolv.conf is used.
// Returns an error if the address is invalid or the server is unreachable.
func NewResolver(addr string) (*Resolver, error) {
	r := &Resolver{
		client: &mdns.Client{Timeout: 5 * time.Second},
	}
	if addr != "" {
		parsed, err := parseAddr(addr)
		if err != nil {
			return nil, err
		}
		r.server = parsed
	} else {
		r.server = systemNameserver()
	}

	// Verify the resolver is reachable
	if err := r.probe(); err != nil {
		return nil, fmt.Errorf("resolver %s: %w", r.server, err)
	}

	return r, nil
}

// parseAddr validates and normalizes a resolver address.
// Returns "host:port" or an error if the address is invalid.
func parseAddr(addr string) (string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		// No port — treat entire string as host, default port 53
		host = strings.Trim(addr, "[]")
		if net.ParseIP(host) == nil {
			return "", fmt.Errorf("invalid resolver address: %q is not a valid IP address", addr)
		}
		return net.JoinHostPort(host, "53"), nil
	}

	// Has port — validate both host and port
	if net.ParseIP(host) == nil {
		return "", fmt.Errorf("invalid resolver address: %q is not a valid IP address", host)
	}
	portNum, err := net.LookupPort("udp", port)
	if err != nil || portNum <= 0 || portNum > 65535 {
		return "", fmt.Errorf("invalid resolver port: %q", port)
	}
	return net.JoinHostPort(host, port), nil
}

// probe sends a DNS query to verify the resolver can respond.
func (r *Resolver) probe() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	m := new(mdns.Msg)
	m.SetQuestion("example.com.", mdns.TypeA)
	m.RecursionDesired = true

	resp, _, err := r.client.ExchangeContext(ctx, m, r.server)
	if err != nil {
		return err
	}
	if resp == nil {
		return fmt.Errorf("no response")
	}
	if resp.Rcode != mdns.RcodeSuccess {
		return fmt.Errorf("unexpected rcode: %d", resp.Rcode)
	}
	return nil
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
