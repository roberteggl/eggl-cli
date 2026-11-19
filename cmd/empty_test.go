package cmd

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestEmptyHelp(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	output := runHelp(t, "empty", "--help")

	for _, want := range []string{
		"eggl empty",
		"--message",
		"-m",
		"--push",
		"-p",
		"Examples:",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in empty help, got %q", want, output)
		}
	}

	if strings.Contains(output, "Available Commands:") {
		t.Fatalf("empty help should not list subcommands, got %q", output)
	}
}

func TestEmptyCreatesCommit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	t.Setenv("NO_COLOR", "1")

	dir := t.TempDir()
	initGitRepo(t, dir)

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

	stdout, _, err := runCmd(t, "empty")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(stdout, "empty commit ") {
		t.Fatalf("expected empty commit output, got %q", stdout)
	}
	if strings.Contains(stdout, "pushed") {
		t.Fatalf("expected no push output, got %q", stdout)
	}

	msg := gitLogMessage(t, dir)
	if msg != "chore: empty commit" {
		t.Fatalf("commit message = %q, want default", msg)
	}
}

func TestEmptyCustomMessage(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	t.Setenv("NO_COLOR", "1")

	dir := t.TempDir()
	initGitRepo(t, dir)

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

	stdout, _, err := runCmd(t, "empty", "-m", "ci: retrigger")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(stdout, "empty commit ") {
		t.Fatalf("expected empty commit output, got %q", stdout)
	}

	msg := gitLogMessage(t, dir)
	if msg != "ci: retrigger" {
		t.Fatalf("commit message = %q, want %q", msg, "ci: retrigger")
	}
}

func TestEmptyPushWithoutRemote(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	t.Setenv("NO_COLOR", "1")

	dir := t.TempDir()
	initGitRepo(t, dir)

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

	_, _, err = runCmd(t, "empty", "-p")
	if err == nil {
		t.Fatal("expected error when pushing without remote")
	}
}

func initGitRepo(t *testing.T, dir string) {
	t.Helper()

	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
}

func gitLogMessage(t *testing.T, dir string) string {
	t.Helper()

	cmd := exec.Command("git", "log", "-1", "--pretty=%s")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git log: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out))
}
