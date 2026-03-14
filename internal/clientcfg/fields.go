package clientcfg

import "github.com/anonvector/slipgate/internal/config"

// v16 field positions (pipe-delimited).
const (
	FieldVersion    = 0  // always "16"
	FieldType       = 1  // tunnel type
	FieldDomain     = 2  // tunnel domain
	FieldPubKey     = 3  // public key (hex)
	FieldMTU        = 4  // MTU
	FieldDOH        = 5  // DoH resolver
	FieldSOCKSUser  = 6  // SOCKS username
	FieldSOCKSPass  = 7  // SOCKS password
	FieldSSHUser    = 8  // SSH username
	FieldSSHPass    = 9  // SSH password
	FieldServerIP   = 10 // server IP
	FieldCert       = 11 // certificate fingerprint
	FieldExtra1     = 12 // extra field 1
	FieldExtra2     = 13 // extra field 2
	FieldExtra3     = 14 // extra field 3
	FieldName       = 15 // profile name
	TotalFields     = 16
)

// Client modes for DNSTT transport (server is the same, client behavior differs).
const (
	ClientModeDNSTT   = "dnstt"
	ClientModeNoizDNS = "noizdns"
)

// TunnelTypeMap maps transport+clientMode+backend to slipnet:// field[1] value.
// For DNSTT transport, the clientMode determines whether the URL uses dnstt or sayedns.
var TunnelTypeMap = map[string]map[string]map[string]string{
	config.TransportDNSTT: {
		ClientModeDNSTT: {
			config.BackendSOCKS: "dnstt",
			config.BackendSSH:   "dnstt_ssh",
		},
		ClientModeNoizDNS: {
			config.BackendSOCKS: "sayedns",
			config.BackendSSH:   "sayedns_ssh",
		},
	},
	config.TransportSlipstream: {
		"": {
			config.BackendSOCKS: "ss",
			config.BackendSSH:   "slipstream_ssh",
		},
	},
	config.TransportNaive: {
		"": {
			config.BackendSOCKS: "naive",
			config.BackendSSH:   "naive_ssh",
		},
	},
}

// GetTunnelType returns the slipnet:// type string.
// clientMode is only relevant for DNSTT transport ("dnstt" or "noizdns").
func GetTunnelType(transport, backend, clientMode string) string {
	modes, ok := TunnelTypeMap[transport]
	if !ok {
		return "unknown"
	}
	// For non-DNSTT transports, clientMode is ignored
	if transport != config.TransportDNSTT {
		clientMode = ""
	}
	if m, ok := modes[clientMode]; ok {
		if t, ok := m[backend]; ok {
			return t
		}
	}
	return "unknown"
}
