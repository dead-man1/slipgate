package proxy

import (
	"fmt"
	"path/filepath"

	"github.com/anonvector/slipgate/internal/config"
	"github.com/anonvector/slipgate/internal/service"
)

// SetupMicrosocks creates the systemd service for microsocks (localhost only).
func SetupMicrosocks() error {
	return setupMicrosocks("127.0.0.1", "", "")
}

// SetupMicrosocksWithAuth creates microsocks with optional auth (localhost only).
func SetupMicrosocksWithAuth(user, password string) error {
	return setupMicrosocks("127.0.0.1", user, password)
}

// SetupMicrosocksExternal creates microsocks listening on all interfaces (for direct SOCKS5).
func SetupMicrosocksExternal(user, password string) error {
	return setupMicrosocks("0.0.0.0", user, password)
}

func setupMicrosocks(listenAddr, user, password string) error {
	binPath := filepath.Join(config.DefaultBinDir, "microsocks")

	args := fmt.Sprintf("%s -i %s -p 1080", binPath, listenAddr)
	if user != "" && password != "" {
		args += fmt.Sprintf(" -u %s -P %s", user, password)
	}

	unit := &service.Unit{
		Name:        "slipgate-microsocks",
		Description: "SlipGate SOCKS5 proxy (microsocks)",
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
