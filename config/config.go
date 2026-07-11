package config

import (
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
		// dns: (empty) — all records optional
		d.A = DNSOptional
		d.AAAA = DNSOptional
		d.HTTPS = DNSOptional
		return nil
	}
	// Normal mapping
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
		case "any", "redirect", "direct", "":
			*m = HTTPMode(value.Value)
		default:
			*m = HTTPMode(value.Value)
		}
		return nil
	}
	return nil
}

type WebChecks struct {
	HTTP  HTTPMode `yaml:"http,omitempty"`
	HTTPS HTTPMode `yaml:"https,omitempty"`
}

// UnmarshalYAML handles empty web: section (sets defaults: http=any, https=any)
func (w *WebChecks) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode && value.Value == "" {
		// web: (empty) — defaults
		w.HTTP = HTTPAny
		w.HTTPS = HTTPAny
		return nil
	}
	// Normal mapping
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

	// Check if dns/web keys exist in the YAML mapping
	if value.Kind == yaml.MappingNode {
		for i := 0; i < len(value.Content)-1; i += 2 {
			key := value.Content[i].Value
			if key == "dns" {
				d.HasDNS = true
				// Empty dns: means all records optional
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

func LoadDomain(path string) (*DomainConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg DomainConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
