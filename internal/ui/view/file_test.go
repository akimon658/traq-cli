package view

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akimon658/traq-cli/internal/domain/file"
)

func TestWriteFileSaved(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := WriteFileSaved(&buf, file.DownloadResult{
		FileID: testFileID1,
		Path:   "./report.pdf",
		Size:   12345,
	})
	require.NoError(t, err)
	assert.Equal(
		t,
		"saved 12345 bytes to ./report.pdf (id:"+testFileID1+")\n",
		buf.String(),
	)
}
