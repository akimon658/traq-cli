package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akimon658/traq-cli/internal/exit"
)

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
			var ec *exit.Error
			require.ErrorAs(t, err, &ec)
			assert.Equal(t, exit.Usage, ec.Code)
		})
	}
}

func TestChannelUnreadRejectsArgs(t *testing.T) {
	t.Parallel()
	e := botEnv(t, "")
	_, err := runCLI(t, e, "channel", "unread", "extra")
	require.Error(t, err)
}

func TestFileDownloadInvalidArgs(t *testing.T) {
	t.Parallel()

	e := botEnv(t, "")
	cases := map[string][]string{
		"no-arg":     {"file", "download"},
		"too-many":   {"file", "download", "33333333-3333-3333-3333-333333333333", "extra"},
		"invalid-id": {"file", "download", "not-a-uuid"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := runCLI(t, e, args...)
			var ec *exit.Error
			require.ErrorAs(t, err, &ec)
			assert.Equal(t, exit.Usage, ec.Code)
		})
	}
}
