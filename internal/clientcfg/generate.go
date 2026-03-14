package clientcfg

import (
	"fmt"
	"net"

	"github.com/anonvector/slipgate/internal/config"
)

// URIOptions controls URI generation.
type URIOptions struct {
	ClientMode string // "dnstt" or "noizdns" (DNSTT transport only)
	Username   string // override SOCKS/SSH username
	Password   string // override SOCKS/SSH password
}

// GenerateURI builds a slipnet:// URI from tunnel + backend config.
func GenerateURI(tunnel *config.TunnelConfig, backend *config.BackendConfig, cfg *config.Config, opts URIOptions) (string, error) {
	var fields [TotalFields]string

	fields[FieldVersion] = "16"
	fields[FieldType] = GetTunnelType(tunnel.Transport, tunnel.Backend, opts.ClientMode)
	fields[FieldDomain] = tunnel.Domain
	fields[FieldName] = tunnel.Tag

	serverIP := getServerIP()
	fields[FieldServerIP] = serverIP

	switch tunnel.Transport {
	case config.TransportDNSTT:
		if tunnel.DNSTT != nil {
			fields[FieldPubKey] = tunnel.DNSTT.PublicKey
			fields[FieldMTU] = fmt.Sprintf("%d", tunnel.DNSTT.MTU)
		}

	case config.TransportSlipstream:
		if tunnel.Slipstream != nil {
			// cert fingerprint could go here
		}

	case config.TransportNaive:
		if tunnel.Naive != nil {
			fields[FieldSOCKSUser] = tunnel.Naive.User
			fields[FieldSOCKSPass] = tunnel.Naive.Password
		}
	}

	// User credentials override backend defaults
	if opts.Username != "" {
		if tunnel.Backend == config.BackendSOCKS {
			fields[FieldSOCKSUser] = opts.Username
			fields[FieldSOCKSPass] = opts.Password
		} else if tunnel.Backend == config.BackendSSH {
			fields[FieldSSHUser] = opts.Username
			fields[FieldSSHPass] = opts.Password
		}
	} else if backend != nil && backend.Type == config.BackendSOCKS && backend.SOCKS != nil {
		if fields[FieldSOCKSUser] == "" {
			fields[FieldSOCKSUser] = backend.SOCKS.User
		}
		if fields[FieldSOCKSPass] == "" {
			fields[FieldSOCKSPass] = backend.SOCKS.Password
		}
	}

	return Encode(fields), nil
}

func getServerIP() string {
	// Try to find the public IP by connecting to a known address
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}
