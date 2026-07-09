package config

import (
	"os"
	"testing"
)

func TestLoadDomain(t *testing.T) {
	yaml := `
name: example.com
dns:
  a: yes
  aaaa: maybe
  https: no
web:
  http: redirect
  https: true
aliases:
  - name: www.example.com
    dns:
      a: yes
    web:
      http: direct
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(yaml); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	cfg, err := LoadDomain(tmpFile.Name())
	if err != nil {
		t.Fatalf("LoadDomain error: %v", err)
	}

	if cfg.Name != "example.com" {
		t.Errorf("Name = %q, want %q", cfg.Name, "example.com")
	}
	if cfg.DNS == nil {
		t.Fatal("DNS is nil")
	}
	if cfg.DNS.A != DNSYes {
		t.Errorf("DNS.A = %q, want %q", cfg.DNS.A, DNSYes)
	}
	if cfg.DNS.AAAA != DNSMaybe {
		t.Errorf("DNS.AAAA = %q, want %q", cfg.DNS.AAAA, DNSMaybe)
	}
	if cfg.DNS.HTTPS != DNSNo {
		t.Errorf("DNS.HTTPS = %q, want %q", cfg.DNS.HTTPS, DNSNo)
	}
	if cfg.Web == nil {
		t.Fatal("Web is nil")
	}
	if cfg.Web.HTTP != HTTPRedirect {
		t.Errorf("Web.HTTP = %q, want %q", cfg.Web.HTTP, HTTPRedirect)
	}
	if cfg.Web.HTTPS != HTTPAny {
		t.Errorf("Web.HTTPS = %q, want %q", cfg.Web.HTTPS, HTTPAny)
	}
	if len(cfg.Aliases) != 1 {
		t.Fatalf("len(Aliases) = %d, want 1", len(cfg.Aliases))
	}
	if cfg.Aliases[0].Name != "www.example.com" {
		t.Errorf("Alias[0].Name = %q, want %q", cfg.Aliases[0].Name, "www.example.com")
	}
	if cfg.Aliases[0].Web == nil {
		t.Fatal("Alias[0].Web is nil")
	}
	if cfg.Aliases[0].Web.HTTP != HTTPDirect {
		t.Errorf("Alias[0].Web.HTTP = %q, want %q", cfg.Aliases[0].Web.HTTP, HTTPDirect)
	}
}

func TestLoadDomain_EmptyDNS(t *testing.T) {
	yaml := `
name: example.com
dns:
web:
  http: redirect
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(yaml); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	cfg, err := LoadDomain(tmpFile.Name())
	if err != nil {
		t.Fatalf("LoadDomain error: %v", err)
	}

	// Empty dns: creates struct with all maybe
	if cfg.DNS == nil {
		t.Fatal("Empty dns: DNS should not be nil")
	}
	if cfg.DNS.A != DNSMaybe {
		t.Errorf("Empty dns: A = %q, want %q", cfg.DNS.A, DNSMaybe)
	}
	if cfg.DNS.AAAA != DNSMaybe {
		t.Errorf("Empty dns: AAAA = %q, want %q", cfg.DNS.AAAA, DNSMaybe)
	}
	if cfg.DNS.HTTPS != DNSMaybe {
		t.Errorf("Empty dns: HTTPS = %q, want %q", cfg.DNS.HTTPS, DNSMaybe)
	}
}

func TestLoadDomain_EmptyWeb(t *testing.T) {
	yaml := `
name: example.com
dns:
  a: yes
web:
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(yaml); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	cfg, err := LoadDomain(tmpFile.Name())
	if err != nil {
		t.Fatalf("LoadDomain error: %v", err)
	}

	// Empty web: results in nil pointer
	if cfg.Web != nil {
		t.Errorf("Empty web: Web should be nil, got %+v", cfg.Web)
	}
	if !cfg.HasWeb {
		t.Error("HasWeb should be true for empty web:")
	}
}

func TestLoadDomain_HasWebDetection(t *testing.T) {
	yamlWithWeb := `
name: example.com
web:
`
	yamlWithoutWeb := `
name: example.com
dns:
  a: yes
`

	tests := []struct {
		name    string
		yaml    string
		wantWeb bool
	}{
		{"with web", yamlWithWeb, true},
		{"without web", yamlWithoutWeb, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", "config-*.yaml")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(tmpFile.Name())

			if _, err := tmpFile.WriteString(tt.yaml); err != nil {
				t.Fatal(err)
			}
			tmpFile.Close()

			cfg, err := LoadDomain(tmpFile.Name())
			if err != nil {
				t.Fatalf("LoadDomain error: %v", err)
			}

			if cfg.HasWeb != tt.wantWeb {
				t.Errorf("HasWeb = %v, want %v", cfg.HasWeb, tt.wantWeb)
			}
		})
	}
}

func TestLoadDomain_HTTPModeFromBool(t *testing.T) {
	yaml := `
name: example.com
web:
  http: true
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(yaml); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	cfg, err := LoadDomain(tmpFile.Name())
	if err != nil {
		t.Fatalf("LoadDomain error: %v", err)
	}

	if cfg.Web == nil {
		t.Fatal("Web is nil")
	}
	if cfg.Web.HTTP != HTTPAny {
		t.Errorf("http: true should be any, got %q", cfg.Web.HTTP)
	}
}
