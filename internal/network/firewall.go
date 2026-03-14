package network

import (
	"fmt"
	"os/exec"
)

// AllowPort opens a port using the first available firewall tool.
func AllowPort(port int, proto string) error {
	// Try UFW first
	if _, err := exec.LookPath("ufw"); err == nil {
		return ufwAllow(port, proto)
	}

	// Try firewall-cmd
	if _, err := exec.LookPath("firewall-cmd"); err == nil {
		return firewalldAllow(port, proto)
	}

	// Fallback to iptables
	if _, err := exec.LookPath("iptables"); err == nil {
		return iptablesAllow(port, proto)
	}

	return fmt.Errorf("no supported firewall found (ufw, firewalld, iptables)")
}

// RemovePort removes a firewall rule for a port.
func RemovePort(port int, proto string) error {
	if _, err := exec.LookPath("ufw"); err == nil {
		return run("ufw", "delete", "allow", fmt.Sprintf("%d/%s", port, proto))
	}
	if _, err := exec.LookPath("firewall-cmd"); err == nil {
		_ = run("firewall-cmd", "--permanent", "--remove-port", fmt.Sprintf("%d/%s", port, proto))
		return run("firewall-cmd", "--reload")
	}
	if _, err := exec.LookPath("iptables"); err == nil {
		return run("iptables", "-D", "INPUT", "-p", proto, "--dport", fmt.Sprintf("%d", port), "-j", "ACCEPT")
	}
	return nil
}

func ufwAllow(port int, proto string) error {
	return run("ufw", "allow", fmt.Sprintf("%d/%s", port, proto))
}

func firewalldAllow(port int, proto string) error {
	if err := run("firewall-cmd", "--permanent", "--add-port", fmt.Sprintf("%d/%s", port, proto)); err != nil {
		return err
	}
	return run("firewall-cmd", "--reload")
}

func iptablesAllow(port int, proto string) error {
	return run("iptables", "-A", "INPUT", "-p", proto, "--dport", fmt.Sprintf("%d", port), "-j", "ACCEPT")
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	return cmd.Run()
}
