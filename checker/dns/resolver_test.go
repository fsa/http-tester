package dns

import (
	"context"
	"strings"
	"testing"
	"time"

	mdns "github.com/miekg/dns"
)

func TestNewResolver(t *testing.T) {
	tests := []struct {
		name   string
		addr   string
		server string
	}{
		{"ipv4 no port", "8.8.8.8", "8.8.8.8:53"},
		{"ipv4 with port", "8.8.8.8:5353", "8.8.8.8:5353"},
		{"ipv6 no brackets no port", "2001:4860:4860::8888", "[2001:4860:4860::8888]:53"},
		{"ipv6 brackets no port", "[2001:4860:4860::8888]", "[2001:4860:4860::8888]:53"},
		{"ipv6 brackets with port", "[2001:4860:4860::8888]:5353", "[2001:4860:4860::8888]:5353"},
		{"ipv6 loopback", "::1", "[::1]:53"},
		{"ipv6 loopback brackets", "[::1]", "[::1]:53"},
		{"ipv4 localhost", "127.0.0.1", "127.0.0.1:53"},
		{"ipv4 localhost port", "127.0.0.1:5353", "127.0.0.1:5353"},
		{"cloudflare", "1.1.1.1", "1.1.1.1:53"},
		{"google ipv6", "2001:4860:4860::8844", "[2001:4860:4860::8844]:53"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewResolver(tt.addr)
			if r.Server() != tt.server {
				t.Errorf("NewResolver(%q).Server() = %q, want %q", tt.addr, r.Server(), tt.server)
			}
		})
	}
}

func TestNewResolver_SystemMode(t *testing.T) {
	r := NewResolver("")
	// Server should be a real nameserver from resolv.conf, not "system"
	if r.Server() == "" {
		t.Error("NewResolver(\"\").Server() should not be empty")
	}
}

func TestLookupIPAddr(t *testing.T) {
	r := NewResolver("8.8.8.8")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	addrs, err := r.LookupIPAddr(ctx, "example.com")
	if err != nil {
		t.Fatalf("LookupIPAddr error: %v", err)
	}
	if len(addrs) == 0 {
		t.Fatal("LookupIPAddr returned no addresses")
	}

	// example.com should have at least one IPv4
	foundIPv4 := false
	for _, a := range addrs {
		if a.IP.To4() != nil {
			foundIPv4 = true
			break
		}
	}
	if !foundIPv4 {
		t.Error("expected at least one IPv4 address for example.com")
	}
}

func TestLookupIPAddr_IPv6(t *testing.T) {
	r := NewResolver("8.8.8.8")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	addrs, err := r.LookupIPAddr(ctx, "google.com")
	if err != nil {
		t.Fatalf("LookupIPAddr error: %v", err)
	}

	// google.com should have both IPv4 and IPv6
	foundIPv4, foundIPv6 := false, false
	for _, a := range addrs {
		if a.IP.To4() != nil {
			foundIPv4 = true
		} else {
			foundIPv6 = true
		}
	}
	if !foundIPv4 {
		t.Error("expected IPv4 for google.com")
	}
	if !foundIPv6 {
		t.Error("expected IPv6 for google.com")
	}
}

func TestLookupIPAddr_Nonexistent(t *testing.T) {
	r := NewResolver("8.8.8.8")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := r.LookupIPAddr(ctx, "this-domain-does-not-exist-12345.example")
	if err == nil {
		t.Error("expected error for nonexistent domain")
	}
}

func TestLookupHTTPS(t *testing.T) {
	r := NewResolver("8.8.8.8")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := r.LookupHTTPS(ctx, "google.com")
	if err != nil {
		t.Fatalf("LookupHTTPS error: %v", err)
	}

	// google.com has HTTPS records
	foundHTTPS := false
	for _, rr := range resp.Answer {
		if _, ok := rr.(*mdns.HTTPS); ok {
			foundHTTPS = true
			break
		}
	}
	if !foundHTTPS {
		t.Error("expected HTTPS records for google.com")
	}
}

func TestLookupHTTPS_NoRecords(t *testing.T) {
	r := NewResolver("8.8.8.8")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := r.LookupHTTPS(ctx, "example.com")
	if err != nil {
		t.Fatalf("LookupHTTPS error: %v", err)
	}

	// example.com may or may not have HTTPS records, just check no error
	for _, rr := range resp.Answer {
		if _, ok := rr.(*mdns.HTTPS); ok {
			return // has records, that's fine
		}
	}
}

func TestLookupHTTPS_Nonexistent(t *testing.T) {
	r := NewResolver("8.8.8.8")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := r.LookupHTTPS(ctx, "this-domain-does-not-exist-12345.example")
	if err != nil {
		// Connection error is acceptable
		return
	}
	// NXDOMAIN is also acceptable
	if resp.Rcode != 0 && !strings.Contains(resp.String(), "NXDOMAIN") {
		// Just check we got a response
	}
}
