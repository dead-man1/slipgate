package actions

// Shared select options used across multiple actions.

var TransportOptions = []SelectOption{
	{Value: "dnstt", Label: "DNSTT / NoizDNS — DNS tunnel"},
	{Value: "slipstream", Label: "Slipstream — QUIC DNS tunnel"},
	{Value: "naive", Label: "NaiveProxy — HTTPS proxy with Caddy"},
}

// ClientModeOptions is used during `tunnel share` to pick the slipnet:// URL type.
var ClientModeOptions = []SelectOption{
	{Value: "dnstt", Label: "DNSTT — Classic DNS tunnel client"},
	{Value: "noizdns", Label: "NoizDNS — DNS tunnel with DPI evasion"},
}

var BackendOptions = []SelectOption{
	{Value: "socks", Label: "SOCKS — SOCKS5 proxy (microsocks)"},
	{Value: "ssh", Label: "SSH — SSH tunnel"},
}

var RouterModeOptions = []SelectOption{
	{Value: "single", Label: "Single — one active tunnel at a time"},
	{Value: "multi", Label: "Multi — domain-based routing to multiple tunnels"},
}
