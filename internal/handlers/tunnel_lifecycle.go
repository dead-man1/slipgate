package handlers

import (
	"fmt"

	"github.com/anonvector/slipgate/internal/actions"
	"github.com/anonvector/slipgate/internal/config"
	"github.com/anonvector/slipgate/internal/service"
)

func handleTunnelStart(ctx *actions.Context) error {
	cfg := ctx.Config.(*config.Config)
	tag := ctx.GetArg("tag")
	if tag == "" {
		return actions.NewError(actions.TunnelStart, "tunnel tag is required", nil)
	}
	tunnel := cfg.GetTunnel(tag)
	if tunnel == nil {
		return actions.NewError(actions.TunnelStart, fmt.Sprintf("tunnel %q not found", tag), nil)
	}
	svcName := service.TunnelServiceName(tag)
	if err := service.Start(svcName); err != nil {
		return actions.NewError(actions.TunnelStart, "failed to start service", err)
	}
	ctx.Output.Success(fmt.Sprintf("Tunnel %q started", tag))
	return nil
}

func handleTunnelStop(ctx *actions.Context) error {
	cfg := ctx.Config.(*config.Config)
	tag := ctx.GetArg("tag")
	if tag == "" {
		return actions.NewError(actions.TunnelStop, "tunnel tag is required", nil)
	}
	tunnel := cfg.GetTunnel(tag)
	if tunnel == nil {
		return actions.NewError(actions.TunnelStop, fmt.Sprintf("tunnel %q not found", tag), nil)
	}
	svcName := service.TunnelServiceName(tag)
	if err := service.Stop(svcName); err != nil {
		return actions.NewError(actions.TunnelStop, "failed to stop service", err)
	}
	ctx.Output.Success(fmt.Sprintf("Tunnel %q stopped", tag))
	return nil
}

func handleTunnelStatus(ctx *actions.Context) error {
	cfg := ctx.Config.(*config.Config)
	tag := ctx.GetArg("tag")

	if tag != "" {
		tunnel := cfg.GetTunnel(tag)
		if tunnel == nil {
			return actions.NewError(actions.TunnelStatus, fmt.Sprintf("tunnel %q not found", tag), nil)
		}
		return showTunnelStatus(ctx.Output, tunnel)
	}

	// Show all tunnels
	for i := range cfg.Tunnels {
		if err := showTunnelStatus(ctx.Output, &cfg.Tunnels[i]); err != nil {
			ctx.Output.Warning(fmt.Sprintf("Error getting status for %q: %v", cfg.Tunnels[i].Tag, err))
		}
	}
	if len(cfg.Tunnels) == 0 {
		ctx.Output.Info("No tunnels configured. Run 'slipgate tunnel add' to create one.")
	}
	return nil
}

func showTunnelStatus(out actions.OutputWriter, tunnel *config.TunnelConfig) error {
	svcName := service.TunnelServiceName(tunnel.Tag)
	status, err := service.Status(svcName)
	if err != nil {
		status = "unknown"
	}
	out.Print(fmt.Sprintf("  %-15s %-12s %-8s %-25s %s",
		tunnel.Tag, tunnel.Transport, tunnel.Backend, tunnel.Domain, status))
	return nil
}

func handleTunnelLogs(ctx *actions.Context) error {
	cfg := ctx.Config.(*config.Config)
	tag := ctx.GetArg("tag")
	if tag == "" {
		return actions.NewError(actions.TunnelLogs, "tunnel tag is required", nil)
	}
	tunnel := cfg.GetTunnel(tag)
	if tunnel == nil {
		return actions.NewError(actions.TunnelLogs, fmt.Sprintf("tunnel %q not found", tag), nil)
	}
	lines := ctx.GetArg("lines")
	if lines == "" {
		lines = "50"
	}
	svcName := service.TunnelServiceName(tag)
	output, err := service.Logs(svcName, lines)
	if err != nil {
		return actions.NewError(actions.TunnelLogs, "failed to get logs", err)
	}
	ctx.Output.Print(output)
	return nil
}
