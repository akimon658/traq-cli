package traq

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/moznion/go-optional"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/akimon658/traq-cli/internal/domain/auth"
	"github.com/akimon658/traq-cli/internal/foundation/exit"
)

func TestService_LoginWithToken(t *testing.T) {
	t.Run("bot ok", func(t *testing.T) {
		t.Parallel()
		requireTraqStack(t)

		ms := auth.NewMockStore(gomock.NewController(t))
		var saved auth.Token
		ms.EXPECT().Save(gomock.Any()).DoAndReturn(func(tok auth.Token) (string, error) {
			saved = tok
			return "keychain", nil
		}).Times(1)

		svc := NewService(testTraqOrigin, "test", ms, nil)
		got, venue, err := svc.LoginWithToken(t.Context(), testBotToken)
		require.NoError(t, err)
		assert.Equal(t, "keychain", venue)
		assert.Equal(t, auth.MethodBot, got.Method)
		assert.Equal(t, testBotToken, got.AccessToken)
		assert.Equal(t, testTraqOrigin, got.Origin)
		assert.NotEmpty(t, got.UserName)
		assert.Equal(t, saved, got)
	})

	t.Run("invalid token", func(t *testing.T) {
		t.Parallel()
		requireTraqStack(t)

		ms := auth.NewMockStore(gomock.NewController(t))
		svc := NewService(testTraqOrigin, "test", ms, nil)
		_, _, err := svc.LoginWithToken(t.Context(), "invalid")
		var ec *exit.Error
		require.ErrorAs(t, err, &ec)
		assert.Equal(t, exit.General, ec.Code)
		assert.Contains(t, err.Error(), "the token was rejected")
	})
}

func TestService_Logout(t *testing.T) {
	t.Run("token login and logout", func(t *testing.T) {
		t.Parallel()
		requireTraqStack(t)

		ms := auth.NewMockStore(gomock.NewController(t))
		var saved auth.Token
		gomock.InOrder(
			ms.EXPECT().Save(gomock.Any()).DoAndReturn(func(tok auth.Token) (string, error) {
				saved = tok
				return "keychain", nil
			}).Times(1),
			ms.EXPECT().Load().DoAndReturn(func() (auth.Token, string, bool, error) {
				return saved, "keychain", true, nil
			}).Times(1),
			ms.EXPECT().Delete().Return(true, nil).Times(1),
			ms.EXPECT().Load().Return(auth.Token{}, "", false, nil).Times(1),
		)
		svc := NewService(testTraqOrigin, "test", ms, nil)

		_, _, err := svc.LoginWithToken(t.Context(), testBotToken)
		require.NoError(t, err)

		_, err = svc.Logout(t.Context())
		require.NoError(t, err)

		_, err = svc.Logout(t.Context())
		var ec *exit.Error
		require.ErrorAs(t, err, &ec)
		assert.Equal(t, exit.AuthRequired, ec.Code)
	})
}

func TestService_Status(t *testing.T) {
	t.Run("bot via environment", func(t *testing.T) {
		t.Parallel()
		requireTraqStack(t)

		svc := NewService(testTraqOrigin, "test", nil, nil)
		st, err := svc.Status(t.Context(), "", testBotToken)
		require.NoError(t, err)
		assert.Equal(t, "environment", st.Source)
		require.NoError(t, st.VerifyErr)
		require.NotNil(t, st.Me)
		assert.True(t, st.Me.Bot)
		assert.Equal(t, st.Me.Id, st.Token.UserID)
	})

	t.Run("invalid token verify error", func(t *testing.T) {
		t.Parallel()
		requireTraqStack(t)

		svc := NewService(testTraqOrigin, "test", nil, nil)
		st, err := svc.Status(t.Context(), "invalid", "")
		require.NoError(t, err, "Status returns VerifyErr instead of error for bad tokens")
		require.Error(t, st.VerifyErr)
	})
}

func TestClient_GetFile(t *testing.T) {
	t.Run("meta and body roundtrip", func(t *testing.T) {
		t.Parallel()
		requireTraqStack(t)

		content := []byte("hello traq-cli file download\n")
		src := filepath.Join(t.TempDir(), "upload.txt")
		require.NoError(t, os.WriteFile(src, content, 0o600))
		f, err := os.Open(src)
		require.NoError(t, err)
		defer func() { _ = f.Close() }()

		info, _, err := traqClient(testTraqOrigin, optional.None[*http.Cookie]()).
			FileAPI.PostFile(TokenContext(t.Context(), testBotToken)).
			ChannelId(testGeneralChannelID(t)).
			File(f).
			Execute()
		require.NoError(t, err, "failed to upload a test file")

		c := testBotClient(t)
		meta, err := c.GetFileMeta(t.Context(), info.Id)
		require.NoError(t, err)
		assert.Equal(t, info.Id, meta.Id)

		tmp, err := c.GetFile(t.Context(), info.Id)
		require.NoError(t, err)
		defer func() {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
		}()
		got, err := os.ReadFile(tmp.Name())
		require.NoError(t, err)
		assert.Equal(t, content, got)
	})
}
