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
	HTTPAuto     HTTPMode = "auto"
	HTTPRedirect HTTPMode = "redirect"
	HTTPDirect   HTTPMode = "direct"
	HTTPNo       HTTPMode = "no"
	HTTPNone     HTTPMode = ""
)

// UnmarshalYAML supports string values and boolean aliases
func (m *HTTPMode) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		switch value.Value {
		case "true", "yes":
			*m = HTTPAuto
		case "false", "no", "":
			*m = HTTPNo
		default:
			*m = HTTPMode(value.Value)
		}
		return nil
	}
	return nil
}

type WebChecks struct {
	HTTP  HTTPMode `yaml:"http,omitempty"`
	HTTPS bool     `yaml:"https,omitempty"`
}

// UnmarshalYAML handles empty web: section (sets defaults: http=auto, https=true)
func (w *WebChecks) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode && value.Value == "" {
		// web: (empty) — defaults
		w.HTTP = HTTPAuto
		w.HTTPS = true
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
	Web    *WebChecks `yaml:"web,omitempty"`
	HasWeb bool       `yaml:"-"`
}

type DomainConfig struct {
	Name    string        `yaml:"name"`
	DNS     *DNSChecks    `yaml:"dns,omitempty"`
	Web     *WebChecks    `yaml:"web,omitempty"`
	HasWeb  bool          `yaml:"-"`
	Aliases []AliasConfig `yaml:"aliases,omitempty"`
}

// UnmarshalYAML for DomainConfig to detect web: key presence
func (d *DomainConfig) UnmarshalYAML(value *yaml.Node) error {
	type Alias DomainConfig
	var a Alias
	if err := value.Decode(&a); err != nil {
		return err
	}
	*d = DomainConfig(a)

	// Check if web key exists in the YAML mapping
	if value.Kind == yaml.MappingNode {
		for i := 0; i < len(value.Content)-1; i += 2 {
			if value.Content[i].Value == "web" {
				d.HasWeb = true
				break
			}
		}
	}
	return nil
}

// UnmarshalYAML for AliasConfig to detect web: key presence
func (a *AliasConfig) UnmarshalYAML(value *yaml.Node) error {
	type Alias AliasConfig
	var al Alias
	if err := value.Decode(&al); err != nil {
		return err
	}
	*a = AliasConfig(al)

	if value.Kind == yaml.MappingNode {
		for i := 0; i < len(value.Content)-1; i += 2 {
			if value.Content[i].Value == "web" {
				a.HasWeb = true
				break
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
