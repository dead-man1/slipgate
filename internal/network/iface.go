package network

import (
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// DefaultInterface returns the name of the default network interface.
func DefaultInterface() string {
	out, err := exec.Command("ip", "route", "show", "default").Output()
	if err != nil {
		return "eth0"
	}
	// "default via 10.0.0.1 dev eth0 ..."
	fields := strings.Fields(string(out))
	for i, f := range fields {
		if f == "dev" && i+1 < len(fields) {
			return fields[i+1]
		}
	}
	return "eth0"
}

// PublicIP returns the server's public IPv4 address.
func PublicIP() string {
	client := &http.Client{Timeout: 5 * time.Second}
	for _, url := range []string{"https://ifconfig.me", "https://api.ipify.org", "https://icanhazip.com"} {
		resp, err := client.Get(url)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		ip := strings.TrimSpace(string(body))
		if ip != "" {
			return ip
		}
	}
	return ""
}
