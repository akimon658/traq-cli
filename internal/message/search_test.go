package message

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchLabel(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "search", SearchLabel("", ""))
	assert.Equal(t, `search "foo"`, SearchLabel("foo", ""))
	assert.Equal(t, "search in #general", SearchLabel("", "#general"))
	assert.Equal(t, "search in #general", SearchLabel("", "general"))
	assert.Equal(t, `search "foo" in #general/a`, SearchLabel("foo", "#general/a"))
}

func TestCleanUserInputs(t *testing.T) {
	t.Parallel()
	assert.Empty(t, CleanUserInputs(nil))
	assert.Equal(t, []string{"a", "b"}, CleanUserInputs([]string{"a", " ", "", "b"}))
}

func TestResolveUserIDs(t *testing.T) {
	t.Parallel()
	nameToID := map[string]string{"alice": "u1", "BOT_x": "u2"}

	ids, err := ResolveUserIDs([]string{"u9-uuid-will-fail-name-check"}, nameToID)
	_ = ids
	// "u9-uuid-will-fail-name-check" は UUID ではないので名前解決されエラーになる
	require.Error(t, err)

	ids, err = ResolveUserIDs([]string{"00000000-0000-0000-0000-000000000000"}, nameToID)
	require.NoError(t, err)
	assert.Equal(t, []string{"00000000-0000-0000-0000-000000000000"}, ids)

	ids, err = ResolveUserIDs([]string{"alice", "@BOT_x"}, nameToID)
	require.NoError(t, err)
	assert.Equal(t, []string{"u1", "u2"}, ids)

	_, err = ResolveUserIDs([]string{"@unknown"}, nameToID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "user not found")

	_, err = ResolveUserIDs([]string{"@"}, nameToID)
	require.Error(t, err)
}
