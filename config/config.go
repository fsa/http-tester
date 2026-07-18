package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type DNSRecordCheck string

const (
	DNSYes      DNSRecordCheck = "yes"
	DNSNo       DNSRecordCheck = "no"
	DNSOptional DNSRecordCheck = "optional"
	DNSNone     DNSRecordCheck = ""
)

type DNSChecks struct {
	A     DNSRecordCheck `yaml:"a"`
	AAAA  DNSRecordCheck `yaml:"aaaa"`
	HTTPS DNSRecordCheck `yaml:"https"`
}

// UnmarshalYAML handles empty dns: section (sets all to optional)
func (d *DNSChecks) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode && value.Value == "" {
		d.A = DNSOptional
		d.AAAA = DNSOptional
		d.HTTPS = DNSOptional
		return nil
	}
	type Alias DNSChecks
	var a Alias
	if err := value.Decode(&a); err != nil {
		return err
	}
	*d = DNSChecks(a)
	return nil
}

type HTTPMode string

const (
	HTTPAny      HTTPMode = "any"
	HTTPRedirect HTTPMode = "redirect"
	HTTPDirect   HTTPMode = "direct"
	HTTPNo       HTTPMode = "no"
	HTTPNone     HTTPMode = ""
)

// UnmarshalYAML supports: any, redirect, direct, no
func (m *HTTPMode) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		switch value.Value {
		case "no":
			*m = HTTPNo
		default:
			*m = HTTPMode(value.Value)
		}
		return nil
	}
	return nil
}

type WebChecks struct {
	HTTP       HTTPMode `yaml:"http,omitempty"`
	HTTPS      HTTPMode `yaml:"https,omitempty"`
	TestAllIPs bool     `yaml:"test_all_ips,omitempty"`
}

// UnmarshalYAML handles empty web: section (sets defaults: http=any, https=any)
func (w *WebChecks) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode && value.Value == "" {
		w.HTTP = HTTPAny
		w.HTTPS = HTTPAny
		return nil
	}
	type Alias WebChecks
	var a Alias
	if err := value.Decode(&a); err != nil {
		return err
	}
	*w = WebChecks(a)
	return nil
}

type DomainConfig struct {
	Name   string     `yaml:"name"`
	DNS    *DNSChecks `yaml:"dns,omitempty"`
	HasDNS bool       `yaml:"-"`
	Web    *WebChecks `yaml:"web,omitempty"`
	HasWeb bool       `yaml:"-"`
}

// UnmarshalYAML for DomainConfig to detect dns: and web: key presence
func (d *DomainConfig) UnmarshalYAML(value *yaml.Node) error {
	type Alias DomainConfig
	var a Alias
	if err := value.Decode(&a); err != nil {
		return err
	}
	*d = DomainConfig(a)

	if value.Kind == yaml.MappingNode {
		for i := 0; i < len(value.Content)-1; i += 2 {
			key := value.Content[i].Value
			if key == "dns" {
				d.HasDNS = true
				if d.DNS == nil {
					d.DNS = &DNSChecks{A: DNSOptional, AAAA: DNSOptional, HTTPS: DNSOptional}
				}
			}
			if key == "web" {
				d.HasWeb = true
			}
		}
	}
	return nil
}

// Load creates a config from domain name and optional config file.
// Priority: CLI args > config file > defaults.
func Load(domain, configFile string) (*DomainConfig, error) {
	if domain == "" && configFile == "" {
		return nil, fmt.Errorf("domain name required (via CLI or config file)")
	}

	cfg := &DomainConfig{
		HasDNS: true,
		DNS:    &DNSChecks{A: DNSOptional, AAAA: DNSOptional, HTTPS: DNSOptional},
		HasWeb: true,
		Web:    &WebChecks{HTTP: HTTPAny, HTTPS: HTTPAny},
	}

	if configFile != "" {
		data, err := os.ReadFile(configFile)
		if err != nil {
			return nil, fmt.Errorf("reading config: %w", err)
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parsing config: %w", err)
		}
	}

	// CLI domain overrides config file
	if domain != "" {
		cfg.Name = domain
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	if cfg.Name == "" {
		return nil, fmt.Errorf("domain name required (via CLI or config file)")
	}

	return cfg, nil
}

// ApplyCLI applies CLI overrides on top of loaded config.
func (c *DomainConfig) ApplyCLI(dnsA, dnsAAAA, dnsHTTPS, webHTTP, webHTTPS string, testAllIPs bool, testAllIPsSet bool) {
	if dnsA != "" {
		c.DNS.A = DNSRecordCheck(dnsA)
	}
	if dnsAAAA != "" {
		c.DNS.AAAA = DNSRecordCheck(dnsAAAA)
	}
	if dnsHTTPS != "" {
		c.DNS.HTTPS = DNSRecordCheck(dnsHTTPS)
	}
	if webHTTP != "" {
		c.Web.HTTP = HTTPMode(webHTTP)
	}
	if webHTTPS != "" {
		c.Web.HTTPS = HTTPMode(webHTTPS)
	}
	if testAllIPsSet {
		c.Web.TestAllIPs = testAllIPs
	}
}

// LoadDomain is a convenience wrapper for loading config from a file only.
func LoadDomain(path string) (*DomainConfig, error) {
	return Load("", path)
}

func (c *DomainConfig) validate() error {
	if err := validateDNSChecks(c.DNS, "dns"); err != nil {
		return err
	}
	if err := validateWebChecks(c.Web, "web"); err != nil {
		return err
	}
	return nil
}

func validateDNSChecks(d *DNSChecks, prefix string) error {
	if d == nil {
		return nil
	}
	for _, check := range []struct {
		name string
		val  DNSRecordCheck
	}{
		{"a", d.A}, {"aaaa", d.AAAA}, {"https", d.HTTPS},
	} {
		if check.val != "" && check.val != DNSYes && check.val != DNSNo && check.val != DNSOptional {
			return fmt.Errorf("invalid %s.%s: %q (expected yes, no, or optional)", prefix, check.name, check.val)
		}
	}
	return nil
}

func validateWebChecks(w *WebChecks, prefix string) error {
	if w == nil {
		return nil
	}
	for _, check := range []struct {
		name string
		val  HTTPMode
	}{
		{"http", w.HTTP}, {"https", w.HTTPS},
	} {
		if check.val != "" && check.val != HTTPAny && check.val != HTTPRedirect && check.val != HTTPDirect && check.val != HTTPNo {
			return fmt.Errorf("invalid %s.%s: %q (expected any, redirect, direct, or no)", prefix, check.name, check.val)
		}
	}
	return nil
}

// WebModes returns the HTTP and HTTPS modes, defaulting to "any" if not configured.
func (d *DomainConfig) WebModes() (httpMode, httpsMode string) {
	httpMode = "any"
	httpsMode = "any"
	if d.Web != nil {
		if d.Web.HTTP != "" {
			httpMode = string(d.Web.HTTP)
		}
		if d.Web.HTTPS != "" {
			httpsMode = string(d.Web.HTTPS)
		}
	}
	return httpMode, httpsMode
}

// HasHTTPSCheck reports whether an HTTPS DNS check is configured.
func (d *DomainConfig) HasHTTPSCheck() bool {
	return d.DNS != nil && d.DNS.HTTPS != ""
}

// HTTPSCheckMode returns the HTTPS DNS check mode as a string.
func (d *DomainConfig) HTTPSCheckMode() string {
	if d.DNS != nil {
		return string(d.DNS.HTTPS)
	}
	return ""
}

// TestAllIPs reports whether all resolved IPs should be tested.
func (d *DomainConfig) TestAllIPs() bool {
	if d.Web != nil {
		return d.Web.TestAllIPs
	}
	return false
}
