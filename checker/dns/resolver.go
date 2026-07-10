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
// By default it uses the system resolver (net.Resolver) for all query types.
// When an explicit resolver address is provided, uses that server for all queries.
type Resolver struct {
	system *net.Resolver
	server string // empty = use system resolver
	client *mdns.Client
}

// NewResolver creates a resolver.
// When addr is empty, the system resolver is used for all queries.
// When addr is provided, that server is used for all queries.
func NewResolver(addr string) *Resolver {
	r := &Resolver{
		system: &net.Resolver{PreferGo: true},
		client: &mdns.Client{Timeout: 5 * time.Second},
	}
	if addr != "" {
		r.server = addr
	}
	return r
}

// detectNameserver finds a DNS server that miekg/dns can reach.
//
// On IPv6-only hosts with NAT64/DNS64, external nameservers from
// resolv.conf (e.g. 1.1.1.1) are unreachable — the system resolver
// synthesizes AAAA via DNS64, but miekg/dns connects to the raw IP.
// Local resolvers (systemd-resolved, dnsmasq) handle DNS64 transparently.
func detectNameserver() string {
	// 1. Local resolvers first — they handle DNS64/DNSSEC
	for _, local := range []string{"127.0.0.53:53", "127.0.0.1:53", "[::1]:53"} {
		if probeUDP(local) {
			return local
		}
	}

	// 2. Nameservers from resolv.conf — only if directly reachable
	for _, ns := range resolvConfNameservers() {
		if probeUDP(ns) {
			return ns
		}
	}

	// 3. Nothing reachable — return local, best effort
	return "127.0.0.1:53"
}

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

// LookupHTTPS sends a DNS HTTPS (type 65) query.
// When an explicit resolver is configured, queries it directly via miekg/dns.
// Otherwise uses the system resolver, falling back to detected nameservers
// if the system resolver cannot handle type 65 queries.
func (r *Resolver) LookupHTTPS(ctx context.Context, domain string) (*mdns.Msg, error) {
	m := new(mdns.Msg)
	m.SetQuestion(mdns.Fqdn(domain), mdns.TypeHTTPS)
	m.RecursionDesired = true

	if r.server != "" {
		resp, _, err := r.client.ExchangeContext(ctx, m, r.server)
		return resp, err
	}

	// System resolver path: try a regular lookup first to confirm reachability,
	// then send the type 65 query via miekg/dns through detected nameservers.
	// Go's net.Resolver does not support arbitrary DNS types.
	_, err := r.system.LookupIPAddr(ctx, domain)
	if err == nil {
		server := detectNameserver()
		resp, _, err := r.client.ExchangeContext(ctx, m, server)
		return resp, err
	}

	// System resolver unreachable — fall back entirely to detected nameserver
	server := detectNameserver()
	resp, _, err := r.client.ExchangeContext(ctx, m, server)
	return resp, err
}

// Server returns the DNS server address used for queries.
// Returns "system" when using the system resolver (no explicit address).
func (r *Resolver) Server() string {
	if r.server == "" {
		return "system"
	}
	return r.server
}
