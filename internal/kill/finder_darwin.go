//go:build darwin

package kill

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type darwinFinder struct{}

func (darwinFinder) FindListeners(ctx context.Context, port int) ([]Process, error) {
	lsof, err := exec.LookPath("lsof")
	if err != nil {
		lsof = "/usr/sbin/lsof"
	}
	cmd := exec.CommandContext(ctx, lsof, "-nP", fmt.Sprintf("-iTCP:%d", port), "-sTCP:LISTEN", "-t")
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) == 0 && len(out) == 0 {
			return nil, nil
		}
		return nil, fmt.Errorf("lsof: %w: %s", err, trimOutput(out))
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	processes := make([]Process, 0, len(lines))
	seen := make(map[int]bool, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pid, err := strconv.Atoi(line)
		if err != nil {
			return nil, fmt.Errorf("parse lsof pid %q: %w", line, err)
		}
		if seen[pid] {
			continue
		}
		seen[pid] = true
		processes = append(processes, Process{
			PID:  pid,
			Name: processName(pid),
		})
	}
	return processes, nil
}
