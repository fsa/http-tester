package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type DNSRecordCheck string

const (
	DNSYes   DNSRecordCheck = "yes"
	DNSNo    DNSRecordCheck = "no"
	DNSMaybe DNSRecordCheck = "maybe"
	DNSNone  DNSRecordCheck = ""
)

type DNSChecks struct {
	A     DNSRecordCheck `yaml:"a"`
	AAAA  DNSRecordCheck `yaml:"aaaa"`
	HTTPS DNSRecordCheck `yaml:"https"`
}

// UnmarshalYAML handles empty dns: section (sets all to maybe)
func (d *DNSChecks) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode && value.Value == "" {
		// dns: (empty) — all records optional
		d.A = DNSMaybe
		d.AAAA = DNSMaybe
		d.HTTPS = DNSMaybe
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
	HTTPRedirect HTTPMode = "redirect"
	HTTPDirect   HTTPMode = "direct"
	HTTPNone     HTTPMode = ""
)

// UnmarshalYAML supports both string values and boolean true (treated as "redirect")
func (m *HTTPMode) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		if value.Value == "true" {
			*m = HTTPRedirect
			return nil
		}
		if value.Value == "false" || value.Value == "" {
			*m = HTTPNone
			return nil
		}
		*m = HTTPMode(value.Value)
		return nil
	}
	return nil
}

type WebChecks struct {
	HTTP  HTTPMode `yaml:"http,omitempty"`
	HTTPS bool     `yaml:"https,omitempty"`
}

type AliasConfig struct {
	Name string     `yaml:"name"`
	DNS  *DNSChecks `yaml:"dns,omitempty"`
	Web  *WebChecks `yaml:"web,omitempty"`
}

type DomainConfig struct {
	Name    string        `yaml:"name"`
	DNS     *DNSChecks    `yaml:"dns,omitempty"`
	Web     *WebChecks    `yaml:"web,omitempty"`
	Aliases []AliasConfig `yaml:"aliases,omitempty"`
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
