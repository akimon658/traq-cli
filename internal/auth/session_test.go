package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/akimon658/traq-cli/internal/foundation/exit"
)

func TestResolveTokenPrefersEnvironment(t *testing.T) {
	t.Parallel()
	ms := NewMockStore(gomock.NewController(t))
	rt, err := ResolveToken(t.Context(), DefaultOrigin, "client", "", "env-token", ms)
	require.NoError(t, err)
	assert.Equal(t, "environment", rt.Source)
	assert.Equal(t, "env-token", rt.Token.AccessToken)
}

func TestResolveTokenPrefersFlag(t *testing.T) {
	t.Parallel()
	ms := NewMockStore(gomock.NewController(t))
	rt, err := ResolveToken(t.Context(), DefaultOrigin, "client", "flag-token", "env-token", ms)
	require.NoError(t, err)
	assert.Equal(t, "flag", rt.Source)
	assert.Equal(t, "flag-token", rt.Token.AccessToken)
}

func TestResolveTokenFromStore(t *testing.T) {
	t.Parallel()
	ms := NewMockStore(gomock.NewController(t))
	ms.EXPECT().Load().Return(Token{
		Method:      MethodOAuth,
		AccessToken: "stored-token",
		Scope:       []string{ScopeRead},
		Origin:      DefaultOrigin,
		ExpiresAt:   time.Now().Add(24 * time.Hour),
	}, "keychain", true, nil).Times(1)
	rt, err := ResolveToken(t.Context(), DefaultOrigin, "client", "", "", ms)
	require.NoError(t, err)
	assert.Equal(t, "keychain", rt.Source)
	assert.Equal(t, "stored-token", rt.Token.AccessToken)
}

func TestResolveTokenNotAuthenticated(t *testing.T) {
	t.Parallel()
	ms := NewMockStore(gomock.NewController(t))
	ms.EXPECT().Load().Return(Token{}, "", false, nil).Times(1)
	_, err := ResolveToken(t.Context(), DefaultOrigin, "client", "", "", ms)
	var ec *exit.Error
	require.ErrorAs(t, err, &ec)
	assert.Equal(t, exit.AuthRequired, ec.Code)
}

func TestResolveTokenExpiredWithoutRefreshToken(t *testing.T) {
	t.Parallel()
	ms := NewMockStore(gomock.NewController(t))
	ms.EXPECT().Load().Return(Token{
		Method:      MethodOAuth,
		AccessToken: "expired-token",
		ExpiresAt:   time.Now().Add(-1 * time.Hour),
	}, "keychain", true, nil).Times(1)
	_, err := ResolveToken(t.Context(), DefaultOrigin, "client", "", "", ms)
	var ec *exit.Error
	require.ErrorAs(t, err, &ec)
	assert.Equal(t, exit.AuthRequired, ec.Code)
}
