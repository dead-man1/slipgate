package router

import (
	"fmt"

	"github.com/anonvector/slipgate/internal/config"
	"github.com/anonvector/slipgate/internal/dnsrouter"
	"github.com/anonvector/slipgate/internal/service"
)

// SwitchMode transitions between single and multi mode.
func SwitchMode(cfg *config.Config, newMode string) error {
	oldMode := cfg.Route.Mode

	switch {
	case oldMode == "single" && newMode == "multi":
		return switchToMulti(cfg)
	case oldMode == "multi" && newMode == "single":
		return switchToSingle(cfg)
	default:
		return fmt.Errorf("already in %s mode", newMode)
	}
}

func switchToMulti(cfg *config.Config) error {
	// Stop all tunnel services that listen on :53 directly
	for _, t := range cfg.Tunnels {
		if t.IsDNSTunnel() && t.Enabled {
			svcName := service.TunnelServiceName(t.Tag)
			_ = service.Stop(svcName)
		}
	}

	// Create and start the DNS router
	if err := dnsrouter.CreateRouterService(); err != nil {
		return fmt.Errorf("create router: %w", err)
	}
	if err := dnsrouter.StartRouterService(); err != nil {
		return fmt.Errorf("start router: %w", err)
	}

	// Restart all tunnel services on their local ports
	for _, t := range cfg.Tunnels {
		if t.IsDNSTunnel() && t.Enabled {
			svcName := service.TunnelServiceName(t.Tag)
			if err := service.Start(svcName); err != nil {
				return fmt.Errorf("start tunnel %s: %w", t.Tag, err)
			}
		}
	}

	return nil
}

func switchToSingle(cfg *config.Config) error {
	// Stop the DNS router
	_ = dnsrouter.StopRouterService()

	// Stop all tunnel services except the active one
	for _, t := range cfg.Tunnels {
		if t.IsDNSTunnel() && t.Enabled && t.Tag != cfg.Route.Active {
			svcName := service.TunnelServiceName(t.Tag)
			_ = service.Stop(svcName)
		}
	}

	// Restart the active tunnel to listen on :53 directly
	if cfg.Route.Active != "" {
		svcName := service.TunnelServiceName(cfg.Route.Active)
		return service.Restart(svcName)
	}

	return nil
}
