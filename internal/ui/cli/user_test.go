package cli

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gotraq "github.com/traPtitech/go-traq"
	"go.uber.org/mock/gomock"

	"github.com/akimon658/traq-cli/internal/foundation/exit"
	"github.com/akimon658/traq-cli/internal/traq"
)

func TestUserMe(t *testing.T) {
	t.Parallel()

	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().GetMe(gomock.Any()).Return(&gotraq.MyUserDetail{Id: "u1", Name: "alice"}, nil)

		_, err := runCLIWithResolver(
			t, unitEnv(), stubResolve(mc, nil), "user", "me",
		)
		require.NoError(t, err)
	})

	t.Run("resolve error propagates", func(t *testing.T) {
		t.Parallel()
		resolveErr := exit.AuthRequiredf("the token was rejected")
		_, err := runCLIWithResolver(
			t, unitEnv(), stubResolve(nil, resolveErr), "user", "me",
		)
		var ec *exit.Error
		require.ErrorAs(t, err, &ec)
		assert.Equal(t, exit.AuthRequired, ec.Code)
	})

	t.Run("domain error propagates", func(t *testing.T) {
		t.Parallel()
		mc := traq.NewMockClient(gomock.NewController(t))
		mc.EXPECT().GetMe(gomock.Any()).Return(nil, errors.New("me failed"))

		_, err := runCLIWithResolver(
			t, unitEnv(), stubResolve(mc, nil), "user", "me",
		)
		require.Error(t, err)
	})
}
