package runner

import (
	"testing"

	"http-tester/config"
)

func TestDomainConfigWebModes(t *testing.T) {
	tests := []struct {
		name            string
		cfg             *config.DomainConfig
		expectedHTTP    string
		expectedHTTPS   string
	}{
		{
			name:          "no web config",
			cfg:           &config.DomainConfig{},
			expectedHTTP:  "any",
			expectedHTTPS: "any",
		},
		{
			name: "custom HTTP mode",
			cfg: &config.DomainConfig{
				Web: &config.WebChecks{
					HTTP:  config.HTTPRedirect,
					HTTPS: config.HTTPDirect,
				},
			},
			expectedHTTP:  "redirect",
			expectedHTTPS: "direct",
		},
		{
			name: "partial web config",
			cfg: &config.DomainConfig{
				Web: &config.WebChecks{
					HTTP: config.HTTPNo,
				},
			},
			expectedHTTP:  "no",
			expectedHTTPS: "any",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpMode, httpsMode := tt.cfg.WebModes()
			if httpMode != tt.expectedHTTP {
				t.Errorf("WebModes() HTTP = %q, want %q", httpMode, tt.expectedHTTP)
			}
			if httpsMode != tt.expectedHTTPS {
				t.Errorf("WebModes() HTTPS = %q, want %q", httpsMode, tt.expectedHTTPS)
			}
		})
	}
}

func TestDomainConfigHasHTTPSCheck(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.DomainConfig
		expected bool
	}{
		{
			name:     "no DNS config",
			cfg:      &config.DomainConfig{},
			expected: false,
		},
		{
			name: "DNS config without HTTPS",
			cfg: &config.DomainConfig{
				DNS: &config.DNSChecks{
					A:    config.DNSYes,
					AAAA: config.DNSYes,
				},
			},
			expected: false,
		},
		{
			name: "DNS config with HTTPS",
			cfg: &config.DomainConfig{
				DNS: &config.DNSChecks{
					A:     config.DNSYes,
					AAAA:  config.DNSYes,
					HTTPS: config.DNSOptional,
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cfg.HasHTTPSCheck()
			if result != tt.expected {
				t.Errorf("HasHTTPSCheck() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDomainConfigHTTPSCheckMode(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.DomainConfig
		expected string
	}{
		{
			name:     "no DNS config",
			cfg:      &config.DomainConfig{},
			expected: "",
		},
		{
			name: "DNS config with HTTPS optional",
			cfg: &config.DomainConfig{
				DNS: &config.DNSChecks{
					HTTPS: config.DNSOptional,
				},
			},
			expected: "optional",
		},
		{
			name: "DNS config with HTTPS yes",
			cfg: &config.DomainConfig{
				DNS: &config.DNSChecks{
					HTTPS: config.DNSYes,
				},
			},
			expected: "yes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cfg.HTTPSCheckMode()
			if result != tt.expected {
				t.Errorf("HTTPSCheckMode() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestDomainConfigTestAllIPs(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.DomainConfig
		expected bool
	}{
		{
			name:     "no web config",
			cfg:      &config.DomainConfig{},
			expected: false,
		},
		{
			name: "test all IPs enabled",
			cfg: &config.DomainConfig{
				Web: &config.WebChecks{
					TestAllIPs: true,
				},
			},
			expected: true,
		},
		{
			name: "test all IPs disabled",
			cfg: &config.DomainConfig{
				Web: &config.WebChecks{
					TestAllIPs: false,
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cfg.TestAllIPs()
			if result != tt.expected {
				t.Errorf("TestAllIPs() = %v, want %v", result, tt.expected)
			}
		})
	}
}
