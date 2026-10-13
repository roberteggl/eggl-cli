package eol

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
)

func TestRunInvalidPath(t *testing.T) {
	_, err := Run(context.Background(), Options{Root: filepath.Join(t.TempDir(), "missing")})
	if err == nil {
		t.Fatal("expected error for missing path")
	}
}

func TestRunFileNotDirectory(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "file.txt")
	if err := os.WriteFile(path, []byte("text\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Run(context.Background(), Options{Root: path})
	if err == nil {
		t.Fatal("expected error when root is a file")
	}
}

func TestRunScannedWithoutEOLChanges(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "plain.md", []byte("no crlf here\n"))

	report, err := Run(context.Background(), defaultOpts(root))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Scanned != 1 || report.Modified != 0 {
		t.Fatalf("report = %+v", report)
	}
}

func TestRunMultipleFiles(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "a.md", []byte("one\r\n"))
	writeTestFile(t, root, "b.md", []byte("two\r\nlines\r\n"))

	report, err := Run(context.Background(), defaultOpts(root))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Modified != 2 || TotalReplacements(report) != 3 {
		t.Fatalf("report = %+v", report)
	}
}

func TestRunSkipsBuildOutputDirs(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "node_modules", "pkg", "readme.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("hello\r\nworld\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := Run(context.Background(), defaultOpts(root))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Scanned != 0 || report.Modified != 0 {
		t.Fatalf("report = %+v", report)
	}
}

func TestRunSkipsJarFiles(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "lib", "app.jar")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("PK\x03\x04fake\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := Run(context.Background(), defaultOpts(root))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Scanned != 0 || report.Skipped != 1 {
		t.Fatalf("report = %+v", report)
	}
}

func TestRunSkipsHiddenUnlessIncluded(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".env", []byte("secret\r\n"))

	report, err := Run(context.Background(), defaultOpts(root))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Scanned != 0 {
		t.Fatalf("report = %+v", report)
	}

	report, err = Run(context.Background(), Options{Root: root, IncludeHidden: true, Yes: true})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Modified != 1 {
		t.Fatalf("report = %+v", report)
	}
}

func TestRunSkipsNonRegularFile(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "readme.md")
	writeTestFile(t, root, "readme.md", []byte("hello\r\n"))
	link := filepath.Join(root, "link.md")
	if err := os.Symlink(target, link); err != nil {
		t.Skip("symlinks not supported:", err)
	}

	report, err := Run(context.Background(), defaultOpts(root))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Scanned != 1 || report.Skipped != 1 {
		t.Fatalf("report = %+v", report)
	}
}

func TestRunRespectsContextCancel(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "readme.md", []byte("hello\r\n"))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Run(ctx, defaultOpts(root))
	if err != context.Canceled {
		t.Fatalf("Run() error = %v, want context.Canceled", err)
	}
}

func TestRunCancelled(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "readme.md", []byte("hello\r\n"))

	tty, master := openEOLTTY(t)
	defer func() { _ = master.Close() }()
	defer func() { _ = tty.Close() }()

	done := make(chan struct{})
	go func() {
		time.Sleep(20 * time.Millisecond)
		_, _ = master.Write([]byte("n\n"))
		close(done)
	}()

	report, err := Run(context.Background(), Options{
		Root:   root,
		Input:  tty,
		Output: tty,
	})
	<-done
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !report.Cancelled {
		t.Fatal("expected cancelled result")
	}
}

func openEOLTTY(t *testing.T) (*os.File, *os.File) {
	t.Helper()

	master, tty, err := pty.Open()
	if err != nil {
		t.Skip("pty:", err)
	}
	t.Cleanup(func() {
		_ = master.Close()
		_ = tty.Close()
	})
	return tty, master
}

func TestRunGitDiffSkipsDeletedPaths(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "keep.md", []byte("hello\r\n"))

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
	if report.Modified != 1 || report.Skipped != 0 {
		t.Fatalf("report = %+v", report)
	}
}

func TestRunGitDiffSkipsFileOutsidePath(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, "docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, root, "root.md", []byte("root\r\n"))
	writeTestFile(t, docs, "docs.md", []byte("docs\r\n"))

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
	if report.Modified != 1 || report.Changes[0].Path != "docs.md" {
		t.Fatalf("report = %+v", report)
	}
}

func TestRunGitDiffRespectsExtensionFilter(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "readme.md", []byte("md\r\n"))
	writeTestFile(t, root, "notes.txt", []byte("txt\r\n"))

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
	if report.Modified != 1 || report.Skipped != 1 {
		t.Fatalf("report = %+v", report)
	}
}

func TestRunGitDiffSkipsNonRegularFile(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "readme.md")
	writeTestFile(t, root, "readme.md", []byte("hello\r\n"))
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
	if report.Modified != 1 || report.Skipped != 1 {
		t.Fatalf("report = %+v", report)
	}
}

func TestRunGitDiffSkipsOversizedFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "big.md")
	if err := os.WriteFile(path, []byte("\r\n"), 0o644); err != nil {
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
	if report.Modified != 0 || report.Skipped != 1 {
		t.Fatalf("report = %+v", report)
	}
}

func TestRunGitDiffRepoRootError(t *testing.T) {
	_, err := Run(context.Background(), Options{
		Root:    t.TempDir(),
		GitDiff: true,
		Git: &fakeGitDiff{
			inside: true,
			rootE:  errors.New("repo root unavailable"),
		},
	})
	if err == nil || !strings.Contains(err.Error(), "repo root unavailable") {
		t.Fatalf("error = %v", err)
	}
}

func TestRunGitDiffBaseChangedPathsError(t *testing.T) {
	_, err := Run(context.Background(), Options{
		Root:        t.TempDir(),
		GitDiffBase: "main",
		Git: &fakeGitDiff{
			inside:      true,
			root:        t.TempDir(),
			filesSinceE: errors.New("branch diff failed"),
		},
	})
	if err == nil || !strings.Contains(err.Error(), "branch diff failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestRunGitDiffRespectsContextCancel(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "readme.md", []byte("hello\r\n"))

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
	runEOLGit(t, dir, "init")
	runEOLGit(t, dir, "config", "user.email", "test@example.com")
	runEOLGit(t, dir, "config", "user.name", "Test User")
	writeEOLGitFile(t, dir, "readme.md", "original\n")
	runEOLGit(t, dir, "add", "readme.md")
	runEOLGit(t, dir, "commit", "-m", "init")
	writeEOLGitFile(t, dir, "readme.md", "hello\r\nworld\r\n")

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

func runEOLGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
}

func writeEOLGitFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunSkipsOversizedFileInWalk(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "big.md")
	if err := os.WriteFile(path, []byte("\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(path, MaxFileSize+1); err != nil {
		t.Fatal(err)
	}

	report, err := Run(context.Background(), defaultOpts(root))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Modified != 0 || report.Skipped != 1 {
		t.Fatalf("report = %+v", report)
	}
}

func TestRunContextCancelBeforeScan(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Run(ctx, Options{Root: t.TempDir(), Yes: true})
	if err != context.Canceled {
		t.Fatalf("Run() error = %v, want context.Canceled", err)
	}
}

func TestRunConfirmPromptError(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "readme.md", []byte("hello\r\n"))

	tty, master := openEOLTTY(t)
	done := make(chan struct{})
	go func() {
		time.Sleep(20 * time.Millisecond)
		_ = master.Close()
		close(done)
	}()

	_, err := Run(context.Background(), Options{
		Root:   root,
		Input:  tty,
		Output: tty,
	})
	<-done
	if err == nil {
		t.Fatal("expected confirm prompt error")
	}
}

func TestRunConfirmedYes(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "readme.md", []byte("hello\r\n"))

	tty, master := openEOLTTY(t)
	defer func() { _ = master.Close() }()
	defer func() { _ = tty.Close() }()

	done := make(chan struct{})
	go func() {
		time.Sleep(20 * time.Millisecond)
		_, _ = master.Write([]byte("y\n"))
		close(done)
	}()

	report, err := Run(context.Background(), Options{
		Root:   root,
		Input:  tty,
		Output: tty,
	})
	<-done
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Modified != 1 {
		t.Fatalf("report = %+v", report)
	}
}

func TestRunGitDiffSkipsUnreadableFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "secret.md")
	if err := os.WriteFile(path, []byte("secret\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

	report, err := Run(context.Background(), Options{
		Root:    root,
		Yes:     true,
		GitDiff: true,
		Git: &fakeGitDiff{
			inside: true,
			root:   root,
			files:  []string{"secret.md"},
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Skipped != 1 || report.Modified != 0 {
		t.Fatalf("report = %+v", report)
	}
}

func TestRunWalkSkipsDotDirectory(t *testing.T) {
	root := t.TempDir()
	hiddenDir := filepath.Join(root, ".hidden")
	if err := os.MkdirAll(hiddenDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, hiddenDir, "readme.md", []byte("hello\r\n"))

	report, err := Run(context.Background(), defaultOpts(root))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Scanned != 0 {
		t.Fatalf("report = %+v", report)
	}
}

func TestRunWriteFileError(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "readme.md")
	writeTestFile(t, root, "readme.md", []byte("hello\r\n"))
	if err := os.Chmod(path, 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(root, 0o755)
		_ = os.Chmod(path, 0o644)
	})

	_, err := Run(context.Background(), Options{Root: root, Yes: true})
	if err == nil {
		t.Fatal("expected write error")
	}
}

func TestRunGitDiffResolveRepoRootError(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "readme.md", []byte("hello\r\n"))

	_, err := Run(context.Background(), Options{
		Root:    root,
		GitDiff: true,
		Git: &fakeGitDiff{
			inside: true,
			root:   filepath.Join(root, "missing-repo-root"),
			files:  []string{"readme.md"},
		},
	})
	if err == nil {
		t.Fatal("expected repo root resolve error")
	}
}

func TestRunGitDiffResolveScanRootError(t *testing.T) {
	root := t.TempDir()
	badScanRoot := filepath.Join(root, "scan")
	if err := os.Symlink(filepath.Join(root, "missing"), badScanRoot); err != nil {
		t.Skip("symlinks not supported:", err)
	}
	writeTestFile(t, root, "readme.md", []byte("hello\r\n"))

	_, err := Run(context.Background(), Options{
		Root:    badScanRoot,
		GitDiff: true,
		Git: &fakeGitDiff{
			inside: true,
			root:   root,
			files:  []string{"readme.md"},
		},
	})
	if err == nil {
		t.Fatal("expected scan root resolve error")
	}
}

func TestRunSkipsUnreadableFileInWalk(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "locked.md")
	if err := os.WriteFile(path, []byte("hello\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

	report, err := Run(context.Background(), defaultOpts(root))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Skipped != 1 || report.Modified != 0 {
		t.Fatalf("report = %+v", report)
	}
}

func TestRunWalkErrorsOnInaccessibleDirectory(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, sub, "readme.md", []byte("hello\r\n"))
	if err := os.Chmod(sub, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(sub, 0o755) })

	_, err := Run(context.Background(), defaultOpts(root))
	if err == nil {
		t.Fatal("expected walk error")
	}
}
