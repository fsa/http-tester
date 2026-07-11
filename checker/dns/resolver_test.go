package dns

import "testing"

func TestNewResolver(t *testing.T) {
	tests := []struct {
		name   string
		addr   string
		server string
	}{
		// IPv4 without port
		{"ipv4 no port", "8.8.8.8", "8.8.8.8:53"},
		// IPv4 with port
		{"ipv4 with port", "8.8.8.8:5353", "8.8.8.8:5353"},
		// IPv6 without port, no brackets
		{"ipv6 no brackets no port", "2001:4860:4860::8888", "[2001:4860:4860::8888]:53"},
		// IPv6 with brackets, no port
		{"ipv6 brackets no port", "[2001:4860:4860::8888]", "[2001:4860:4860::8888]:53"},
		// IPv6 with brackets and port
		{"ipv6 brackets with port", "[2001:4860:4860::8888]:5353", "[2001:4860:4860::8888]:5353"},
		// IPv6 loopback
		{"ipv6 loopback", "::1", "[::1]:53"},
		// IPv6 loopback with brackets
		{"ipv6 loopback brackets", "[::1]", "[::1]:53"},
		// IPv4 localhost
		{"ipv4 localhost", "127.0.0.1", "127.0.0.1:53"},
		// IPv4 localhost with port
		{"ipv4 localhost port", "127.0.0.1:5353", "127.0.0.1:5353"},
		// Cloudflare DNS
		{"cloudflare", "1.1.1.1", "1.1.1.1:53"},
		// Google DNS IPv6
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
	if r.Server() != "system" {
		t.Errorf("NewResolver(\"\").Server() = %q, want %q", r.Server(), "system")
	}
}
