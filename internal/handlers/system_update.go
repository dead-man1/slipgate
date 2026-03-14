package handlers

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/anonvector/slipgate/internal/actions"
	"github.com/anonvector/slipgate/internal/binary"
	"github.com/anonvector/slipgate/internal/version"
)

func handleSystemUpdate(ctx *actions.Context) error {
	out := ctx.Output
	out.Info("Current version: " + version.String())
	out.Info("Checking for updates...")

	newVersion, downloadURL, err := binary.CheckUpdate()
	if err != nil {
		return actions.NewError(actions.SystemUpdate, "failed to check for updates", err)
	}

	if newVersion == "" {
		out.Success("Already up to date")
		return nil
	}

	out.Info(fmt.Sprintf("New version available: %s", newVersion))
	out.Info("Downloading...")

	tmpPath, err := binary.Download(downloadURL)
	if err != nil {
		return actions.NewError(actions.SystemUpdate, "failed to download update", err)
	}
	defer os.Remove(tmpPath)

	// Replace current binary
	execPath, err := os.Executable()
	if err != nil {
		return actions.NewError(actions.SystemUpdate, "failed to find current binary", err)
	}

	if runtime.GOOS == "linux" {
		if err := os.Rename(tmpPath, execPath); err != nil {
			// Try copy if rename fails (cross-device)
			cpCmd := exec.Command("cp", tmpPath, execPath)
			if err := cpCmd.Run(); err != nil {
				return actions.NewError(actions.SystemUpdate, "failed to replace binary", err)
			}
		}
		os.Chmod(execPath, 0755)
	}

	out.Success(fmt.Sprintf("Updated to %s", newVersion))
	return nil
}
