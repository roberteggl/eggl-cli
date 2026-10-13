package filescan

import "testing"

func TestNormalizeExtensions(t *testing.T) {
	got := NormalizeExtensions([]string{" md ", "txt,.go", ".MD", "txt", "", "  "})
	want := []string{".md", ".txt", ".go"}
	if len(got) != len(want) {
		t.Fatalf("NormalizeExtensions() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("NormalizeExtensions()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestMatchesExtension(t *testing.T) {
	extensions := NormalizeExtensions([]string{"md", "txt"})

	if !MatchesExtension("readme.md", extensions) {
		t.Fatal("expected .md to match")
	}
	if !MatchesExtension("README.MD", extensions) {
		t.Fatal("expected .MD to match case-insensitively")
	}
	if MatchesExtension("image.png", extensions) {
		t.Fatal("expected .png not to match")
	}
	if !MatchesExtension("notes.txt", nil) {
		t.Fatal("expected all extensions to match when filter is empty")
	}
}
