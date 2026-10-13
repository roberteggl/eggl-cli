package dedash

import "testing"

func TestRunFiltersByExtension(t *testing.T) {
	root := t.TempDir()
	for name, content := range map[string]string{
		"readme.md": "hello \u2014 world",
		"notes.txt": "text \u2014 file",
		"data.json": "json \u2014 file",
	} {
		if err := writeTestFile(root, name, content); err != nil {
			t.Fatal(err)
		}
	}

	report, err := Run(t.Context(), Options{Root: root, Extensions: []string{"md", "txt"}, Yes: true})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Scanned != 2 {
		t.Fatalf("Scanned = %d, want 2", report.Scanned)
	}
	if report.Modified != 2 {
		t.Fatalf("Modified = %d, want 2", report.Modified)
	}
	if report.Skipped != 1 {
		t.Fatalf("Skipped = %d, want 1", report.Skipped)
	}

	got, err := readTestFile(root, "data.json")
	if err != nil {
		t.Fatal(err)
	}
	if got != "json \u2014 file" {
		t.Fatalf("json file = %q, want unchanged", got)
	}
}
