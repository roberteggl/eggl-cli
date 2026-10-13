package kill

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/roberteggl/eggl-cli/internal/ui"
)

type Process struct {
	PID  int
	Name string
}

type Options struct {
	Port   int
	DryRun bool
	Yes    bool
	Force  bool
	Finder Finder
	Killer Killer
	Input  io.Reader
	Output io.Writer
}

type Result struct {
	Found     []Process
	Killed    []Process
	DryRun    bool
	Cancelled bool
}

func Run(ctx context.Context, opts Options) (*Result, error) {
	if opts.Port < 1 || opts.Port > 65535 {
		return nil, fmt.Errorf("port must be between 1 and 65535")
	}

	finder := opts.Finder
	if finder == nil {
		finder = DefaultFinder()
	}
	killer := opts.Killer
	if killer == nil {
		killer = DefaultKiller()
	}

	found, err := finder.FindListeners(ctx, opts.Port)
	if err != nil {
		return nil, err
	}
	found = filterProtected(found)

	result := &Result{
		Found:  found,
		DryRun: opts.DryRun,
	}
	if len(found) == 0 || opts.DryRun {
		return result, nil
	}

	if !opts.Yes {
		ok, err := confirmKill(opts, len(found))
		if err != nil {
			return result, err
		}
		if !ok {
			result.Cancelled = true
			return result, nil
		}
	}

	killed := make([]Process, 0, len(found))
	for _, proc := range found {
		if err := killer.Kill(ctx, proc.PID, opts.Force); err != nil {
			return result, fmt.Errorf("kill pid %d: %w", proc.PID, err)
		}
		killed = append(killed, proc)
	}
	result.Killed = killed
	return result, nil
}

func confirmKill(opts Options, count int) (bool, error) {
	in := opts.Input
	if in == nil {
		in = os.Stdin
	}
	out := opts.Output
	if out == nil {
		out = os.Stderr
	}
	if !ui.IsInteractiveInput(in) {
		return false, fmt.Errorf("not a terminal; use --yes to confirm kills")
	}
	prompt := fmt.Sprintf("Kill %d process(es) on port %d? [y/N]: ", count, opts.Port)
	return ui.ConfirmPrompt(out, in, prompt)
}

func filterProtected(processes []Process) []Process {
	self := os.Getpid()
	filtered := make([]Process, 0, len(processes))
	seen := make(map[int]bool, len(processes))
	for _, proc := range processes {
		if proc.PID <= 1 || proc.PID == self || seen[proc.PID] {
			continue
		}
		seen[proc.PID] = true
		filtered = append(filtered, proc)
	}
	return filtered
}
