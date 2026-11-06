package message

import (
	"errors"
	"testing"
	"time"

	"github.com/moznion/go-optional"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gotraq "github.com/traPtitech/go-traq"
	"go.uber.org/mock/gomock"

	"github.com/akimon658/traq-cli/internal/domain/traq"
	"github.com/akimon658/traq-cli/internal/foundation/exit"
)

func validListOptions() ListOptions {
	return ListOptions{Limit: 10, Offset: 0, Order: "desc", ShowUnread: false}
}

func TestService_List(t *testing.T) {
	t.Parallel()

	msgs := []gotraq.Message{{Id: "m1", ChannelId: "c1", Content: "hello"}}

	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().ResolveChannelID(gomock.Any(), "#general").Return("c1", nil)
		mc.EXPECT().GetChannelMessages(gomock.Any(), "c1", gomock.Any()).Return(msgs, nil)

		res, err := NewService(mc).List(t.Context(), "#general", validListOptions())
		require.NoError(t, err)
		require.Len(t, res.Msgs, 1)
		assert.Equal(t, "#general", res.Label)
	})

	t.Run("invalid flags", func(t *testing.T) {
		t.Parallel()
		cases := map[string]ListOptions{
			"limit":  {Limit: 0, Order: "desc"},
			"offset": {Limit: 10, Offset: -1, Order: "desc"},
			"order":  {Limit: 10, Order: "up"},
			"since":  {Limit: 10, Order: "desc", Since: "not-a-time"},
		}
		for name, opts := range cases {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				mc := traq.NewMockClient(gomock.NewController(t))
				_, err := NewService(mc).List(t.Context(), "#general", opts)
				var ec *exit.Error
				require.ErrorAs(t, err, &ec)
				assert.Equal(t, exit.Usage, ec.Code)
			})
		}
	})

	t.Run("channel not found", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().ResolveChannelID(gomock.Any(), "#does-not-exist").
			Return("", exit.Generalf("channel not found"))

		_, err := NewService(mc).List(t.Context(), "#does-not-exist", validListOptions())
		var ec *exit.Error
		require.ErrorAs(t, err, &ec)
		assert.Equal(t, exit.General, ec.Code)
	})

	t.Run("unread-only with no unread skips fetch", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().ResolveChannelID(gomock.Any(), "#general").Return("c1", nil)
		mc.EXPECT().GetMyUnreadChannels(gomock.Any()).Return([]gotraq.UnreadChannel{}, nil)

		res, err := NewService(mc).
			List(t.Context(), "#general", ListOptions{Limit: 10, Order: "desc", UnreadOnly: true})
		require.NoError(t, err)
		assert.True(t, res.UnreadOnly)
		assert.Empty(t, res.Msgs)
	})

	t.Run("unread-only intersects since", func(t *testing.T) {
		t.Parallel()
		base := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
		unread := gotraq.UnreadChannel{ChannelId: "c1", Count: 1, Since: base}
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().ResolveChannelID(gomock.Any(), "c1").Return("c1", nil)
		mc.EXPECT().GetMyUnreadChannels(gomock.Any()).
			Return([]gotraq.UnreadChannel{unread}, nil)
		// --since が未読 since より後なら新しい方が採用される。取得は1回だけ行われる
		mc.EXPECT().GetChannelMessages(gomock.Any(), "c1", gomock.Any()).Return(msgs, nil)

		opts := ListOptions{
			Limit: 10, Order: "desc", UnreadOnly: true,
			Since: base.Add(time.Second).Format(time.RFC3339),
		}
		res, err := NewService(mc).List(t.Context(), "c1", opts)
		require.NoError(t, err)
		require.Len(t, res.Msgs, 1)
	})

	t.Run("fetch failure propagates", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().ResolveChannelID(gomock.Any(), "#general").Return("c1", nil)
		mc.EXPECT().GetChannelMessages(gomock.Any(), "c1", gomock.Any()).
			Return(nil, errors.New("fetch failed"))

		_, err := NewService(mc).List(t.Context(), "#general", validListOptions())
		require.Error(t, err)
	})

	t.Run("show-unread failure is ignored", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().ResolveChannelID(gomock.Any(), "#general").Return("c1", nil)
		mc.EXPECT().GetChannelMessages(gomock.Any(), "c1", gomock.Any()).Return(msgs, nil)
		mc.EXPECT().GetMyUnreadChannels(gomock.Any()).
			Return(nil, errors.New("unread failed")).AnyTimes()

		opts := validListOptions()
		opts.ShowUnread = true
		res, err := NewService(mc).List(t.Context(), "#general", opts)
		require.NoError(t, err)
		assert.True(t, res.Unread.IsNone(), "unread should be empty, got %v", res.Unread)
		_ = optional.None[gotraq.UnreadChannel]()
	})
}
