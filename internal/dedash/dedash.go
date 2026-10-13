package dedash

import (
	"context"
	"fmt"
	"strings"

	"github.com/roberteggl/eggl-cli/internal/filescan"
)

const emDash = "\u2014"

// MaxFileSize is the largest regular file dedash will read (50 MiB).
const MaxFileSize = filescan.MaxFileSize

type GitDiffRunner = filescan.GitDiffRunner
type Options = filescan.CommonOptions
type FileChange = filescan.FileChange
type Report = filescan.Report

func Run(ctx context.Context, opts Options) (*Report, error) {
	return filescan.Run(ctx, filescan.Options{
		CommonOptions: opts,
		Transform:     transformEmDash,
		ConfirmPrompt: func(total, files int) string {
			return fmt.Sprintf("This will replace %d em-dashes in %d files. Continue? [y/N]: ", total, files)
		},
		FoundLogMessage: "found em-dashes",
	})
}

func transformEmDash(data []byte) (int, []byte) {
	content := string(data)
	if !strings.Contains(content, emDash) {
		return 0, nil
	}
	count := strings.Count(content, emDash)
	return count, []byte(strings.ReplaceAll(content, emDash, "-"))
}

func TotalReplacements(report *Report) int {
	return filescan.TotalReplacements(report)
}
