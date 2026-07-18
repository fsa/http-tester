package checker

import "time"

const (
	// Network timeouts
	DefaultDialTimeout       = 10 * time.Second
	DefaultResponseTimeout   = 10 * time.Second
	DefaultTLSHandshake      = 10 * time.Second
	DefaultRequestTimeout    = 15 * time.Second
	DefaultDNSClientTimeout  = 5 * time.Second
	DefaultResolverProbe     = 3 * time.Second

	// Body size limits
	MaxBodySizeForCheck = 512 * 1024 // 512 KB
)
