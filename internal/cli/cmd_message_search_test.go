package cli

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/traPtitech/go-traq"
)

func TestSearchLabel(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "search", searchLabel("", ""))
	assert.Equal(t, `search "foo"`, searchLabel("foo", ""))
	assert.Equal(t, "search in #general", searchLabel("", "#general"))
	assert.Equal(t, "search in #general", searchLabel("", "general"))
	assert.Equal(t, `search "foo" in #general/a`, searchLabel("foo", "#general/a"))
}

func TestCleanUserInputs(t *testing.T) {
	t.Parallel()
	assert.Empty(t, cleanUserInputs(nil))
	assert.Equal(t, []string{"a", "b"}, cleanUserInputs([]string{"a", " ", "", "b"}))
}

func TestResolveUserIDs(t *testing.T) {
	t.Parallel()
	nameToID := map[string]string{"alice": "u1", "BOT_x": "u2"}

	ids, err := resolveUserIDs([]string{"u9-uuid-will-fail-name-check"}, nameToID)
	_ = ids
	// "u9-uuid-will-fail-name-check" は UUID ではないので名前解決されエラーになる
	require.Error(t, err)

	ids, err = resolveUserIDs([]string{"00000000-0000-0000-0000-000000000000"}, nameToID)
	require.NoError(t, err)
	assert.Equal(t, []string{"00000000-0000-0000-0000-000000000000"}, ids)

	ids, err = resolveUserIDs([]string{"alice", "@BOT_x"}, nameToID)
	require.NoError(t, err)
	assert.Equal(t, []string{"u1", "u2"}, ids)

	_, err = resolveUserIDs([]string{"@unknown"}, nameToID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "user not found")

	_, err = resolveUserIDs([]string{"@"}, nameToID)
	require.Error(t, err)
}

func TestWriteMessageListSearchHeader(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	total := int64(42)
	var buf bytes.Buffer
	require.NoError(t, writeMessageList(&buf, messageList{
		label: "search \"foo\"", limit: 10, offset: 0, sort: "-createdAt",
		msgs: []traq.Message{
			{Id: "m1", UserId: "u1", Content: "hello foo", CreatedAt: base, UpdatedAt: base},
		},
		users:     map[string]traq.User{"u1": {Id: "u1", Name: "alice", DisplayName: "alice"}},
		stamps:    map[string]string{},
		loc:       time.UTC,
		totalHits: &total,
	}))
	out := buf.String()
	assert.Contains(t, out, "search \"foo\" (全42件中 1件 limit=10 offset=0 sort=-createdAt)")
	assert.Contains(t, out, "hello foo")
}

func TestMessageSearchInvalidFlags(t *testing.T) {
	t.Parallel()

	e := botEnv(t, "")
	cases := map[string][]string{
		"limit-low":    {"message", "search", "foo", "--limit", "0"},
		"limit-high":   {"message", "search", "foo", "--limit", "101"},
		"offset-neg":   {"message", "search", "foo", "--offset=-1"},
		"offset-high":  {"message", "search", "foo", "--offset", "9901"},
		"sort":         {"message", "search", "foo", "--sort", "asc"},
		"after":        {"message", "search", "foo", "--after", "not-a-time"},
		"before":       {"message", "search", "foo", "--before", "not-a-time"},
		"citation":     {"message", "search", "foo", "--citation", "not-a-uuid"},
		"too-many-arg": {"message", "search", "a", "b"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := runCLI(t, e, args...)
			var ec *ExitCodeError
			require.ErrorAs(t, err, &ec)
			assert.Equal(t, exitUsage, ec.Code)
		})
	}
}
