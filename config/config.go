package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type DNSChecks struct {
	A           bool `yaml:"a"`
	AAAA        bool `yaml:"aaaa"`
	HTTPS       bool `yaml:"https"`
	Consistency bool `yaml:"consistency"`
}

type HTTPCheck struct {
	Protocol string `yaml:"protocol"` // http1, http2, http3
	Port     int    `yaml:"port"`     // 80, 443 (default 443)
	Status   []int  `yaml:"status"`   // expected status codes, e.g. [200] or [301, 302]
}

type AliasConfig struct {
	Name string      `yaml:"name"`
	DNS  *DNSChecks  `yaml:"dns,omitempty"`
	HTTP []HTTPCheck `yaml:"http,omitempty"`
}

type DomainConfig struct {
	Name    string        `yaml:"name"`
	DNS     *DNSChecks    `yaml:"dns,omitempty"`
	HTTP    []HTTPCheck   `yaml:"http,omitempty"`
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
