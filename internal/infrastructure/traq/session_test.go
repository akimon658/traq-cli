package traq

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/akimon658/traq-cli/internal/domain/auth"
	"github.com/akimon658/traq-cli/internal/foundation/exit"
)

func TestService_ResolveToken(t *testing.T) {
	t.Parallel()

	t.Run("prefers environment", func(t *testing.T) {
		t.Parallel()
		ms := auth.NewMockStore(gomock.NewController(t))
		svc := NewService(auth.DefaultOrigin, "client", ms, nil)
		rt, err := svc.ResolveToken(t.Context(), "", "env-token")
		require.NoError(t, err)
		assert.Equal(t, "environment", rt.Source)
		assert.Equal(t, "env-token", rt.Token.AccessToken)
	})

	t.Run("prefers flag", func(t *testing.T) {
		t.Parallel()
		ms := auth.NewMockStore(gomock.NewController(t))
		svc := NewService(auth.DefaultOrigin, "client", ms, nil)
		rt, err := svc.ResolveToken(t.Context(), "flag-token", "env-token")
		require.NoError(t, err)
		assert.Equal(t, "flag", rt.Source)
		assert.Equal(t, "flag-token", rt.Token.AccessToken)
	})

	t.Run("from store", func(t *testing.T) {
		t.Parallel()
		ms := auth.NewMockStore(gomock.NewController(t))
		ms.EXPECT().Load().Return(auth.Token{
			Method:      auth.MethodOAuth,
			AccessToken: "stored-token",
			Scope:       []string{auth.ScopeRead},
			Origin:      auth.DefaultOrigin,
			ExpiresAt:   time.Now().Add(24 * time.Hour),
		}, "keychain", true, nil).Times(1)
		svc := NewService(auth.DefaultOrigin, "client", ms, nil)
		rt, err := svc.ResolveToken(t.Context(), "", "")
		require.NoError(t, err)
		assert.Equal(t, "keychain", rt.Source)
		assert.Equal(t, "stored-token", rt.Token.AccessToken)
	})

	t.Run("not authenticated", func(t *testing.T) {
		t.Parallel()
		ms := auth.NewMockStore(gomock.NewController(t))
		ms.EXPECT().Load().Return(auth.Token{}, "", false, nil).Times(1)
		svc := NewService(auth.DefaultOrigin, "client", ms, nil)
		_, err := svc.ResolveToken(t.Context(), "", "")
		var ec *exit.Error
		require.ErrorAs(t, err, &ec)
		assert.Equal(t, exit.AuthRequired, ec.Code)
	})

	t.Run("expired without refresh token", func(t *testing.T) {
		t.Parallel()
		ms := auth.NewMockStore(gomock.NewController(t))
		ms.EXPECT().Load().Return(auth.Token{
			Method:      auth.MethodOAuth,
			AccessToken: "expired-token",
			ExpiresAt:   time.Now().Add(-1 * time.Hour),
		}, "keychain", true, nil).Times(1)
		svc := NewService(auth.DefaultOrigin, "client", ms, nil)
		_, err := svc.ResolveToken(t.Context(), "", "")
		var ec *exit.Error
		require.ErrorAs(t, err, &ec)
		assert.Equal(t, exit.AuthRequired, ec.Code)
	})

	t.Run("expired refreshes and saves", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/v3/oauth2/token", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "new-access",
				"refresh_token": "new-refresh",
				"expires_in":    3600,
				"scope":         "read",
			})
		}))
		defer srv.Close()

		ms := auth.NewMockStore(gomock.NewController(t))
		stored := auth.Token{
			Method:       auth.MethodOAuth,
			AccessToken:  "old-access",
			RefreshToken: "old-refresh",
			Scope:        []string{auth.ScopeRead},
			Origin:       srv.URL,
			ExpiresAt:    time.Now().Add(-time.Hour),
		}
		ms.EXPECT().Load().Return(stored, "keychain", true, nil).Times(1)
		ms.EXPECT().Save(gomock.Any()).DoAndReturn(func(tok auth.Token) (string, error) {
			assert.Equal(t, "new-access", tok.AccessToken)
			assert.Equal(t, "new-refresh", tok.RefreshToken)
			return "keychain", nil
		}).Times(1)
		svc := NewService(srv.URL, "client", ms, nil)
		rt, err := svc.ResolveToken(t.Context(), "", "")
		require.NoError(t, err)
		assert.Equal(t, "new-access", rt.Token.AccessToken)
	})
}
