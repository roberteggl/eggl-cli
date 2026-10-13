package filescan

import (
	"path/filepath"
	"strings"
)

func NormalizeExtensions(raw []string) []string {
	if len(raw) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, part := range raw {
		for _, ext := range strings.Split(part, ",") {
			ext = strings.TrimSpace(ext)
			if ext == "" {
				continue
			}
			if !strings.HasPrefix(ext, ".") {
				ext = "." + ext
			}
			ext = strings.ToLower(ext)
			if _, ok := seen[ext]; ok {
				continue
			}
			seen[ext] = struct{}{}
			out = append(out, ext)
		}
	}
	return out
}

func MatchesExtension(name string, extensions []string) bool {
	if len(extensions) == 0 {
		return true
	}

	ext := strings.ToLower(filepath.Ext(name))
	for _, want := range extensions {
		if ext == want {
			return true
		}
	}
	return false
}
