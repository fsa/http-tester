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

type Checks struct {
	DNS *DNSChecks `yaml:"dns"`
}

type DomainConfig struct {
	Name   string  `yaml:"name"`
	Checks Checks  `yaml:"checks"`
}

type TestsConfig struct {
	Domains []DomainConfig `yaml:"domains"`
}

func LoadTests(path string) (*TestsConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg TestsConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
