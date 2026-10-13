package filescan

import (
	"path/filepath"
	"strings"
	"unicode/utf8"
)

var skippedDirs = map[string]struct{}{
	// VCS
	".git": {},
	".hg":  {},
	".svn": {},

	// Node / frontend
	"node_modules":     {},
	"bower_components": {},
	".next":            {},
	".nuxt":            {},
	".turbo":           {},
	".parcel-cache":    {},
	".yarn":            {},
	".pnpm":            {},
	".angular":         {},
	".sass-cache":      {},

	// Go / general
	"vendor": {},
	"bin":    {},
	"dist":   {},
	"build":  {},

	// Java / JVM
	"target":  {},
	".gradle": {},
	"out":     {},
	".idea":   {},

	// Python
	"__pycache__":   {},
	".pytest_cache": {},
	".mypy_cache":   {},
	".ruff_cache":   {},
	".venv":         {},
	"venv":          {},
	".tox":          {},
	".eggs":         {},
	"htmlcov":       {},

	// Ruby
	".bundle": {},

	// Mobile
	"Pods":        {},
	"DerivedData": {},
	"Carthage":    {},

	// Infra / misc
	".terraform": {},
	".dart_tool": {},
	".cache":     {},
	"coverage":   {},
	"tmp":        {},
	".tmp":       {},
	"__MACOSX":   {},
}

var skippedFilenames = map[string]struct{}{
	".ds_store":   {},
	"thumbs.db":   {},
	"desktop.ini": {},
}

var skippedSuffixes = []string{
	".min.js",
	".min.css",
}

var skippedExtensions = map[string]struct{}{
	// Images
	".png":  {},
	".jpg":  {},
	".jpeg": {},
	".gif":  {},
	".webp": {},
	".ico":  {},
	".bmp":  {},
	".heic": {},
	".heif": {},
	".tiff": {},
	".tif":  {},
	".svgz": {},
	".avif": {},

	// Fonts
	".woff":  {},
	".woff2": {},
	".ttf":   {},
	".otf":   {},
	".eot":   {},

	// Archives / packages
	".zip": {},
	".tar": {},
	".gz":  {},
	".bz2": {},
	".xz":  {},
	".7z":  {},
	".rar": {},
	".jar": {},
	".war": {},
	".ear": {},
	".nar": {},
	".whl": {},
	".gem": {},
	".dmg": {},
	".deb": {},
	".rpm": {},
	".pkg": {},
	".msi": {},
	".apk": {},
	".aab": {},
	".ipa": {},
	".iso": {},

	// Media
	".mp3":  {},
	".mp4":  {},
	".mov":  {},
	".avi":  {},
	".mkv":  {},
	".wav":  {},
	".flac": {},
	".ogg":  {},
	".webm": {},
	".m4a":  {},
	".wma":  {},

	// Binaries / objects
	".exe":   {},
	".dll":   {},
	".so":    {},
	".dylib": {},
	".a":     {},
	".o":     {},
	".obj":   {},
	".lib":   {},
	".pdb":   {},
	".node":  {},
	".bin":   {},
	".wasm":  {},
	".class": {},
	".pyc":   {},
	".lockb": {},

	// Documents / data
	".pdf":    {},
	".sqlite": {},
	".db":     {},
	".mdb":    {},
	".dat":    {},
	".map":    {},
}

func IsHiddenName(name string) bool {
	return len(name) > 0 && name[0] == '.'
}

func ShouldSkipDir(name string) bool {
	_, ok := skippedDirs[name]
	return ok
}

func ShouldSkipFile(name string) bool {
	lower := strings.ToLower(name)
	if _, ok := skippedFilenames[lower]; ok {
		return true
	}

	for _, suffix := range skippedSuffixes {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}

	ext := strings.ToLower(filepath.Ext(name))
	_, ok := skippedExtensions[ext]
	return ok
}

func IsBinaryContent(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	if strings.Contains(string(data), "\x00") {
		return true
	}
	return !utf8.Valid(data)
}
