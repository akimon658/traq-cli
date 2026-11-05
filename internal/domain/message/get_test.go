package message

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gotraq "github.com/traPtitech/go-traq"
	"go.uber.org/mock/gomock"

	"github.com/akimon658/traq-cli/internal/foundation/exit"
	"github.com/akimon658/traq-cli/internal/traq"
)

func TestService_Get(t *testing.T) {
	t.Parallel()

	const messageID = "33333333-3333-3333-3333-333333333333"
	msg := &gotraq.Message{Id: messageID, ChannelId: "c1", Content: "hello"}

	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().GetMessage(gomock.Any(), messageID).Return(msg, nil)
		mc.EXPECT().GetChannelLabel(gomock.Any(), "c1").Return("#general")
		stubEnrichment(mc)

		res, err := NewService(mc).Get(t.Context(), messageID)
		require.NoError(t, err)
		assert.Equal(t, "hello", res.Msg.Content)
		assert.Equal(t, "#general", res.ChannelLabel)
	})

	t.Run("invalid id", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))

		_, err := NewService(mc).Get(t.Context(), "not-a-uuid")
		var ec *exit.Error
		require.ErrorAs(t, err, &ec)
		assert.Equal(t, exit.Usage, ec.Code)
	})

	t.Run("not found propagates", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().GetMessage(gomock.Any(), messageID).
			Return(nil, exit.Generalf("traQ API error (HTTP 404)"))

		_, err := NewService(mc).Get(t.Context(), messageID)
		var ec *exit.Error
		require.ErrorAs(t, err, &ec)
		assert.Equal(t, exit.General, ec.Code)
	})
}
