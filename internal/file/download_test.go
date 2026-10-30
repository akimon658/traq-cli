package file

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akimon658/traq-cli/internal/exit"
)

const testFileID = "33333333-3333-3333-3333-333333333333"

func TestValidateFileID(t *testing.T) {
	t.Parallel()

	require.NoError(t, ValidateFileID(testFileID))

	err := ValidateFileID("not-a-uuid")
	var ec *exit.Error
	require.ErrorAs(t, err, &ec)
	assert.Equal(t, exit.Usage, ec.Code)
}

func TestResolveDestPath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	cases := map[string]struct {
		out     string
		name    string
		want    string
		wantErr string
	}{
		"empty-out-uses-name": {
			out: "", name: "report.pdf", want: "report.pdf",
		},
		"empty-out-empty-name-falls-back-to-id": {
			out: "", name: "", want: testFileID,
		},
		"existing-dir-joins-name": {
			out: dir, name: "report.pdf", want: filepath.Join(dir, "report.pdf"),
		},
		"trailing-separator-dir-joins-name": {
			out: dir + string(os.PathSeparator), name: "report.pdf",
			want: filepath.Join(dir, "report.pdf"),
		},
		"explicit-path": {
			out: filepath.Join(dir, "renamed.pdf"), name: "report.pdf",
			want: filepath.Join(dir, "renamed.pdf"),
		},
		"traversal-name-is-stripped": {
			out: "", name: "../../etc/passwd", want: "passwd",
		},
		"dotdot-name-falls-back-to-id": {
			out: "", name: "..", want: testFileID,
		},
		"missing-parent": {
			out: filepath.Join(dir, "missing", "a.pdf"), name: "report.pdf",
			wantErr: "no such directory",
		},
		"missing-trailing-dir": {
			out: filepath.Join(dir, "missing") + string(os.PathSeparator), name: "report.pdf",
			wantErr: "no such directory",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := resolveDestPath(tc.out, tc.name, testFileID)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestResolveDestPathParentIsFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	parent := filepath.Join(dir, "file")
	require.NoError(t, os.WriteFile(parent, []byte("x"), 0o600))

	_, err := resolveDestPath(filepath.Join(parent, "a.pdf"), "a.pdf", testFileID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a directory")
}

func TestCopyToDoesNotOverwriteWithoutForce(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dst := filepath.Join(dir, "a.txt")
	require.NoError(t, os.WriteFile(dst, []byte("old"), 0o600))

	_, err := copyTo(dst, strings.NewReader("new"), false)
	require.Error(t, err)

	got, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, "old", string(got))
}

func TestCopyToOverwritesWithForce(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dst := filepath.Join(dir, "a.txt")
	require.NoError(t, os.WriteFile(dst, []byte("old"), 0o600))

	n, err := copyTo(dst, strings.NewReader("new"), true)
	require.NoError(t, err)
	assert.Equal(t, int64(3), n)

	got, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, "new", string(got))
}

func TestCopyToWritesNewFile(t *testing.T) {
	t.Parallel()

	dst := filepath.Join(t.TempDir(), "a.txt")
	n, err := copyTo(dst, strings.NewReader("hello"), false)
	require.NoError(t, err)
	assert.Equal(t, int64(5), n)

	got, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(got))
}
