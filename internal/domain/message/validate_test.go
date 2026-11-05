package message

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akimon658/traq-cli/internal/foundation/exit"
)

func TestValidateMessageID(t *testing.T) {
	t.Parallel()

	require.NoError(t, ValidateMessageID("33333333-3333-3333-3333-333333333333"))
	err := ValidateMessageID("not-a-uuid")
	require.Error(t, err)
	var ec *exit.Error
	require.ErrorAs(t, err, &ec)
	assert.Equal(t, exit.Usage, ec.Code)
}
