package channel

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gotraq "github.com/traPtitech/go-traq"
	"go.uber.org/mock/gomock"

	"github.com/akimon658/traq-cli/internal/traq"
)

func TestService_GetUnreadGroups(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)

	t.Run("empty unreads", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().GetMyUnreadChannels(gomock.Any()).Return([]gotraq.UnreadChannel{}, nil)

		groups, err := NewService(mc).GetUnreadGroups(t.Context())
		require.NoError(t, err)
		assert.Empty(t, groups.Others)
	})

	t.Run("classified", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().GetMyUnreadChannels(gomock.Any()).Return([]gotraq.UnreadChannel{
			{ChannelId: "c1", Count: 1, UpdatedAt: base},
		}, nil)
		mc.EXPECT().Clone().Return(mc).AnyTimes()
		mc.EXPECT().GetChannelPathMap(gomock.Any()).Return(map[string]string{"c1": "#general"}, nil)
		mc.EXPECT().GetDMUserMap(gomock.Any()).Return(map[string]string{}, nil)
		mc.EXPECT().GetSubscriptionLevels(gomock.Any()).Return(
			map[string]gotraq.ChannelSubscribeLevel{}, nil,
		)
		mc.EXPECT().GetStarSet(gomock.Any()).Return(map[string]bool{}, nil)

		groups, err := NewService(mc).GetUnreadGroups(t.Context())
		require.NoError(t, err)
		require.Len(t, groups.Others, 1)
		assert.Equal(t, "#general", groups.Others[0].Display)
	})

	t.Run("fetch failure propagates", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().GetMyUnreadChannels(gomock.Any()).
			Return(nil, errors.New("unreads failed"))

		_, err := NewService(mc).GetUnreadGroups(t.Context())
		require.Error(t, err)
	})

	t.Run("tables failure propagates", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().GetMyUnreadChannels(gomock.Any()).Return([]gotraq.UnreadChannel{
			{ChannelId: "unknown", Count: 1, UpdatedAt: base},
		}, nil)
		mc.EXPECT().Clone().Return(mc).AnyTimes()
		mc.EXPECT().GetChannelPathMap(gomock.Any()).Return(map[string]string{}, nil)
		mc.EXPECT().GetDMUserMap(gomock.Any()).Return(map[string]string{}, nil)
		mc.EXPECT().GetSubscriptionLevels(gomock.Any()).Return(
			map[string]gotraq.ChannelSubscribeLevel{}, nil,
		)
		mc.EXPECT().GetStarSet(gomock.Any()).Return(map[string]bool{}, nil)

		_, err := NewService(mc).GetUnreadGroups(t.Context())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "channel not found")
	})
}
