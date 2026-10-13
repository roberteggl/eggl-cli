package dedash

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestFile(root, name, content string) error {
	return os.WriteFile(filepath.Join(root, name), []byte(content), 0o644)
}

func defaultOpts(root string) Options {
	return Options{Root: root, Yes: true}
}

func readTestFile(root, name string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func TestRunReplacesEmDash(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "readme.md")
	if err := os.WriteFile(path, []byte("hello \u2014 world \u2014 again"), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := Run(context.Background(), defaultOpts(root))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if report.Modified != 1 {
		t.Fatalf("Modified = %d, want 1", report.Modified)
	}
	if TotalReplacements(report) != 2 {
		t.Fatalf("TotalReplacements = %d, want 2", TotalReplacements(report))
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello - world - again" {
		t.Fatalf("file content = %q, want %q", got, "hello - world - again")
	}
}

func TestRunDryRunDoesNotWrite(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "readme.md")
	original := "hello \u2014 world"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := Run(context.Background(), Options{Root: root, DryRun: true})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if report.Modified != 1 {
		t.Fatalf("Modified = %d, want 1", report.Modified)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != original {
		t.Fatalf("file content = %q, want unchanged %q", got, original)
	}
}

func TestRunSkipsBinaryFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "image.png")
	if err := os.WriteFile(path, []byte("fake\x00png - dash"), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := Run(context.Background(), defaultOpts(root))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if report.Modified != 0 {
		t.Fatalf("Modified = %d, want 0", report.Modified)
	}
	if report.Skipped != 1 {
		t.Fatalf("Skipped = %d, want 1", report.Skipped)
	}
}

func TestRunSkipsNodeModules(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "node_modules", "pkg", "index.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("hello \u2014 world"), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := Run(context.Background(), defaultOpts(root))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if report.Modified != 0 {
		t.Fatalf("Modified = %d, want 0", report.Modified)
	}
	if report.Scanned != 0 {
		t.Fatalf("Scanned = %d, want 0", report.Scanned)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello \u2014 world" {
		t.Fatalf("file content = %q, want unchanged", got)
	}
}

func TestRunDoesNotReplaceEnDash(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "readme.md")
	original := "range \u2013 value"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := Run(context.Background(), defaultOpts(root))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if report.Modified != 0 {
		t.Fatalf("Modified = %d, want 0", report.Modified)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != original {
		t.Fatalf("file content = %q, want unchanged %q", got, original)
	}
}

func TestRunSkipsBuildOutputDirs(t *testing.T) {
	root := t.TempDir()
	cases := []struct {
		dir  string
		file string
	}{
		{"target", "classes/readme.md"},
		{".gradle", "caches/readme.md"},
		{"__pycache__", "module/readme.md"},
	}
	for _, tc := range cases {
		path := filepath.Join(root, tc.dir, tc.file)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("hello \u2014 world"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	report, err := Run(context.Background(), defaultOpts(root))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Scanned != 0 {
		t.Fatalf("Scanned = %d, want 0", report.Scanned)
	}
	if report.Modified != 0 {
		t.Fatalf("Modified = %d, want 0", report.Modified)
	}
}

func TestRunSkipsJarFiles(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "lib", "app.jar")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("PK\x03\x04fake jar"), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := Run(context.Background(), defaultOpts(root))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Scanned != 0 {
		t.Fatalf("Scanned = %d, want 0", report.Scanned)
	}
	if report.Skipped != 1 {
		t.Fatalf("Skipped = %d, want 1", report.Skipped)
	}
}

func TestRunInvalidPath(t *testing.T) {
	_, err := Run(context.Background(), Options{Root: filepath.Join(t.TempDir(), "missing")})
	if err == nil {
		t.Fatal("expected error for missing path")
	}
}

func TestRunScannedWithoutEmDash(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "plain.md")
	if err := os.WriteFile(path, []byte("no special dashes here"), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := Run(context.Background(), defaultOpts(root))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Scanned != 1 {
		t.Fatalf("Scanned = %d, want 1", report.Scanned)
	}
	if report.Modified != 0 {
		t.Fatalf("Modified = %d, want 0", report.Modified)
	}
}

func TestRunMultipleFiles(t *testing.T) {
	root := t.TempDir()
	for name, content := range map[string]string{
		"a.md": "one \u2014 dash",
		"b.md": "two \u2014 dashes \u2014 here",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	report, err := Run(context.Background(), defaultOpts(root))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Modified != 2 {
		t.Fatalf("Modified = %d, want 2", report.Modified)
	}
	if TotalReplacements(report) != 3 {
		t.Fatalf("TotalReplacements = %d, want 3", TotalReplacements(report))
	}
}

func TestRunSkipsNonRegularFile(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "readme.md")
	if err := os.WriteFile(target, []byte("hello \u2014 world"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link.md")
	if err := os.Symlink(target, link); err != nil {
		t.Skip("symlinks not supported:", err)
	}

	report, err := Run(context.Background(), defaultOpts(root))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Scanned != 1 {
		t.Fatalf("Scanned = %d, want 1 (regular file only)", report.Scanned)
	}
	if report.Skipped != 1 {
		t.Fatalf("Skipped = %d, want 1 (symlink)", report.Skipped)
	}
}

func TestRunFileNotDirectory(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "file.txt")
	if err := os.WriteFile(path, []byte("text"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Run(context.Background(), Options{Root: path})
	if err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("expected not a directory error, got %v", err)
	}
}

func TestRunSkipsHiddenFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("secret \u2014 value"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "visible.md"), []byte("hello \u2014 world"), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := Run(context.Background(), defaultOpts(root))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Scanned != 1 {
		t.Fatalf("Scanned = %d, want 1", report.Scanned)
	}
	if report.Skipped != 1 {
		t.Fatalf("Skipped = %d, want 1 (.env)", report.Skipped)
	}
}

func TestRunIncludeHidden(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("secret \u2014 value"), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := Run(context.Background(), Options{Root: root, IncludeHidden: true, Yes: true})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Modified != 1 {
		t.Fatalf("Modified = %d, want 1", report.Modified)
	}
}

func TestRunSkipsOversizedFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "big.md")
	if err := os.WriteFile(path, []byte("\u2014"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(path, MaxFileSize+1); err != nil {
		t.Fatal(err)
	}

	report, err := Run(context.Background(), defaultOpts(root))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Scanned != 0 {
		t.Fatalf("Scanned = %d, want 0", report.Scanned)
	}
	if report.Skipped != 1 {
		t.Fatalf("Skipped = %d, want 1", report.Skipped)
	}
}

func TestRunRespectsContextCancel(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "readme.md"), []byte("hello \u2014 world"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Run(ctx, defaultOpts(root))
	if err != context.Canceled {
		t.Fatalf("Run() error = %v, want context.Canceled", err)
	}
}

type fakeGitDiff struct {
	inside      bool
	root        string
	files       []string
	filesSince  map[string][]string
	insideE     error
	rootE       error
	filesE      error
	filesSinceE error
}

func (f *fakeGitDiff) InsideWorkTree(context.Context) (bool, error) {
	return f.inside, f.insideE
}

func (f *fakeGitDiff) RepoRoot(context.Context) (string, error) {
	return f.root, f.rootE
}

func (f *fakeGitDiff) ChangedFilePaths(context.Context) ([]string, error) {
	return f.files, f.filesE
}

func (f *fakeGitDiff) ChangedFilePathsSince(_ context.Context, base string) ([]string, error) {
	if f.filesSinceE != nil {
		return nil, f.filesSinceE
	}
	if f.filesSince != nil {
		return f.filesSince[base], nil
	}
	return f.files, nil
}

func TestRunGitDiffOnlyChangedFiles(t *testing.T) {
	root := t.TempDir()
	if err := writeTestFile(root, "changed.md", "hello \u2014 world"); err != nil {
		t.Fatal(err)
	}
	if err := writeTestFile(root, "unchanged.md", "also \u2014 dash"); err != nil {
		t.Fatal(err)
	}

	report, err := Run(context.Background(), Options{
		Root:    root,
		Yes:     true,
		GitDiff: true,
		Git: &fakeGitDiff{
			inside: true,
			root:   root,
			files:  []string{"changed.md"},
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Modified != 1 {
		t.Fatalf("Modified = %d, want 1", report.Modified)
	}

	got, err := readTestFile(root, "changed.md")
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello - world" {
		t.Fatalf("changed.md = %q, want %q", got, "hello - world")
	}

	got, err = readTestFile(root, "unchanged.md")
	if err != nil {
		t.Fatal(err)
	}
	if got != "also \u2014 dash" {
		t.Fatalf("unchanged.md should be untouched, got %q", got)
	}
}

func TestRunGitDiffBaseOnlyBranchFiles(t *testing.T) {
	root := t.TempDir()
	if err := writeTestFile(root, "branch.md", "hello \u2014 world"); err != nil {
		t.Fatal(err)
	}
	if err := writeTestFile(root, "main-only.md", "also \u2014 dash"); err != nil {
		t.Fatal(err)
	}

	report, err := Run(context.Background(), Options{
		Root:        root,
		Yes:         true,
		GitDiffBase: "main",
		Git: &fakeGitDiff{
			inside: true,
			root:   root,
			filesSince: map[string][]string{
				"main": {"branch.md"},
			},
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Modified != 1 {
		t.Fatalf("Modified = %d, want 1", report.Modified)
	}

	got, err := readTestFile(root, "branch.md")
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello - world" {
		t.Fatalf("branch.md = %q, want %q", got, "hello - world")
	}

	got, err = readTestFile(root, "main-only.md")
	if err != nil {
		t.Fatal(err)
	}
	if got != "also \u2014 dash" {
		t.Fatalf("main-only.md should be untouched, got %q", got)
	}
}

func TestRunGitDiffSkipsDeletedPaths(t *testing.T) {
	root := t.TempDir()
	if err := writeTestFile(root, "keep.md", "hello \u2014 world"); err != nil {
		t.Fatal(err)
	}

	report, err := Run(context.Background(), Options{
		Root:    root,
		Yes:     true,
		GitDiff: true,
		Git: &fakeGitDiff{
			inside: true,
			root:   root,
			files:  []string{"keep.md", "removed.md"},
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Modified != 1 {
		t.Fatalf("Modified = %d, want 1", report.Modified)
	}
	if report.Skipped != 0 {
		t.Fatalf("Skipped = %d, want 0 (deleted paths are ignored)", report.Skipped)
	}
}

func TestRunGitDiffSkipsFileOutsidePath(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, "docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeTestFile(root, "root.md", "root \u2014 dash"); err != nil {
		t.Fatal(err)
	}
	if err := writeTestFile(docs, "docs.md", "docs \u2014 dash"); err != nil {
		t.Fatal(err)
	}

	report, err := Run(context.Background(), Options{
		Root:    docs,
		Yes:     true,
		GitDiff: true,
		Git: &fakeGitDiff{
			inside: true,
			root:   root,
			files:  []string{"root.md", "docs/docs.md"},
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Modified != 1 {
		t.Fatalf("Modified = %d, want 1", report.Modified)
	}
	if report.Changes[0].Path != "docs.md" {
		t.Fatalf("change path = %q, want docs.md", report.Changes[0].Path)
	}
}

func TestRunGitDiffRespectsExtensionFilter(t *testing.T) {
	root := t.TempDir()
	if err := writeTestFile(root, "readme.md", "md \u2014 dash"); err != nil {
		t.Fatal(err)
	}
	if err := writeTestFile(root, "notes.txt", "txt \u2014 dash"); err != nil {
		t.Fatal(err)
	}

	report, err := Run(context.Background(), Options{
		Root:       root,
		Yes:        true,
		GitDiff:    true,
		Extensions: []string{"md"},
		Git: &fakeGitDiff{
			inside: true,
			root:   root,
			files:  []string{"readme.md", "notes.txt"},
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Modified != 1 {
		t.Fatalf("Modified = %d, want 1", report.Modified)
	}
	if report.Skipped != 1 {
		t.Fatalf("Skipped = %d, want 1", report.Skipped)
	}
}

func TestRunGitDiffSkipsNonRegularFile(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "readme.md")
	if err := writeTestFile(root, "readme.md", "hello \u2014 world"); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link.md")
	if err := os.Symlink(target, link); err != nil {
		t.Skip("symlinks not supported:", err)
	}

	report, err := Run(context.Background(), Options{
		Root:    root,
		Yes:     true,
		GitDiff: true,
		Git: &fakeGitDiff{
			inside: true,
			root:   root,
			files:  []string{"readme.md", "link.md"},
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Modified != 1 {
		t.Fatalf("Modified = %d, want 1", report.Modified)
	}
	if report.Skipped != 1 {
		t.Fatalf("Skipped = %d, want 1", report.Skipped)
	}
}

func TestRunGitDiffSkipsOversizedFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "big.md")
	if err := os.WriteFile(path, []byte("\u2014"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(path, MaxFileSize+1); err != nil {
		t.Fatal(err)
	}

	report, err := Run(context.Background(), Options{
		Root:    root,
		Yes:     true,
		GitDiff: true,
		Git: &fakeGitDiff{
			inside: true,
			root:   root,
			files:  []string{"big.md"},
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Modified != 0 {
		t.Fatalf("Modified = %d, want 0", report.Modified)
	}
	if report.Skipped != 1 {
		t.Fatalf("Skipped = %d, want 1", report.Skipped)
	}
}

func TestRunGitDiffRepoRootError(t *testing.T) {
	root := t.TempDir()

	_, err := Run(context.Background(), Options{
		Root:    root,
		GitDiff: true,
		Git: &fakeGitDiff{
			inside: true,
			rootE:  errors.New("repo root unavailable"),
		},
	})
	if err == nil {
		t.Fatal("expected repo root error")
	}
	if !strings.Contains(err.Error(), "repo root unavailable") {
		t.Fatalf("error = %v", err)
	}
}

func TestRunGitDiffChangedPathsError(t *testing.T) {
	root := t.TempDir()

	_, err := Run(context.Background(), Options{
		Root:    root,
		GitDiff: true,
		Git: &fakeGitDiff{
			inside: true,
			root:   root,
			filesE: errors.New("diff failed"),
		},
	})
	if err == nil {
		t.Fatal("expected changed paths error")
	}
	if !strings.Contains(err.Error(), "diff failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestRunGitDiffBaseChangedPathsError(t *testing.T) {
	root := t.TempDir()

	_, err := Run(context.Background(), Options{
		Root:        root,
		GitDiffBase: "main",
		Git: &fakeGitDiff{
			inside:      true,
			root:        root,
			filesSinceE: errors.New("branch diff failed"),
		},
	})
	if err == nil {
		t.Fatal("expected branch diff error")
	}
	if !strings.Contains(err.Error(), "branch diff failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestRunGitDiffRespectsContextCancel(t *testing.T) {
	root := t.TempDir()
	if err := writeTestFile(root, "readme.md", "hello \u2014 world"); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Run(ctx, Options{
		Root:    root,
		GitDiff: true,
		Git: &fakeGitDiff{
			inside: true,
			root:   root,
			files:  []string{"readme.md"},
		},
	})
	if err != context.Canceled {
		t.Fatalf("Run() error = %v, want context.Canceled", err)
	}
}

func TestRunGitDiffIntegration(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	initDedashGitRepo(t, dir)
	writeDedashGitFile(t, dir, "readme.md", "original\n")
	runDedashGit(t, dir, "add", "readme.md")
	runDedashGit(t, dir, "commit", "-m", "init")
	writeDedashGitFile(t, dir, "readme.md", "hello \u2014 world\n")

	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origWD)
	})

	report, err := Run(context.Background(), Options{
		Root:    dir,
		GitDiff: true,
		Yes:     true,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Modified != 1 {
		t.Fatalf("Modified = %d, want 1", report.Modified)
	}
}

func TestRunGitDiffNotInsideWorkTree(t *testing.T) {
	root := t.TempDir()

	_, err := Run(context.Background(), Options{
		Root:    root,
		GitDiff: true,
		Git: &fakeGitDiff{
			inside: false,
		},
	})
	if err == nil {
		t.Fatal("expected error outside git work tree")
	}
	if !strings.Contains(err.Error(), "not inside a git work tree") {
		t.Fatalf("error = %v", err)
	}
}

func TestRunNotTerminalRequiresYes(t *testing.T) {
	root := t.TempDir()
	if err := writeTestFile(root, "readme.md", "hello \u2014 world"); err != nil {
		t.Fatal(err)
	}

	_, err := Run(context.Background(), Options{
		Root:  root,
		Input: strings.NewReader(""),
	})
	if err == nil {
		t.Fatal("expected error when stdin is not a terminal")
	}
	if !strings.Contains(err.Error(), "not a terminal") {
		t.Fatalf("error = %v", err)
	}
}

func initDedashGitRepo(t *testing.T, dir string) {
	t.Helper()
	runDedashGit(t, dir, "init")
	runDedashGit(t, dir, "config", "user.email", "test@example.com")
	runDedashGit(t, dir, "config", "user.name", "Test User")
}

func runDedashGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
}

func writeDedashGitFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
