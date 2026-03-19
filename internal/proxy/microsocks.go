package proxy

import (
	"fmt"
	"os"

	"github.com/anonvector/slipgate/internal/config"
	"github.com/anonvector/slipgate/internal/service"
)

const socksServiceName = "slipgate-socks5"

// SetupSOCKS creates the SOCKS5 proxy service (localhost only).
func SetupSOCKS() error {
	return setupSOCKS("127.0.0.1", "", "")
}

// SetupSOCKSWithAuth creates the SOCKS5 proxy with auth (localhost only).
func SetupSOCKSWithAuth(user, password string) error {
	return setupSOCKS("127.0.0.1", user, password)
}

// SetupSOCKSExternal creates the SOCKS5 proxy on all interfaces (for direct SOCKS5).
func SetupSOCKSExternal(user, password string) error {
	return setupSOCKS("0.0.0.0", user, password)
}

func setupSOCKS(listenAddr, user, password string) error {
	execPath, err := os.Executable()
	if err != nil {
		return err
	}

	args := fmt.Sprintf("%s socks serve --addr %s --port 1080", execPath, listenAddr)
	if user != "" && password != "" {
		args += fmt.Sprintf(" --user %s --pass %s", user, password)
	}

	// Clean up old microsocks service if it exists
	_ = service.Stop("slipgate-microsocks")
	_ = service.Remove("slipgate-microsocks")

	unit := &service.Unit{
		Name:        socksServiceName,
		Description: "SlipGate SOCKS5 proxy",
		ExecStart:   args,
		User:        config.SystemUser,
		Group:       config.SystemGroup,
		After:       "network.target",
		Restart:     "always",
	}

	if err := service.Create(unit); err != nil {
		return err
	}

	// Restart to pick up new ExecStart (e.g. after adding auth)
	_ = service.Restart(unit.Name)
	return service.Start(unit.Name)
}
