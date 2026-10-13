//go:build windows

package kill

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type windowsKiller struct{}

func platformKiller() Killer {
	return windowsKiller{}
}

func (windowsKiller) Kill(ctx context.Context, pid int, force bool) error {
	args := []string{"/PID", fmt.Sprintf("%d", pid)}
	if force {
		args = append(args, "/F")
	}
	taskkill := filepath.Join(os.Getenv("SystemRoot"), "System32", "taskkill.exe")
	cmd := exec.CommandContext(ctx, taskkill, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("taskkill: %w: %s", err, trimOutput(out))
	}
	return nil
}

func processName(int) string {
	return ""
}
