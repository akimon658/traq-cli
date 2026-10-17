package cli

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/akimon658/traq-cli/internal/auth"
)

// authorizeURLFromOutput は traq auth login --manual の出力から認可 URL の行を取り出す。
func authorizeURLFromOutput(t *testing.T, out string) string {
	t.Helper()
	lines := strings.Split(out, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "Open this URL in a browser") {
			require.Greater(t, len(lines), i+1, "no URL after the label: %s", out)
			return lines[i+1]
		}
	}
	t.Fatalf("no authorize URL in the output: %s", out)
	return ""
}

// manualBrowserDecision は traq auth login --manual 用のブラウザシミュレーション。
// ローカルサーバーを立てない手動フローに合わせて、認可 URL への GET と同意フォームの
// 送信までを行い、リダイレクト先の URL (コールバックするべき URL) をそのまま返す。
func manualBrowserDecision(t *testing.T, sess *http.Cookie, submit, authorizeURL string) string {
	t.Helper()
	aURL, err := url.Parse(authorizeURL)
	require.NoError(t, err)
	state := aURL.Query().Get("state")
	require.NotEmpty(t, state, "no state in the authorize URL")
	require.Equal(
		t,
		auth.ManualRedirectURI,
		aURL.Query().Get("redirect_uri"),
		"manual login should use the registered redirect URI without a port",
	)

	authzResp, authzBody := requestNoRedirect(t, http.MethodGet, authorizeURL, sess, nil)
	require.Equal(
		t,
		http.StatusFound,
		authzResp.StatusCode,
		"authorize returned an unexpected response: %s",
		authzBody,
	)
	consent := authzResp.Header.Get("Location")
	assert.True(
		t,
		strings.HasPrefix(consent, "/consent"),
		"authorize should redirect to the consent page: %q", consent,
	)

	decideResp, decideBody := requestNoRedirect(
		t, http.MethodPost, aURL.Scheme+"://"+aURL.Host+"/api/v3/oauth2/authorize/decide",
		sess, url.Values{"submit": {submit}},
	)
	require.Equal(
		t,
		http.StatusFound,
		decideResp.StatusCode,
		"decide returned an unexpected response: %s",
		decideBody,
	)
	decision := decideResp.Header.Get("Location")
	require.NotEmpty(t, decision, "decide should redirect somewhere")
	if strings.HasPrefix(decision, "http") {
		return decision
	}
	return aURL.Scheme + "://" + aURL.Host + decision
}

func TestAuthLoginManualFlow(t *testing.T) {
	t.Parallel()
	requireTraqStack(t)
	clientID, sess := prepareOAuth(t)

	ctrl := gomock.NewController(t)
	ms := auth.NewMockStore(ctrl)
	mp := auth.NewMockSessionStore(ctrl)
	var pending auth.ManualSession
	var saved auth.Token
	gomock.InOrder(
		mp.EXPECT().Save(gomock.Any()).DoAndReturn(func(s auth.ManualSession) (string, error) {
			pending = s
			return "/tmp/login-session.json", nil
		}).Times(1),
		mp.EXPECT().Load(gomock.Any()).DoAndReturn(
			func(time.Time) (auth.ManualSession, bool, error) {
				return pending, true, nil
			},
		).Times(1),
		ms.EXPECT().Save(gomock.Any()).DoAndReturn(func(tok auth.Token) (string, error) {
			saved = tok
			return "keychain", nil
		}).Times(1),
		mp.EXPECT().Delete().Return(true, nil).Times(1),
	)

	e := &Env{Origin: testTraqOrigin, ClientID: clientID, Store: ms, Sessions: mp}
	out, err := runCLI(t, e, "auth", "login", "--manual")
	require.NoError(t, err, out)
	assert.Contains(t, out, "traq auth complete")

	// セッションには PKCE と redirect URI が記録される
	assert.Equal(t, auth.ManualRedirectURI, pending.RedirectURI)
	assert.Equal(t, testTraqOrigin, pending.Origin)
	assert.Equal(t, clientID, pending.ClientID)
	assert.Equal(t, []string{auth.ScopeRead}, pending.Scopes)
	assert.NotEmpty(t, pending.Verifier)
	assert.NotEmpty(t, pending.State)

	decision := manualBrowserDecision(t, sess, "approve", authorizeURLFromOutput(t, out))
	out, err = runCLI(t, e, "auth", "complete", decision)
	require.NoError(t, err, out)
	assert.Contains(t, out, "signed in: @traq (user) scopes: read")
	assert.Contains(t, out, "storage: keychain")

	assert.Equal(t, auth.MethodOAuth, saved.Method)
	assert.Equal(t, []string{auth.ScopeRead}, saved.Scope)
	assert.Equal(t, "traq", saved.UserName)
	assert.False(t, saved.UserBot)
	assert.Equal(t, testTraqOrigin, saved.Origin)
	assert.NotEmpty(t, saved.UserID)
	assert.NotEmpty(t, saved.RefreshToken, "refresh token should be issued")
}

func TestAuthLoginManualDenied(t *testing.T) {
	t.Parallel()
	requireTraqStack(t)
	clientID, sess := prepareOAuth(t)

	ctrl := gomock.NewController(t)
	ms := auth.NewMockStore(ctrl)
	mp := auth.NewMockSessionStore(ctrl)
	gomock.InOrder(
		// 拒否を parse した時点でエラーになるため Load / Save / Delete は呼ばれない
		mp.EXPECT().Save(gomock.Any()).Return("/tmp/login-session.json", nil).Times(1),
	)

	e := &Env{Origin: testTraqOrigin, ClientID: clientID, Store: ms, Sessions: mp}
	out, err := runCLI(t, e, "auth", "login", "--manual")
	require.NoError(t, err, out)

	decision := manualBrowserDecision(t, sess, "deny", authorizeURLFromOutput(t, out))
	out, err = runCLI(t, e, "auth", "complete", decision)
	require.Error(t, err, out)
	assert.Contains(t, err.Error(), "authorization was denied (access_denied)")
	var ec *ExitCodeError
	require.ErrorAs(t, err, &ec)
	assert.Equal(t, exitGeneral, ec.Code)
}

func TestAuthLoginManualStateMismatch(t *testing.T) {
	t.Parallel()

	mp := auth.NewMockSessionStore(gomock.NewController(t))
	mp.EXPECT().Load(gomock.Any()).DoAndReturn(
		func(time.Time) (auth.ManualSession, bool, error) {
			return auth.ManualSession{Verifier: "v", State: "right"}, true, nil
		},
	).Times(1)
	// state 不一致ではトークンを保存しない
	ms := auth.NewMockStore(gomock.NewController(t))

	e := &Env{Store: ms, Sessions: mp}
	_, err := runCLI(
		t, e, "auth", "complete",
		"https://q.trap.jp/callback?code=abc&state=wrong",
	)
	require.Error(t, err)
	var ec *ExitCodeError
	require.ErrorAs(t, err, &ec)
	assert.Equal(t, exitGeneral, ec.Code)
	assert.Contains(t, err.Error(), "state mismatch")
}

func TestAuthLoginManualNoPending(t *testing.T) {
	t.Parallel()

	mp := auth.NewMockSessionStore(gomock.NewController(t))
	mp.EXPECT().Load(gomock.Any()).Return(auth.ManualSession{}, false, nil).Times(1)

	e := &Env{Store: auth.NewMockStore(gomock.NewController(t)), Sessions: mp}
	_, err := runCLI(
		t, e, "auth", "complete",
		"https://q.trap.jp/callback?code=abc&state=s",
	)
	require.Error(t, err)
	var ec *ExitCodeError
	require.ErrorAs(t, err, &ec)
	assert.Equal(t, exitAuthRequired, ec.Code)
	assert.Contains(t, err.Error(), "no pending manual login")
}

func TestAuthLoginManualUsage(t *testing.T) {
	t.Parallel()

	t.Run("no client id", func(t *testing.T) {
		t.Parallel()
		e := &Env{
			Store:    auth.NewMockStore(gomock.NewController(t)),
			Sessions: auth.NewMockSessionStore(gomock.NewController(t)),
		}
		_, err := runCLI(t, e, "auth", "login", "--manual")
		require.Error(t, err)
		var ec *ExitCodeError
		require.ErrorAs(t, err, &ec)
		assert.Equal(t, exitUsage, ec.Code)
		assert.Contains(t, err.Error(), "client ID is not configured")
	})

	t.Run("browser and manual", func(t *testing.T) {
		t.Parallel()
		_, err := runCLI(t, botEnv(t, ""), "auth", "login", "--browser", "--manual")
		require.Error(t, err)
		var ec *ExitCodeError
		require.ErrorAs(t, err, &ec)
		assert.Equal(t, exitUsage, ec.Code)
		assert.Contains(t, err.Error(), "cannot combine --browser with --manual")
	})

	t.Run("token and manual", func(t *testing.T) {
		t.Parallel()
		_, err := runCLI(t, botEnv(t, ""), "auth", "login", "--manual", "--token", "t")
		require.Error(t, err)
		var ec *ExitCodeError
		require.ErrorAs(t, err, &ec)
		assert.Equal(t, exitUsage, ec.Code)
		assert.Contains(t, err.Error(), "cannot combine --token with --manual")
	})
}

func TestAuthCompleteUsage(t *testing.T) {
	t.Parallel()

	e := &Env{
		Store:    auth.NewMockStore(gomock.NewController(t)),
		Sessions: auth.NewMockSessionStore(gomock.NewController(t)),
	}
	for _, tc := range []struct {
		name string
		arg  string
		want string
	}{
		{"empty", "", "no code in the callback"},
		{"garbage", "%%bad", "neither a URL nor a query string"},
		{"no code", "https://q.trap.jp/callback?state=s", "no code in the callback"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := runCLI(t, e, "auth", "complete", tc.arg)
			require.Error(t, err)
			var ec *ExitCodeError
			require.ErrorAs(t, err, &ec)
			assert.Equal(t, exitUsage, ec.Code)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestParseCallback(t *testing.T) {
	t.Parallel()

	t.Run("full URL", func(t *testing.T) {
		t.Parallel()
		code, state, denied, err := parseCallback(
			"http://127.0.0.1/callback?code=c&state=s&scope=read",
		)
		require.NoError(t, err)
		assert.Nil(t, denied)
		assert.Equal(t, "c", code)
		assert.Equal(t, "s", state)
	})

	t.Run("query string", func(t *testing.T) {
		t.Parallel()
		code, state, denied, err := parseCallback("code=c&state=s")
		require.NoError(t, err)
		assert.Nil(t, denied)
		assert.Equal(t, "c", code)
		assert.Equal(t, "s", state)
	})

	t.Run("leading question mark", func(t *testing.T) {
		t.Parallel()
		code, state, denied, err := parseCallback("?code=c")
		require.NoError(t, err)
		assert.Nil(t, denied)
		assert.Equal(t, "c", code)
		// state が無い場合は PKCE に縛られるため空で受理する
		assert.Empty(t, state)
	})

	t.Run("denied", func(t *testing.T) {
		t.Parallel()
		_, _, denied, err := parseCallback(
			"http://127.0.0.1/callback?error=access_denied&state=s",
		)
		require.NoError(t, err)
		require.NotNil(t, denied)
		assert.Contains(t, denied.Error(), "authorization was denied (access_denied)")

		_, _, denied, _ = parseCallback(
			"http://127.0.0.1/callback?error=access_denied&error_description=no",
		)
		require.NotNil(t, denied)
		assert.Contains(t, denied.Error(), "access_denied: no")
	})
}
