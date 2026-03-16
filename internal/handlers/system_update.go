package handlers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/anonvector/slipgate/internal/actions"
	"github.com/anonvector/slipgate/internal/binary"
	"github.com/anonvector/slipgate/internal/config"
	"github.com/anonvector/slipgate/internal/version"
)

func handleSystemUpdate(ctx *actions.Context) error {
	out := ctx.Output
	out.Info("Current version: " + version.String())
	out.Info("Downloading latest slipgate...")

	downloadURL := fmt.Sprintf("%s/latest/download/slipgate-%s-%s",
		"https://github.com/anonvector/slipgate/releases", runtime.GOOS, runtime.GOARCH)

	tmpPath, err := binary.Download(downloadURL)
	if err != nil {
		return actions.NewError(actions.SystemUpdate, "failed to download update", err)
	}
	defer os.Remove(tmpPath)

	execPath, err := os.Executable()
	if err != nil {
		return actions.NewError(actions.SystemUpdate, "failed to find current binary", err)
	}

	if runtime.GOOS == "linux" {
		if err := os.Rename(tmpPath, execPath); err != nil {
			cpCmd := exec.Command("cp", tmpPath, execPath)
			if err := cpCmd.Run(); err != nil {
				return actions.NewError(actions.SystemUpdate, "failed to replace binary", err)
			}
		}
		os.Chmod(execPath, 0755)
	}

	out.Success("slipgate updated")

	// Update transport binaries
	out.Print("")
	out.Info("Updating transport binaries...")

	transportBins := []string{"dnstt-server", "slipstream-server", "caddy-naive", "microsocks"}
	for _, name := range transportBins {
		binPath := filepath.Join(config.DefaultBinDir, name)
		if _, err := os.Stat(binPath); os.IsNotExist(err) {
			continue // not installed, skip
		}

		out.Info(fmt.Sprintf("  Updating %s...", name))

		// Remove old binary and re-download
		os.Remove(binPath)
		if err := binary.EnsureInstalled(name); err != nil {
			out.Warning(fmt.Sprintf("  Failed to update %s: %v", name, err))
			continue
		}
		out.Success(fmt.Sprintf("  %s updated", name))
	}

	out.Print("")
	out.Success("Update complete!")
	out.Info("Restart services to use new binaries: sudo systemctl restart slipgate-*")

	return nil
}
