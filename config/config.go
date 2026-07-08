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
	IP       string `yaml:"ip"`       // ipv4, ipv6
}

type HTTPChecks struct {
	Checks []HTTPCheck `yaml:"checks"`
}

type Checks struct {
	DNS  *DNSChecks  `yaml:"dns"`
	HTTP *HTTPChecks `yaml:"http"`
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
