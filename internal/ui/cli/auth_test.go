package cli

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/akimon658/traq-cli/internal/auth"
	"github.com/akimon658/traq-cli/internal/foundation/config"
	"github.com/akimon658/traq-cli/internal/foundation/exit"
	"github.com/akimon658/traq-cli/internal/traq"
)

func TestAuthLogin(t *testing.T) {
	t.Parallel()

	t.Run("usage without flags", func(t *testing.T) {
		t.Parallel()
		_, err := runCLI(t, unitEnv(), "auth", "login")
		var ec *exit.Error
		require.ErrorAs(t, err, &ec)
		assert.Equal(t, exit.Usage, ec.Code)
		assert.Contains(t, err.Error(), "specify either --browser, --manual or --token")
	})

	t.Run("usage browser and manual", func(t *testing.T) {
		t.Parallel()
		_, err := runCLI(t, unitEnv(), "auth", "login", "--browser", "--manual")
		var ec *exit.Error
		require.ErrorAs(t, err, &ec)
		assert.Equal(t, exit.Usage, ec.Code)
		assert.Contains(t, err.Error(), "cannot combine --browser with --manual")
	})

	t.Run("usage token and manual", func(t *testing.T) {
		t.Parallel()
		_, err := runCLI(t, unitEnv(), "auth", "login", "--manual", "--token", "t")
		var ec *exit.Error
		require.ErrorAs(t, err, &ec)
		assert.Equal(t, exit.Usage, ec.Code)
		assert.Contains(t, err.Error(), "cannot combine --token with --manual")
	})

	t.Run("usage manual without client id", func(t *testing.T) {
		t.Parallel()
		e := &config.Env{
			Store:    traq.NewMockStore(gomock.NewController(t)),
			Sessions: auth.NewMockSessionStore(gomock.NewController(t)),
		}
		_, err := runCLI(t, e, "auth", "login", "--manual")
		var ec *exit.Error
		require.ErrorAs(t, err, &ec)
		assert.Equal(t, exit.Usage, ec.Code)
		assert.Contains(t, err.Error(), "client ID is not configured")
	})
}

func TestAuthComplete(t *testing.T) {
	t.Parallel()

	newEnv := func(t *testing.T) *config.Env {
		t.Helper()
		return &config.Env{
			Store:    traq.NewMockStore(gomock.NewController(t)),
			Sessions: auth.NewMockSessionStore(gomock.NewController(t)),
		}
	}

	t.Run("usage empty", func(t *testing.T) {
		t.Parallel()
		_, err := runCLI(t, newEnv(t), "auth", "complete", "")
		var ec *exit.Error
		require.ErrorAs(t, err, &ec)
		assert.Equal(t, exit.Usage, ec.Code)
		assert.Contains(t, err.Error(), "no code in the callback")
	})

	t.Run("usage garbage", func(t *testing.T) {
		t.Parallel()
		_, err := runCLI(t, newEnv(t), "auth", "complete", "%%bad")
		var ec *exit.Error
		require.ErrorAs(t, err, &ec)
		assert.Equal(t, exit.Usage, ec.Code)
		assert.Contains(t, err.Error(), "neither a URL nor a query string")
	})

	t.Run("usage no code", func(t *testing.T) {
		t.Parallel()
		_, err := runCLI(t, newEnv(t), "auth", "complete", "https://q.trap.jp/callback?state=s")
		var ec *exit.Error
		require.ErrorAs(t, err, &ec)
		assert.Equal(t, exit.Usage, ec.Code)
		assert.Contains(t, err.Error(), "no code in the callback")
	})

	t.Run("state mismatch", func(t *testing.T) {
		t.Parallel()
		mp := auth.NewMockSessionStore(gomock.NewController(t))
		mp.EXPECT().Load(gomock.Any()).DoAndReturn(
			func(time.Time) (auth.ManualSession, bool, error) {
				return auth.ManualSession{Verifier: "v", State: "right"}, true, nil
			},
		).Times(1)
		ms := traq.NewMockStore(gomock.NewController(t))

		e := &config.Env{Store: ms, Sessions: mp}
		_, err := runCLI(
			t, e, "auth", "complete",
			"https://q.trap.jp/callback?code=abc&state=wrong",
		)
		var ec *exit.Error
		require.ErrorAs(t, err, &ec)
		assert.Equal(t, exit.General, ec.Code)
		assert.Contains(t, err.Error(), "state mismatch")
	})

	t.Run("no pending login", func(t *testing.T) {
		t.Parallel()
		mp := auth.NewMockSessionStore(gomock.NewController(t))
		mp.EXPECT().Load(gomock.Any()).Return(auth.ManualSession{}, false, nil).Times(1)

		e := &config.Env{Store: traq.NewMockStore(gomock.NewController(t)), Sessions: mp}
		_, err := runCLI(
			t, e, "auth", "complete",
			"https://q.trap.jp/callback?code=abc&state=s",
		)
		var ec *exit.Error
		require.ErrorAs(t, err, &ec)
		assert.Equal(t, exit.AuthRequired, ec.Code)
		assert.Contains(t, err.Error(), "no pending manual login")
	})
}
