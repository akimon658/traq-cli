package cli

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akimon658/traq-cli/internal/foundation/client"
	"github.com/akimon658/traq-cli/internal/foundation/exit"
)

func TestFileDownload(t *testing.T) {
	t.Parallel()
	requireTraqStack(t)

	content := []byte("hello traq-cli file download\n")
	fileID := uploadTestFile(t, "traq-cli-download.txt", content)

	dst := filepath.Join(t.TempDir(), "downloaded.txt")
	out, err := runCLI(t, botEnv(t, testTraqToken), "file", "download", fileID, "--out", dst)
	require.NoError(t, err, "file download failed")
	assert.Contains(t, out, "saved "+strconv.Itoa(len(content))+" bytes to "+dst)
	assert.Contains(t, out, "id:"+fileID)

	got, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, content, got)
}

func TestFileDownloadDoesNotOverwriteWithoutForce(t *testing.T) {
	t.Parallel()
	requireTraqStack(t)

	content := []byte("first")
	fileID := uploadTestFile(t, "traq-cli-overwrite.txt", content)

	dst := filepath.Join(t.TempDir(), "out.txt")
	e := botEnv(t, testTraqToken)
	_, err := runCLI(t, e, "file", "download", fileID, "--out", dst)
	require.NoError(t, err)

	_, err = runCLI(t, e, "file", "download", fileID, "--out", dst)
	require.Error(t, err)
	var ec *exit.Error
	require.ErrorAs(t, err, &ec)
	assert.Equal(t, exit.General, ec.Code)

	got, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, content, got)

	_, err = runCLI(t, e, "file", "download", fileID, "--out", dst, "--force")
	require.NoError(t, err)
}

func TestFileDownloadMissingParent(t *testing.T) {
	t.Parallel()
	requireTraqStack(t)

	fileID := uploadTestFile(t, "traq-cli-missing-parent.txt", []byte("x"))
	dst := filepath.Join(t.TempDir(), "missing", "out.txt")

	_, err := runCLI(t, botEnv(t, testTraqToken), "file", "download", fileID, "--out", dst)
	require.Error(t, err)
	var ec *exit.Error
	require.ErrorAs(t, err, &ec)
	assert.Equal(t, exit.General, ec.Code)
	assert.Contains(t, err.Error(), "no such directory")

	_, statErr := os.Stat(filepath.Dir(dst))
	assert.ErrorIs(t, statErr, os.ErrNotExist, "the parent directory should not be created")
}

func uploadTestFile(t *testing.T, name string, content []byte) string {
	t.Helper()

	src := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(src, content, 0o600))
	f, err := os.Open(src)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	info, _, err := botAPIClient(t).FileAPI.
		PostFile(client.TokenContext(t.Context(), testTraqToken)).
		ChannelId(testGeneralChannelID(t)).
		File(f).
		Execute()
	require.NoError(t, err, "failed to upload a test file")
	return info.Id
}
