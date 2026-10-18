package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCommandAliases は複数形のエイリアスが単数形の正式名に解決されることを確認する。
func TestCommandAliases(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"users":    "user",
		"channels": "channel",
		"messages": "message",
	}
	for alias, want := range cases {
		alias, want := alias, want
		t.Run(alias, func(t *testing.T) {
			t.Parallel()
			cmd, _, err := newRootCmd(NewEnv).Find([]string{alias})
			require.NoError(t, err)
			assert.Equal(t, want, cmd.Name())
		})
	}
}
