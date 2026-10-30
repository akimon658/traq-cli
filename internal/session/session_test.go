package session

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/akimon658/traq-cli/internal/auth"
	"github.com/akimon658/traq-cli/internal/config"
	"github.com/akimon658/traq-cli/internal/exit"
)

func TestResolveTokenPrefersEnvironment(t *testing.T) {
	t.Parallel()
	ms := auth.NewMockStore(gomock.NewController(t))
	e := &config.Env{
		Origin:      auth.DefaultOrigin,
		ClientID:    "client",
		Store:       ms,
		AccessToken: "env-token",
	}
	rt, err := ResolveToken(t.Context(), e)
	require.NoError(t, err)
	assert.Equal(t, "environment", rt.Source)
	assert.Equal(t, "env-token", rt.Token.AccessToken)
}

func TestResolveTokenPrefersFlag(t *testing.T) {
	t.Parallel()
	ms := auth.NewMockStore(gomock.NewController(t))
	e := &config.Env{
		Origin:        auth.DefaultOrigin,
		ClientID:      "client",
		Store:         ms,
		AccessToken:   "env-token",
		TokenOverride: "flag-token",
	}
	rt, err := ResolveToken(t.Context(), e)
	require.NoError(t, err)
	assert.Equal(t, "flag", rt.Source)
	assert.Equal(t, "flag-token", rt.Token.AccessToken)
}

func TestResolveTokenFromStore(t *testing.T) {
	t.Parallel()
	ms := auth.NewMockStore(gomock.NewController(t))
	ms.EXPECT().Load().Return(auth.Token{
		Method:      auth.MethodOAuth,
		AccessToken: "stored-token",
		Scope:       []string{auth.ScopeRead},
		Origin:      auth.DefaultOrigin,
		ExpiresAt:   time.Now().Add(24 * time.Hour),
	}, "keychain", true, nil).Times(1)
	e := &config.Env{Origin: auth.DefaultOrigin, ClientID: "client", Store: ms}
	rt, err := ResolveToken(t.Context(), e)
	require.NoError(t, err)
	assert.Equal(t, "keychain", rt.Source)
	assert.Equal(t, "stored-token", rt.Token.AccessToken)
}

func TestResolveTokenNotAuthenticated(t *testing.T) {
	t.Parallel()
	ms := auth.NewMockStore(gomock.NewController(t))
	ms.EXPECT().Load().Return(auth.Token{}, "", false, nil).Times(1)
	e := &config.Env{Origin: auth.DefaultOrigin, ClientID: "client", Store: ms}
	_, err := ResolveToken(t.Context(), e)
	var ec *exit.Error
	require.ErrorAs(t, err, &ec)
	assert.Equal(t, exit.AuthRequired, ec.Code)
}

func TestResolveTokenExpiredWithoutRefreshToken(t *testing.T) {
	t.Parallel()
	ms := auth.NewMockStore(gomock.NewController(t))
	ms.EXPECT().Load().Return(auth.Token{
		Method:      auth.MethodOAuth,
		AccessToken: "expired-token",
		ExpiresAt:   time.Now().Add(-1 * time.Hour),
	}, "keychain", true, nil).Times(1)
	e := &config.Env{Origin: auth.DefaultOrigin, ClientID: "client", Store: ms}
	_, err := ResolveToken(t.Context(), e)
	var ec *exit.Error
	require.ErrorAs(t, err, &ec)
	assert.Equal(t, exit.AuthRequired, ec.Code)
}
