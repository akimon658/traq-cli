package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCallback(t *testing.T) {
	t.Parallel()

	t.Run("full URL", func(t *testing.T) {
		t.Parallel()
		code, state, denied, err := ParseCallback(
			"http://127.0.0.1/callback?code=c&state=s&scope=read",
		)
		require.NoError(t, err)
		assert.Nil(t, denied)
		assert.Equal(t, "c", code)
		assert.Equal(t, "s", state)
	})

	t.Run("query string", func(t *testing.T) {
		t.Parallel()
		code, state, denied, err := ParseCallback("code=c&state=s")
		require.NoError(t, err)
		assert.Nil(t, denied)
		assert.Equal(t, "c", code)
		assert.Equal(t, "s", state)
	})

	t.Run("leading question mark", func(t *testing.T) {
		t.Parallel()
		code, state, denied, err := ParseCallback("?code=c")
		require.NoError(t, err)
		assert.Nil(t, denied)
		assert.Equal(t, "c", code)
		assert.Empty(t, state)
	})

	t.Run("denied", func(t *testing.T) {
		t.Parallel()
		_, _, denied, err := ParseCallback(
			"http://127.0.0.1/callback?error=access_denied&state=s",
		)
		require.NoError(t, err)
		require.NotNil(t, denied)
		assert.Contains(t, denied.Error(), "authorization was denied (access_denied)")

		_, _, denied, _ = ParseCallback(
			"http://127.0.0.1/callback?error=access_denied&error_description=no",
		)
		require.NotNil(t, denied)
		assert.Contains(t, denied.Error(), "access_denied: no")
	})
}
