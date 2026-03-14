package handlers

import (
	"fmt"
	"runtime"

	"github.com/anonvector/slipgate/internal/actions"
	"github.com/anonvector/slipgate/internal/binary"
	"github.com/anonvector/slipgate/internal/config"
	"github.com/anonvector/slipgate/internal/network"
	"github.com/anonvector/slipgate/internal/prompt"
	"github.com/anonvector/slipgate/internal/proxy"
	"github.com/anonvector/slipgate/internal/system"
)

func handleSystemInstall(ctx *actions.Context) error {
	out := ctx.Output

	if runtime.GOOS != "linux" {
		return actions.NewError(actions.SystemInstall, "slipgate only supports Linux servers", nil)
	}

	out.Print("")
	out.Print("  Which transports do you want to install?")

	transports, err := prompt.MultiSelect("Transports", actions.TransportOptions)
	if err != nil {
		return err
	}
	if len(transports) == 0 {
		return actions.NewError(actions.SystemInstall, "no transports selected", nil)
	}

	// Create system user
	out.Info("Creating system user 'slipgate'...")
	if err := system.EnsureUser(); err != nil {
		return actions.NewError(actions.SystemInstall, "failed to create system user", err)
	}

	// Create directories
	for _, dir := range []string{config.DefaultConfigDir, config.DefaultTunnelDir} {
		if err := system.EnsureDir(dir, config.SystemUser); err != nil {
			return actions.NewError(actions.SystemInstall, fmt.Sprintf("failed to create %s", dir), err)
		}
	}

	// Download binaries
	out.Info("Downloading binaries...")
	needsSOCKS := false
	for _, t := range transports {
		bin, ok := config.TransportBinaries[t]
		if !ok {
			continue
		}
		out.Info(fmt.Sprintf("  Downloading %s...", bin))
		if err := binary.EnsureInstalled(bin); err != nil {
			return actions.NewError(actions.SystemInstall, fmt.Sprintf("failed to download %s", bin), err)
		}
		out.Success(fmt.Sprintf("  %s (%s/%s)", bin, runtime.GOOS, runtime.GOARCH))

		if t != config.TransportNaive {
			needsSOCKS = true
		}
	}

	// Install microsocks if any DNS tunnel selected
	if needsSOCKS {
		out.Info("  Downloading microsocks...")
		if err := binary.EnsureInstalled("microsocks"); err != nil {
			return actions.NewError(actions.SystemInstall, "failed to download microsocks", err)
		}
		if err := proxy.SetupMicrosocks(); err != nil {
			out.Warning("Failed to setup microsocks service: " + err.Error())
		}
		out.Success("  microsocks")
	}

	// Configure firewall
	out.Info("Configuring firewall...")
	needsDNS := false
	needsHTTPS := false
	for _, t := range transports {
		switch t {
		case config.TransportDNSTT, config.TransportSlipstream:
			needsDNS = true
		case config.TransportNaive:
			needsHTTPS = true
		}
	}
	if needsDNS {
		if err := network.AllowPort(53, "udp"); err != nil {
			out.Warning("Failed to open port 53/udp: " + err.Error())
		}
	}
	if needsHTTPS {
		if err := network.AllowPort(443, "tcp"); err != nil {
			out.Warning("Failed to open port 443/tcp: " + err.Error())
		}
	}

	// Write default config
	cfg := config.Default()
	if err := cfg.Save(); err != nil {
		return actions.NewError(actions.SystemInstall, "failed to write config", err)
	}

	out.Print("")
	out.Success("Installation complete!")
	out.Print("")
	out.Print("  Add a tunnel:")
	out.Print("    slipgate tunnel add")
	out.Print("")

	return nil
}
