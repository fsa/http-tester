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

	// server is the DNS server address for miekg/dns queries (type 65).
	server string

	// client is the miekg/dns client, reused across queries.
	client *mdns.Client
}

// NewResolver creates a resolver.
// If addr is empty, uses system resolver for A/AAAA and detects a working
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

// detectSystemNameserver finds a working DNS server for miekg/dns queries.
//
// On IPv6-only hosts with NAT64/DNS64, the nameserver from /etc/resolv.conf
// (e.g. 1.1.1.1) is not directly reachable — the system resolver synthesizes
// AAAA records via DNS64, but miekg/dns connects to the raw IP and can't do
// that. In this case we fall back to 127.0.0.1 (local resolver like
// systemd-resolved or dnsmasq) which handles DNS64 transparently.
func detectSystemNameserver() string {
	// Try nameservers from resolv.conf first
	for _, ns := range resolvConfNameservers() {
		if probeUDP(ns) {
			return ns
		}
	}

	// None reachable — likely IPv6-only with NAT64. Try local resolver.
	for _, fallback := range []string{"127.0.0.1:53", "[::1]:53"} {
		if probeUDP(fallback) {
			return fallback
		}
	}

	// Last resort
	return "127.0.0.1:53"
}

// resolvConfNameservers reads nameserver entries from /etc/resolv.conf.
func resolvConfNameservers() []string {
	data, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		return nil
	}
	var servers []string
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
		servers = append(servers, ns)
	}
	return servers
}

// probeUDP checks if a DNS server is reachable via UDP with a short timeout.
func probeUDP(addr string) bool {
	conn, err := net.DialTimeout("udp", addr, 500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
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
