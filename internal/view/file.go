package view

import (
	"fmt"
	"io"

	"github.com/akimon658/traq-cli/internal/file"
)

// WriteFileSaved はダウンロード結果を1行で出力する。
func WriteFileSaved(w io.Writer, res file.DownloadResult) error {
	_, err := fmt.Fprintf(w, "saved %d bytes to %s (id:%s)\n", res.Size, res.Path, res.FileID)
	return err
}
