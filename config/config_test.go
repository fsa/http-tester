package config

import (
	"os"
	"testing"
)

func TestLoad_DomainFromCLI(t *testing.T) {
	cfg, err := Load("example.com", "")
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Name != "example.com" {
		t.Errorf("Name = %q, want %q", cfg.Name, "example.com")
	}
	if cfg.DNS == nil {
		t.Fatal("DNS is nil")
	}
	if cfg.DNS.A != DNSOptional {
		t.Errorf("DNS.A = %q, want %q", cfg.DNS.A, DNSOptional)
	}
	if cfg.Web == nil {
		t.Fatal("Web is nil")
	}
	if cfg.Web.HTTP != HTTPAny {
		t.Errorf("Web.HTTP = %q, want %q", cfg.Web.HTTP, HTTPAny)
	}
}

func TestLoad_ConfigFile(t *testing.T) {
	yaml := `
name: example.com
dns:
  a: yes
  aaaa: optional
  https: no
web:
  http: redirect
  https: any
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

	cfg, err := Load("", tmpFile.Name())
	if err != nil {
		t.Fatalf("Load error: %v", err)
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
	if cfg.DNS.AAAA != DNSOptional {
		t.Errorf("DNS.AAAA = %q, want %q", cfg.DNS.AAAA, DNSOptional)
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
}

func TestLoad_DomainFromCLIOverridesFile(t *testing.T) {
	yaml := `
name: from-file.com
dns:
  a: yes
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

	// Domain from CLI overrides config file
	cfg, err := Load("cli.com", tmpFile.Name())
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Name != "cli.com" {
		t.Errorf("Name = %q, want %q (CLI overrides file)", cfg.Name, "cli.com")
	}
	// But other fields from file are preserved
	if cfg.DNS.A != DNSYes {
		t.Errorf("DNS.A = %q, want %q (file value preserved)", cfg.DNS.A, DNSYes)
	}
}

func TestLoad_EmptyDNS(t *testing.T) {
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

	cfg, err := Load("", tmpFile.Name())
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if cfg.DNS == nil {
		t.Fatal("Empty dns: DNS should not be nil")
	}
	if cfg.DNS.A != DNSOptional {
		t.Errorf("Empty dns: A = %q, want %q", cfg.DNS.A, DNSOptional)
	}
	if cfg.DNS.AAAA != DNSOptional {
		t.Errorf("Empty dns: AAAA = %q, want %q", cfg.DNS.AAAA, DNSOptional)
	}
	if cfg.DNS.HTTPS != DNSOptional {
		t.Errorf("Empty dns: HTTPS = %q, want %q", cfg.DNS.HTTPS, DNSOptional)
	}
}

func TestLoad_EmptyWeb(t *testing.T) {
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

	cfg, err := Load("", tmpFile.Name())
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if cfg.Web != nil {
		t.Errorf("Empty web: Web should be nil, got %+v", cfg.Web)
	}
	if !cfg.HasWeb {
		t.Error("HasWeb should be true for empty web:")
	}
}

func TestLoad_HasWebDetection(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantWeb bool
	}{
		{"with web", "name: example.com\nweb:\n", true},
		{"without web", "name: example.com\ndns:\n  a: yes\n", false},
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

			cfg, err := Load("", tmpFile.Name())
			if err != nil {
				t.Fatalf("Load error: %v", err)
			}

			if cfg.HasWeb != tt.wantWeb {
				t.Errorf("HasWeb = %v, want %v", cfg.HasWeb, tt.wantWeb)
			}
		})
	}
}

func TestLoad_InvalidDNS(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{"invalid a", "name: x\ndns:\n  a: maybe\n"},
		{"invalid aaaa", "name: x\ndns:\n  aaaa: true\n"},
		{"invalid https", "name: x\ndns:\n  https: blah\n"},
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

			_, err = Load("", tmpFile.Name())
			if err == nil {
				t.Error("expected error for invalid DNS value")
			}
		})
	}
}

func TestLoad_InvalidWeb(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{"invalid http", "name: x\nweb:\n  http: blah\n"},
		{"invalid https", "name: x\nweb:\n  https: true\n"},
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

			_, err = Load("", tmpFile.Name())
			if err == nil {
				t.Error("expected error for invalid web value")
			}
		})
	}
}

func TestLoad_NoDomain(t *testing.T) {
	_, err := Load("", "")
	if err == nil {
		t.Error("expected error when no domain and no config")
	}
}

func TestLoad_DomainRequired(t *testing.T) {
	yaml := `
dns:
  a: yes
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

	_, err = Load("", tmpFile.Name())
	if err == nil {
		t.Error("expected error when config has no name and no domain from CLI")
	}
}

func TestLoadDomain_Wrapper(t *testing.T) {
	yaml := `
name: example.com
dns:
  a: yes
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
}
