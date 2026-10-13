package eol

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/roberteggl/eggl-cli/internal/filescan"
)

// MaxFileSize is the largest regular file eol will read (50 MiB).
const MaxFileSize = filescan.MaxFileSize

type GitDiffRunner = filescan.GitDiffRunner
type Options = filescan.CommonOptions
type FileChange = filescan.FileChange
type Report = filescan.Report

func Run(ctx context.Context, opts Options) (*Report, error) {
	return filescan.Run(ctx, filescan.Options{
		CommonOptions: opts,
		Transform: func(data []byte) (int, []byte) {
			content, count := normalizeEOL(data)
			return count, content
		},
		ConfirmPrompt: func(total, files int) string {
			return fmt.Sprintf("This will fix %d line endings in %d files. Continue? [y/N]: ", total, files)
		},
		FoundLogMessage: "found line endings to fix",
	})
}

func normalizeEOL(data []byte) ([]byte, int) {
	if !bytes.Contains(data, []byte{'\r'}) {
		return nil, 0
	}

	s := string(data)
	crlf := strings.Count(s, "\r\n")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	cr := strings.Count(s, "\r")
	if cr > 0 {
		s = strings.ReplaceAll(s, "\r", "\n")
	}

	total := crlf + cr
	if total == 0 {
		return nil, 0
	}
	return []byte(s), total
}

func TotalReplacements(report *Report) int {
	return filescan.TotalReplacements(report)
}
