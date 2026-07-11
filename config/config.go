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

type AliasConfig struct {
	Name   string     `yaml:"name"`
	DNS    *DNSChecks `yaml:"dns,omitempty"`
	HasDNS bool       `yaml:"-"`
	Web    *WebChecks `yaml:"web,omitempty"`
	HasWeb bool       `yaml:"-"`
}

type DomainConfig struct {
	Name    string        `yaml:"name"`
	DNS     *DNSChecks    `yaml:"dns,omitempty"`
	HasDNS  bool          `yaml:"-"`
	Web     *WebChecks    `yaml:"web,omitempty"`
	HasWeb  bool          `yaml:"-"`
	Aliases []AliasConfig `yaml:"aliases,omitempty"`
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

// UnmarshalYAML for AliasConfig to detect dns: and web: key presence
func (a *AliasConfig) UnmarshalYAML(value *yaml.Node) error {
	type Alias AliasConfig
	var al Alias
	if err := value.Decode(&al); err != nil {
		return err
	}
	*a = AliasConfig(al)

	if value.Kind == yaml.MappingNode {
		for i := 0; i < len(value.Content)-1; i += 2 {
			key := value.Content[i].Value
			if key == "dns" {
				a.HasDNS = true
				if a.DNS == nil {
					a.DNS = &DNSChecks{A: DNSOptional, AAAA: DNSOptional, HTTPS: DNSOptional}
				}
			}
			if key == "web" {
				a.HasWeb = true
			}
		}
	}
	return nil
}

// Load creates a config from domain name and optional config file.
// Default: dns optional, web any. Config file values override defaults.
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

	if domain != "" {
		cfg.Name = domain
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

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	if cfg.Name == "" {
		return nil, fmt.Errorf("domain name required (via CLI or config file)")
	}

	return cfg, nil
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
	for i, alias := range c.Aliases {
		if alias.Name == "" {
			return fmt.Errorf("alias #%d: name is required", i+1)
		}
		if err := validateDNSChecks(alias.DNS, fmt.Sprintf("aliases[%d].dns", i)); err != nil {
			return err
		}
		if err := validateWebChecks(alias.Web, fmt.Sprintf("aliases[%d].web", i)); err != nil {
			return err
		}
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
