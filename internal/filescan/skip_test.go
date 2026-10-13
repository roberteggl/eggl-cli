package filescan

import "testing"

func TestShouldSkipFile(t *testing.T) {
	if !ShouldSkipFile("photo.JPG") {
		t.Fatal("expected .JPG to be skipped")
	}
	if ShouldSkipFile("readme.md") {
		t.Fatal("expected readme.md not to be skipped")
	}
	if !ShouldSkipFile("app.jar") {
		t.Fatal("expected .jar to be skipped")
	}
	if !ShouldSkipFile("bundle.min.js") {
		t.Fatal("expected .min.js to be skipped")
	}
	if !ShouldSkipFile(".DS_Store") {
		t.Fatal("expected .DS_Store to be skipped")
	}
	if !ShouldSkipFile("archive.zip") {
		t.Fatal("expected .zip to be skipped")
	}
}

func TestShouldSkipDir(t *testing.T) {
	if !ShouldSkipDir("node_modules") {
		t.Fatal("expected node_modules to be skipped")
	}
}

func TestIsBinaryContent(t *testing.T) {
	if !IsBinaryContent([]byte("hello\x00world")) {
		t.Fatal("expected null byte to be binary")
	}
	if IsBinaryContent([]byte("hello \u2014 world")) {
		t.Fatal("expected text not to be binary")
	}
	if IsBinaryContent([]byte("hello\r\nworld")) {
		t.Fatal("expected CRLF text not to be binary")
	}
	if !IsBinaryContent([]byte{0xff, 0xfe, 0xfd}) {
		t.Fatal("expected invalid UTF-8 to be binary")
	}
	if IsBinaryContent(nil) {
		t.Fatal("expected empty content not to be binary")
	}
}
