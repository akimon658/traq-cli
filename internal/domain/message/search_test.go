package message

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
