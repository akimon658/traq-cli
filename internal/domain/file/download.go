package file

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"uuid"

	"github.com/traPtitech/go-traq"

	"github.com/akimon658/traq-cli/internal/auth"
	"github.com/akimon658/traq-cli/internal/foundation/client"
	"github.com/akimon658/traq-cli/internal/foundation/config"
	"github.com/akimon658/traq-cli/internal/foundation/exit"
)

// DownloadResult は保存したファイルの情報。
type DownloadResult struct {
	FileID string
	Path   string
	Size   int64
	Meta   traq.FileInfo
}

const downloadedFilePerm = 0o666

// ValidateFileID はファイル ID が UUID であることを検証する。
func ValidateFileID(id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return exit.Usagef("invalid file ID: %v", err)
	}
	return nil
}

// Download は指定したファイルを取得し、out へ保存する。
// out が空ならカレントディレクトリに元のファイル名で保存する。
// out が既存のディレクトリならその中に元のファイル名で保存する。
// 親ディレクトリは自動作成しない。force が false のとき既存ファイルは上書きしない。
func Download(
	ctx context.Context,
	e *config.Env,
	rt *auth.ResolvedToken,
	c *client.Client,
	fileID, out string,
	force bool,
) (DownloadResult, error) {
	if err := ValidateFileID(fileID); err != nil {
		return DownloadResult{}, err
	}
	if err := auth.RequireScope(rt, auth.ScopeRead); err != nil {
		return DownloadResult{}, err
	}

	var meta *traq.FileInfo
	if err := auth.Exec(
		ctx,
		e.Origin,
		e.ClientID,
		e.LoadStore(),
		rt,
		func(ctx context.Context) (*http.Response, error) {
			var resp *http.Response
			var apiErr error
			meta, resp, apiErr = c.GetFileMeta(ctx, fileID)
			return resp, apiErr
		},
	); err != nil {
		return DownloadResult{}, err
	}
	if meta == nil {
		return DownloadResult{}, exit.Generalf("failed to get the metadata of file %s", fileID)
	}

	path, err := resolveDestPath(out, meta.Name, fileID)
	if err != nil {
		return DownloadResult{}, err
	}

	var tmp *os.File
	if err := auth.Exec(
		ctx,
		e.Origin,
		e.ClientID,
		e.LoadStore(),
		rt,
		func(ctx context.Context) (*http.Response, error) {
			var resp *http.Response
			var apiErr error
			tmp, resp, apiErr = c.GetFile(ctx, fileID)
			return resp, apiErr
		},
	); err != nil {
		return DownloadResult{}, err
	}
	if tmp == nil {
		return DownloadResult{}, exit.Generalf("failed to download file %s", fileID)
	}
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}()

	n, err := copyTo(path, tmp, force)
	if err != nil {
		return DownloadResult{}, exit.GeneralError(err)
	}
	return DownloadResult{
		FileID: fileID,
		Path:   path,
		Size:   n,
		Meta:   *meta,
	}, nil
}

// resolveDestPath は保存先パスを決める。親ディレクトリが無い場合はエラーにする。
func resolveDestPath(out, name, fileID string) (string, error) {
	safe := sanitizeName(name, fileID)
	if out == "" {
		return safe, nil
	}
	if isDir(out) {
		return filepath.Join(out, safe), nil
	}
	if strings.HasSuffix(out, string(os.PathSeparator)) {
		if err := requireDir(out); err != nil {
			return "", err
		}
		return filepath.Join(out, safe), nil
	}
	if err := requireDir(filepath.Dir(out)); err != nil {
		return "", err
	}
	return out, nil
}

// sanitizeName は保存に使えるファイル名だけを取り出す。使えない場合は fileID に置き換える。
func sanitizeName(name, fileID string) string {
	base := filepath.Base(filepath.Clean(name))
	switch base {
	case "", ".", "..", string(filepath.Separator):
		return fileID
	}
	return base
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// requireDir は path が既存のディレクトリであることを確認する。
func requireDir(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return exit.Generalf("no such directory: %s", path)
		}
		return exit.GeneralError(err)
	}
	if !info.IsDir() {
		return exit.Generalf("not a directory: %s", path)
	}
	return nil
}

// copyTo は r の内容を dst へ書き出す。force が false のとき既存ファイルがあれば失敗する。
func copyTo(dst string, r io.Reader, force bool) (int64, error) {
	flag := os.O_WRONLY | os.O_CREATE
	if force {
		flag |= os.O_TRUNC
	} else {
		flag |= os.O_EXCL
	}
	f, err := os.OpenFile(dst, flag, downloadedFilePerm)
	if err != nil {
		return 0, err
	}
	n, copyErr := io.Copy(f, r)
	closeErr := f.Close()
	if copyErr != nil {
		return n, copyErr
	}
	if closeErr != nil {
		return n, fmt.Errorf("%s: %w", dst, closeErr)
	}
	return n, nil
}
