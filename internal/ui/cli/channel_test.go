package cli

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	gotraq "github.com/traPtitech/go-traq"
	"go.uber.org/mock/gomock"

	"github.com/akimon658/traq-cli/internal/domain/traq"
)

func TestChannelUnread(t *testing.T) {
	t.Parallel()

	t.Run("ok empty", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().GetMyUnreadChannels(gomock.Any()).Return([]gotraq.UnreadChannel{}, nil)

		_, err := runCLIWithResolver(
			t, unitEnv(), stubResolve(mc, nil), "channel", "unread",
		)
		require.NoError(t, err)
	})

	t.Run("fetch failure propagates", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().GetMyUnreadChannels(gomock.Any()).Return(nil, errors.New("unreads failed"))

		_, err := runCLIWithResolver(
			t, unitEnv(), stubResolve(mc, nil), "channel", "unread",
		)
		require.Error(t, err)
	})
}
