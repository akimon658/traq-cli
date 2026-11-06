package message

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

func TestService_Send(t *testing.T) {
	t.Parallel()

	msg := &gotraq.Message{Id: "m1", ChannelId: "c1", Content: "hello"}

	t.Run("ok by channel path", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().ResolveChannelID(gomock.Any(), "#general").Return("c1", nil)
		mc.EXPECT().PostMessage(gomock.Any(), "c1", "hello").Return(msg, nil)

		res, err := NewService(mc).Send(t.Context(), "#general", "hello")
		require.NoError(t, err)
		assert.Equal(t, "hello", res.Msg.Content)
	})

	t.Run("ok by channel uuid", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().ResolveChannelID(gomock.Any(), "c1").Return("c1", nil)
		mc.EXPECT().PostMessage(gomock.Any(), "c1", "hello").Return(msg, nil)

		_, err := NewService(mc).Send(t.Context(), "c1", "hello")
		require.NoError(t, err)
	})

	t.Run("channel not found", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().ResolveChannelID(gomock.Any(), "#does-not-exist").
			Return("", exit.Generalf("channel not found: %s", "#does-not-exist"))

		_, err := NewService(mc).Send(t.Context(), "#does-not-exist", "hello")
		var ec *exit.Error
		require.ErrorAs(t, err, &ec)
		assert.Equal(t, exit.General, ec.Code)
		assert.Contains(t, err.Error(), "channel not found")
	})

	t.Run("post failure propagates", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().ResolveChannelID(gomock.Any(), "#general").Return("c1", nil)
		mc.EXPECT().PostMessage(gomock.Any(), "c1", "hello").
			Return(nil, errors.New("post failed"))

		_, err := NewService(mc).Send(t.Context(), "#general", "hello")
		require.Error(t, err)
	})
}
