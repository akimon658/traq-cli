package file

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gotraq "github.com/traPtitech/go-traq"
	"go.uber.org/mock/gomock"

	"github.com/akimon658/traq-cli/internal/foundation/exit"
	"github.com/akimon658/traq-cli/internal/traq"
)

func TestService_Download(t *testing.T) {
	t.Parallel()

	const fileID = "33333333-3333-3333-3333-333333333333"
	content := "hello traq-cli file download\n"
	meta := &gotraq.FileInfo{Id: fileID, Name: "report.pdf", Size: int64(len(content))}

	writeTempDownload := func(t *testing.T, content string) *os.File {
		t.Helper()
		f, err := os.CreateTemp(t.TempDir(), "traq-cli-download-*")
		require.NoError(t, err)
		_, err = f.WriteString(content)
		require.NoError(t, err)
		_, err = f.Seek(0, 0)
		require.NoError(t, err)
		return f
	}

	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().GetFileMeta(gomock.Any(), fileID).Return(meta, nil)
		mc.EXPECT().GetFile(gomock.Any(), fileID).Return(writeTempDownload(t, content), nil)

		dst := filepath.Join(t.TempDir(), "downloaded.txt")
		res, err := NewService(mc).Download(t.Context(), DownloadOptions{FileID: fileID, Out: dst})
		require.NoError(t, err)
		assert.Equal(t, dst, res.Path)
		assert.Equal(t, int64(len(content)), res.Size)

		got, err := os.ReadFile(dst)
		require.NoError(t, err)
		assert.Equal(t, content, string(got))
	})

	t.Run("invalid id", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))

		_, err := NewService(mc).Download(t.Context(), DownloadOptions{FileID: "not-a-uuid"})
		var ec *exit.Error
		require.ErrorAs(t, err, &ec)
		assert.Equal(t, exit.Usage, ec.Code)
	})

	t.Run("does not overwrite without force", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().GetFileMeta(gomock.Any(), fileID).Return(meta, nil)
		mc.EXPECT().GetFile(gomock.Any(), fileID).Return(writeTempDownload(t, content), nil)

		dst := filepath.Join(t.TempDir(), "out.txt")
		require.NoError(t, os.WriteFile(dst, []byte("old"), 0o600))

		_, err := NewService(mc).Download(t.Context(), DownloadOptions{FileID: fileID, Out: dst})
		require.Error(t, err)

		got, err := os.ReadFile(dst)
		require.NoError(t, err)
		assert.Equal(t, "old", string(got))
	})

	t.Run("overwrites with force", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().GetFileMeta(gomock.Any(), fileID).Return(meta, nil)
		mc.EXPECT().GetFile(gomock.Any(), fileID).Return(writeTempDownload(t, content), nil)

		dst := filepath.Join(t.TempDir(), "out.txt")
		require.NoError(t, os.WriteFile(dst, []byte("old"), 0o600))

		_, err := NewService(mc).
			Download(t.Context(), DownloadOptions{FileID: fileID, Out: dst, Force: true})
		require.NoError(t, err)
	})

	t.Run("missing parent", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().GetFileMeta(gomock.Any(), fileID).Return(meta, nil)

		dst := filepath.Join(t.TempDir(), "missing", "out.txt")
		_, err := NewService(mc).Download(t.Context(), DownloadOptions{FileID: fileID, Out: dst})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no such directory")
	})

	t.Run("meta failure propagates", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().GetFileMeta(gomock.Any(), fileID).Return(nil, errors.New("meta failed"))

		dst := filepath.Join(t.TempDir(), "out.txt")
		_, err := NewService(mc).Download(t.Context(), DownloadOptions{FileID: fileID, Out: dst})
		require.Error(t, err)
	})
}
