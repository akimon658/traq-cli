package cli

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gotraq "github.com/traPtitech/go-traq"
	"go.uber.org/mock/gomock"

	"github.com/akimon658/traq-cli/internal/foundation/exit"
	"github.com/akimon658/traq-cli/internal/traq"
)

func TestFileDownload(t *testing.T) {
	t.Parallel()

	const fileID = "33333333-3333-3333-3333-333333333333"

	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().GetFileMeta(gomock.Any(), fileID).
			Return(&gotraq.FileInfo{Id: fileID, Name: "report.pdf", Size: 5}, nil)
		mc.EXPECT().GetFile(gomock.Any(), fileID).Return(writeTempDownload(t, "hello"), nil)

		dst := filepath.Join(t.TempDir(), "downloaded.txt")
		_, err := runCLIWithResolver(
			t, unitEnv(), stubResolve(mc, nil), "file", "download", fileID, "--out", dst,
		)
		require.NoError(t, err)
	})

	t.Run("missing parent", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().GetFileMeta(gomock.Any(), fileID).
			Return(&gotraq.FileInfo{Id: fileID, Name: "report.pdf", Size: 1}, nil)

		dst := filepath.Join(t.TempDir(), "missing", "out.txt")
		_, err := runCLIWithResolver(
			t, unitEnv(), stubResolve(mc, nil), "file", "download", fileID, "--out", dst,
		)
		var ec *exit.Error
		require.ErrorAs(t, err, &ec)
		assert.Equal(t, exit.General, ec.Code)
		assert.Contains(t, err.Error(), "no such directory")
	})
}
