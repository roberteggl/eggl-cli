package filescan

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/roberteggl/eggl-cli/internal/git"
	"github.com/roberteggl/eggl-cli/internal/ui"
)

// MaxFileSize is the largest regular file that will be read (50 MiB).
const MaxFileSize = 50 << 20

type GitDiffRunner interface {
	InsideWorkTree(ctx context.Context) (bool, error)
	RepoRoot(ctx context.Context) (string, error)
	ChangedFilePaths(ctx context.Context) ([]string, error)
	ChangedFilePathsSince(ctx context.Context, base string) ([]string, error)
}

// TransformFunc analyzes file bytes and returns how many replacements were
// made plus the rewritten content. A zero count means no change (content may be nil).
type TransformFunc func(data []byte) (count int, content []byte)

// CommonOptions are shared scan settings used by commands like dedash and eol.
type CommonOptions struct {
	Root          string
	DryRun        bool
	Yes           bool
	Extensions    []string
	IncludeHidden bool
	GitDiff       bool
	GitDiffBase   string
	Git           GitDiffRunner
	Input         io.Reader
	Output        io.Writer
}

type Options struct {
	CommonOptions
	Transform       TransformFunc
	ConfirmPrompt   func(total, files int) string
	FoundLogMessage string
}

type FileChange struct {
	Path         string
	Replacements int
}

type Report struct {
	Scanned   int
	Modified  int
	Skipped   int
	Changes   []FileChange
	Cancelled bool
}

type pendingChange struct {
	absPath string
	content []byte
	mode    fs.FileMode
	change  FileChange
}

func Run(ctx context.Context, opts Options) (*Report, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if opts.Transform == nil {
		return nil, fmt.Errorf("transform is required")
	}

	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}

	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("stat path: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", root)
	}

	extensions := NormalizeExtensions(opts.Extensions)

	report := &Report{}
	var pending []pendingChange

	slog.Debug("scanning directory",
		"root", root,
		"dry_run", opts.DryRun,
		"extensions", extensions,
		"include_hidden", opts.IncludeHidden,
		"git_diff", opts.GitDiff,
		"git_diff_base", opts.GitDiffBase,
	)

	if opts.GitDiff || opts.GitDiffBase != "" {
		if err := scanGitDiff(ctx, opts, root, extensions, report, &pending); err != nil {
			return nil, err
		}
	} else if err := walkDirectory(ctx, opts, root, extensions, report, &pending); err != nil {
		return nil, err
	}

	slog.Debug("scan complete",
		"scanned", report.Scanned,
		"modified", report.Modified,
		"skipped", report.Skipped,
	)

	total := TotalReplacements(report)
	if opts.DryRun || total == 0 {
		return report, nil
	}

	if !opts.Yes {
		ok, err := confirmWrites(opts, total, report.Modified)
		if err != nil {
			return report, err
		}
		if !ok {
			report.Cancelled = true
			return report, nil
		}
	}

	for _, item := range pending {
		if err := os.WriteFile(item.absPath, item.content, item.mode); err != nil {
			return report, err
		}
	}

	return report, nil
}

func confirmWrites(opts Options, total, files int) (bool, error) {
	in := opts.Input
	if in == nil {
		in = os.Stdin
	}
	out := opts.Output
	if out == nil {
		out = os.Stderr
	}
	if !ui.IsInteractiveInput(in) {
		return false, fmt.Errorf("not a terminal; use --yes to confirm writes")
	}
	prompt := "Continue? [y/N]: "
	if opts.ConfirmPrompt != nil {
		prompt = opts.ConfirmPrompt(total, files)
	}
	return ui.ConfirmPrompt(out, in, prompt)
}

func normalizeAbsPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

func gitRunner(opts Options) GitDiffRunner {
	if opts.Git != nil {
		return opts.Git
	}
	return git.CLI{}
}

func scanGitDiff(ctx context.Context, opts Options, root string, extensions []string, report *Report, pending *[]pendingChange) error {
	runner := gitRunner(opts)

	inside, err := runner.InsideWorkTree(ctx)
	if err != nil || !inside {
		return fmt.Errorf("not inside a git work tree")
	}

	repoRoot, err := runner.RepoRoot(ctx)
	if err != nil {
		return err
	}
	repoRoot, err = normalizeAbsPath(repoRoot)
	if err != nil {
		return fmt.Errorf("resolve repo root: %w", err)
	}
	scanRoot, err := normalizeAbsPath(root)
	if err != nil {
		return fmt.Errorf("resolve scan root: %w", err)
	}

	var changed []string
	if opts.GitDiffBase != "" {
		changed, err = runner.ChangedFilePathsSince(ctx, opts.GitDiffBase)
	} else {
		changed, err = runner.ChangedFilePaths(ctx)
	}
	if err != nil {
		return err
	}

	slog.Debug("git diff files", "count", len(changed), "base", opts.GitDiffBase)

	for _, rel := range changed {
		if err := ctx.Err(); err != nil {
			return err
		}
		abs := filepath.Join(repoRoot, rel)
		relToRoot, skip := gitDiffSkip(abs, rel, scanRoot, extensions, opts.IncludeHidden, report)
		if skip {
			continue
		}
		if err := considerFile(ctx, opts, abs, relToRoot, report, pending); err != nil {
			return err
		}
	}

	return nil
}

func gitDiffSkip(abs, rel, scanRoot string, extensions []string, includeHidden bool, report *Report) (string, bool) {
	relToRoot, err := filepath.Rel(scanRoot, abs)
	if err != nil || strings.HasPrefix(relToRoot, "..") {
		slog.Debug("skipping file", "path", rel, "reason", "outside scan root")
		return "", true
	}

	info, err := os.Lstat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Debug("skipping file", "path", rel, "reason", "deleted file")
			return "", true
		}
		slog.Debug("skipping file", "path", rel, "reason", err.Error())
		report.Skipped++
		return "", true
	}
	if !info.Mode().IsRegular() {
		slog.Debug("skipping file", "path", rel, "reason", "not a regular file")
		report.Skipped++
		return "", true
	}
	if skipReason := skipPath(info.Name(), extensions, includeHidden); skipReason != "" {
		slog.Debug("skipping file", "path", rel, "reason", skipReason)
		report.Skipped++
		return "", true
	}
	if info.Size() > MaxFileSize {
		slog.Debug("skipping file", "path", rel, "reason", "file too large", "size", info.Size())
		report.Skipped++
		return "", true
	}
	return relToRoot, false
}

type walkState struct {
	ctx        context.Context
	opts       Options
	root       string
	extensions []string
	report     *Report
	pending    *[]pendingChange
}

func walkDirectory(ctx context.Context, opts Options, root string, extensions []string, report *Report, pending *[]pendingChange) error {
	state := walkState{ctx: ctx, opts: opts, root: root, extensions: extensions, report: report, pending: pending}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if err := state.ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return walkSkipDir(path, state.root, d.Name(), state.opts.IncludeHidden)
		}
		return state.walkFile(path, d)
	})
	if err != nil {
		return fmt.Errorf("walk directory: %w", err)
	}
	return nil
}

func walkSkipDir(path, root, name string, includeHidden bool) error {
	if path == root {
		return nil
	}
	if !includeHidden && IsHiddenName(name) {
		slog.Debug("skipping directory", "path", path, "reason", "hidden path")
		return filepath.SkipDir
	}
	if ShouldSkipDir(name) {
		slog.Debug("skipping directory", "path", path, "reason", "ignored directory name")
		return filepath.SkipDir
	}
	return nil
}

func (s walkState) walkFile(path string, d fs.DirEntry) error {
	if !d.Type().IsRegular() {
		slog.Debug("skipping file", "path", path, "reason", "not a regular file")
		s.report.Skipped++
		return nil
	}

	if skipReason := skipPath(d.Name(), s.extensions, s.opts.IncludeHidden); skipReason != "" {
		slog.Debug("skipping file", "path", path, "reason", skipReason)
		s.report.Skipped++
		return nil
	}

	if fi, err := d.Info(); err == nil && fi.Size() > MaxFileSize {
		slog.Debug("skipping file", "path", path, "reason", "file too large", "size", fi.Size())
		s.report.Skipped++
		return nil
	}

	relPath, relErr := filepath.Rel(s.root, path)
	if relErr != nil {
		relPath = path
	}

	return considerFile(s.ctx, s.opts, path, relPath, s.report, s.pending)
}

func skipPath(name string, extensions []string, includeHidden bool) string {
	if !includeHidden && IsHiddenName(name) {
		return "hidden path"
	}
	if ShouldSkipFile(name) {
		return "ignored file name"
	}
	if !MatchesExtension(name, extensions) {
		return "extension not included"
	}
	return ""
}

func considerFile(ctx context.Context, opts Options, absPath, relPath string, report *Report, pending *[]pendingChange) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	report.Scanned++

	change, content, mode, err := analyzeFile(absPath, opts.Transform)
	if err != nil {
		slog.Debug("skipping file", "path", absPath, "reason", err.Error())
		report.Skipped++
		return nil
	}

	if change.Replacements == 0 {
		return nil
	}

	change.Path = relPath

	report.Modified++
	report.Changes = append(report.Changes, change)
	*pending = append(*pending, pendingChange{
		absPath: absPath,
		content: content,
		mode:    mode,
		change:  change,
	})
	msg := opts.FoundLogMessage
	if msg == "" {
		msg = "found changes"
	}
	slog.Debug(msg,
		"path", relPath,
		"replacements", change.Replacements,
		"dry_run", opts.DryRun,
	)
	return nil
}

func analyzeFile(path string, transform TransformFunc) (FileChange, []byte, fs.FileMode, error) {
	info, err := os.Stat(path)
	if err != nil {
		return FileChange{}, nil, 0, err
	}
	if info.Size() > MaxFileSize {
		return FileChange{}, nil, 0, fmt.Errorf("file too large")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return FileChange{}, nil, 0, err
	}

	if IsBinaryContent(data) {
		return FileChange{}, nil, 0, fmt.Errorf("binary content")
	}

	count, content := transform(data)
	if count == 0 {
		return FileChange{Path: path, Replacements: 0}, nil, info.Mode(), nil
	}

	return FileChange{Path: path, Replacements: count}, content, info.Mode(), nil
}

func TotalReplacements(report *Report) int {
	total := 0
	for _, change := range report.Changes {
		total += change.Replacements
	}
	return total
}
