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

type AliasConfig struct {
	Name string     `yaml:"name"`
	DNS  *DNSChecks `yaml:"dns,omitempty"`
	HTTP bool       `yaml:"http,omitempty"`
}

type DomainConfig struct {
	Name    string        `yaml:"name"`
	DNS     *DNSChecks    `yaml:"dns,omitempty"`
	HTTP    bool          `yaml:"http,omitempty"`
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
