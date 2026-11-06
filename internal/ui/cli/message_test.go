package cli

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gotraq "github.com/traPtitech/go-traq"
	"go.uber.org/mock/gomock"

	"github.com/akimon658/traq-cli/internal/domain/traq"
	"github.com/akimon658/traq-cli/internal/foundation/exit"
)

func TestMessageSend(t *testing.T) {
	t.Parallel()

	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().ResolveChannelID(gomock.Any(), "#general").Return("c1", nil)
		mc.EXPECT().PostMessage(gomock.Any(), "c1", "hello").Return(
			&gotraq.Message{Id: "m1", ChannelId: "c1", Content: "hello"}, nil,
		)
		stubEnrichment(mc)

		_, err := runCLIWithResolver(
			t, unitEnv(), stubResolve(mc, nil), "message", "send", "#general", "hello",
		)
		require.NoError(t, err)
	})

	t.Run("channel not found", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().ResolveChannelID(gomock.Any(), "#does-not-exist").
			Return("", exit.Generalf("channel not found: %s", "#does-not-exist"))

		_, err := runCLIWithResolver(
			t, unitEnv(), stubResolve(mc, nil), "message", "send", "#does-not-exist", "hello",
		)
		var ec *exit.Error
		require.ErrorAs(t, err, &ec)
		assert.Equal(t, exit.General, ec.Code)
	})
}

func TestMessageGet(t *testing.T) {
	t.Parallel()

	const messageID = "33333333-3333-3333-3333-333333333333"

	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().GetMessage(gomock.Any(), messageID).
			Return(&gotraq.Message{Id: messageID, ChannelId: "c1", Content: "hello"}, nil)
		mc.EXPECT().GetChannelLabel(gomock.Any(), "c1").Return("#general")
		stubEnrichment(mc)

		_, err := runCLIWithResolver(
			t, unitEnv(), stubResolve(mc, nil), "message", "get", messageID,
		)
		require.NoError(t, err)
	})

	t.Run("not found propagates", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().GetMessage(gomock.Any(), messageID).
			Return(nil, exit.Generalf("traQ API error (HTTP 404)"))

		_, err := runCLIWithResolver(
			t, unitEnv(), stubResolve(mc, nil), "message", "get", messageID,
		)
		var ec *exit.Error
		require.ErrorAs(t, err, &ec)
		assert.Equal(t, exit.General, ec.Code)
	})
}

func TestMessageList(t *testing.T) {
	t.Parallel()

	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().ResolveChannelID(gomock.Any(), "#general").Return("c1", nil)
		mc.EXPECT().GetChannelMessages(gomock.Any(), "c1", gomock.Any()).
			Return([]gotraq.Message{{Id: "m1", ChannelId: "c1", Content: "hello"}}, nil)
		stubEnrichment(mc)

		_, err := runCLIWithResolver(
			t, unitEnv(), stubResolve(mc, nil),
			"message", "list", "#general", "--limit", "10", "--show-unread=false",
		)
		require.NoError(t, err)
	})

	t.Run("unread-only with no unread", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().ResolveChannelID(gomock.Any(), "#general").Return("c1", nil)
		mc.EXPECT().GetMyUnreadChannels(gomock.Any()).Return([]gotraq.UnreadChannel{}, nil)
		mc.EXPECT().Origin().Return("https://q.trap.jp").AnyTimes()

		_, err := runCLIWithResolver(
			t, unitEnv(), stubResolve(mc, nil),
			"message", "list", "#general", "--unread-only",
		)
		require.NoError(t, err)
	})

	t.Run("channel not found", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().ResolveChannelID(gomock.Any(), "#does-not-exist").
			Return("", exit.Generalf("channel not found"))

		_, err := runCLIWithResolver(
			t, unitEnv(), stubResolve(mc, nil),
			"message", "list", "#does-not-exist", "--show-unread=false",
		)
		var ec *exit.Error
		require.ErrorAs(t, err, &ec)
		assert.Equal(t, exit.General, ec.Code)
	})
}

func TestMessageSearch(t *testing.T) {
	t.Parallel()

	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().SearchMessages(gomock.Any(), gomock.Any()).Return(
			&gotraq.MessageSearchResult{
				TotalHits: 1,
				Hits:      []gotraq.Message{{Id: "m1", Content: "hello foo"}},
			}, nil,
		)
		stubEnrichment(mc)

		_, err := runCLIWithResolver(
			t, unitEnv(), stubResolve(mc, nil), "message", "search", "foo",
		)
		require.NoError(t, err)
	})

	t.Run("in channel not found", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().ResolveChannelID(gomock.Any(), "#does-not-exist").
			Return("", exit.Generalf("channel not found"))

		_, err := runCLIWithResolver(
			t, unitEnv(), stubResolve(mc, nil),
			"message", "search", "foo", "--in", "#does-not-exist",
		)
		var ec *exit.Error
		require.ErrorAs(t, err, &ec)
		assert.Equal(t, exit.General, ec.Code)
	})

	t.Run("from user not found is usage", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().ResolveUserIDs(gomock.Any(), []string{"@no-such-user-xyz"}).
			Return(nil, errors.New("user not found"))

		_, err := runCLIWithResolver(
			t, unitEnv(), stubResolve(mc, nil),
			"message", "search", "foo", "--from", "@no-such-user-xyz",
		)
		var ec *exit.Error
		require.ErrorAs(t, err, &ec)
		assert.Equal(t, exit.Usage, ec.Code)
	})
}
