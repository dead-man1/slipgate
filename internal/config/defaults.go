package config

// BasePort is the starting port for DNS tunnel forwarding.
const BasePort = 5310

// NextAvailablePort returns the next unused port starting from BasePort.
func (c *Config) NextAvailablePort() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	used := make(map[int]bool)
	for _, t := range c.Tunnels {
		if t.Port > 0 {
			used[t.Port] = true
		}
	}
	for port := BasePort; ; port++ {
		if !used[port] {
			return port
		}
	}
}

// DefaultBackends returns the standard backend configs.
func DefaultBackends() []BackendConfig {
	return []BackendConfig{
		{Tag: "socks", Type: BackendSOCKS, Address: "127.0.0.1:1080", SOCKS: &SOCKSConfig{}},
		{Tag: "ssh", Type: BackendSSH, Address: "127.0.0.1:22"},
	}
}

// DefaultMTU for DNS tunnels.
const DefaultMTU = 1232

// TransportBinaries maps transport types to their required binaries.
var TransportBinaries = map[string]string{
	TransportDNSTT:      "dnstt-server",
	TransportSlipstream: "slipstream-server",
	TransportNaive:      "caddy-naive",
}
