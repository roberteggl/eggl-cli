//go:build windows

package kill

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type windowsFinder struct{}

func (windowsFinder) FindListeners(ctx context.Context, port int) ([]Process, error) {
	netstat := filepath.Join(os.Getenv("SystemRoot"), "System32", "netstat.exe")
	cmd := exec.CommandContext(ctx, netstat, "-ano")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("netstat: %w: %s", err, trimOutput(out))
	}

	target := fmt.Sprintf(":%d", port)
	lines := strings.Split(string(out), "\n")
	byPID := make(map[int]Process)

	for _, line := range lines {
		upper := strings.ToUpper(strings.TrimSpace(line))
		if !strings.Contains(upper, "TCP") || !strings.Contains(upper, "LISTENING") {
			continue
		}
		if !strings.Contains(line, target) {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		local := fields[1]
		if !strings.HasSuffix(local, target) {
			continue
		}

		pid, err := strconv.Atoi(fields[len(fields)-1])
		if err != nil {
			continue
		}
		byPID[pid] = Process{PID: pid}
	}

	processes := make([]Process, 0, len(byPID))
	for _, proc := range byPID {
		processes = append(processes, proc)
	}
	return processes, nil
}
