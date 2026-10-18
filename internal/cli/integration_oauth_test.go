package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/akimon658/traq-cli/internal/auth"
)

// noRedirectClient はリダイレクトを追従しない HTTP クライアント。
// traQ サーバーが発行する 302 の Location をテストから直接確認するために使う。
var noRedirectClient = &http.Client{
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

// prepareOAuth は管理者セッションを取得し、OAuth2 公開クライアントを 1 つ登録して
// client ID を返す。クライアントはテストごとに作成し、毎回 init コードを経由させる。
func prepareOAuth(t *testing.T) (clientID string, sess *http.Cookie) {
	t.Helper()
	sess, err := adminSession(testTraqOrigin)
	require.NoError(t, err, "failed to login as the admin user")
	clientID = createOAuthClient(t, sess)
	return clientID, sess
}

// createOAuthClient は管理者セッションで OAuth2 公開クライアントを登録し、client ID を返す。
func createOAuthClient(t *testing.T, sess *http.Cookie) string {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		// traQ のクライアント名には 1〜32 文字の制限がある
		"name":        "cli-" + strings.ReplaceAll(uuid.New().String(), "-", "")[:27],
		"description": "traQ CLI integration test client",
		"callbackUrl": "http://127.0.0.1/callback",
		"scopes":      []string{auth.ScopeRead, auth.ScopeWrite},
	})
	require.NoError(t, err)
	req, err := http.NewRequestWithContext(
		t.Context(), http.MethodPost, testTraqOrigin+"/api/v3/clients", bytes.NewReader(body),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sess)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(
		t,
		http.StatusCreated,
		resp.StatusCode,
		"failed to create an OAuth client: %s",
		raw,
	)
	var created struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(raw, &created))
	require.NotEmpty(t, created.ID, "no client id in the response: %s", raw)
	return created.ID
}

// simObservations はブラウザ役の HTTP シミュレーションで観測した値を記録する。
type simObservations struct {
	err             error
	consentLocation string
	code            string
	state           string
}

// browserSim は traq-cli のブラウザログインでブラウザが行う操作を模倣する関数を返す:
//
//  1. 認可 URL をセッションクッキー付きで GET (未ログインなら /login へのリダイレクト)
//  2. POST /api/v3/oauth2/authorize/decide で submit=approve|deny を送る
//  3. リダイレクト先 (traq-cli のコールバックサーバー) を GET して code を届ける
//
// 失敗した場合もパラメータ無しでコールバック URI に GET し、CLI をすぐに完了させて
// テストが hang しないようにする。
func browserSim(
	t *testing.T,
	sess *http.Cookie,
	submit string,
	obs *simObservations,
) func(url string) {
	t.Helper()
	return func(authorizeURL string) {
		t.Helper()
		aURL, err := url.Parse(authorizeURL)
		var state, redirectURI string
		if err == nil {
			state = aURL.Query().Get("state")
			redirectURI = aURL.Query().Get("redirect_uri")
		}
		defer func() {
			if obs.err == nil {
				return
			}
			// コールバックサーバーにノイズを送り、CLI を即座にエラー終了させる
			if redirectURI != "" {
				_, _ = requestNoRedirect(t, http.MethodGet, redirectURI, nil, nil)
			}
		}()
		if err != nil {
			obs.err = fmt.Errorf("failed to parse the authorize URL: %w", err)
			return
		}
		require.NotEmpty(t, state, "no state in the authorize URL")
		require.NotEmpty(t, redirectURI, "no redirect_uri in the authorize URL")

		// (1) 認可リクエスト
		authzResp, authzBody := requestNoRedirect(t, http.MethodGet, authorizeURL, sess, nil)
		if authzResp.StatusCode != http.StatusFound {
			obs.err = fmt.Errorf(
				"authorize returned %d, expected 302: %s", authzResp.StatusCode, authzBody,
			)
			return
		}
		obs.consentLocation = authzResp.Header.Get("Location")
		assert.True(t, strings.HasPrefix(obs.consentLocation, "/consent"),
			"authorize should redirect to the consent page: %q", obs.consentLocation)
		assert.Contains(t, obs.consentLocation, "state="+state)

		// (2) 同意フォームの送信 (CSRF は無いためセッションクッキーだけで送れる)
		decideResp, decideBody := requestNoRedirect(
			t, http.MethodPost, aURL.Scheme+"://"+aURL.Host+"/api/v3/oauth2/authorize/decide",
			sess, url.Values{"submit": {submit}},
		)
		if decideResp.StatusCode != http.StatusFound {
			obs.err = fmt.Errorf(
				"decide returned %d, expected 302: %s", decideResp.StatusCode, decideBody,
			)
			return
		}
		decision := decideResp.Header.Get("Location")
		dURL, err := url.Parse(decision)
		if err != nil {
			obs.err = fmt.Errorf("failed to parse the decision redirect: %w", err)
			return
		}
		if submit == "approve" {
			obs.code = dURL.Query().Get("code")
			obs.state = dURL.Query().Get("state")
			if obs.code == "" {
				obs.err = fmt.Errorf("no code in the decision redirect: %s", decision)
				return
			}
			assert.Equal(t, state, obs.state, "decision redirect state should match")
		}

		// (3) リダイレクト先を辿って code / error を traq-cli のコールバックサーバーに届ける
		callbackResp, callbackBody := requestNoRedirect(t, http.MethodGet, decision, nil, nil)
		wantStatus := http.StatusOK
		if submit != "approve" {
			wantStatus = http.StatusBadRequest
		}
		if callbackResp.StatusCode != wantStatus {
			obs.err = fmt.Errorf(
				"callback returned %d, expected %d: %s",
				callbackResp.StatusCode, wantStatus, callbackBody,
			)
		}
	}
}

type loginResult struct {
	out string
	err error
	sim *simObservations
}

// driveOAuthLogin は `traq auth login --browser` 相当の OAuth フローを実行し、
// CLI の出力・エラーとブラウザ役シミュレーションの観測結果を返す。
func driveOAuthLogin(
	t *testing.T,
	clientID string,
	sess *http.Cookie,
	ms *auth.MockStore,
	scopeFlag, submit string,
) loginResult {
	t.Helper()
	obs := &simObservations{}
	var res loginResult
	var buf bytes.Buffer
	res.err = login(
		t.Context(),
		&Env{Origin: testTraqOrigin, ClientID: clientID, Store: ms},
		scopeFlag,
		browserSim(t, sess, submit, obs),
		&buf,
	)
	res.out = buf.String()
	res.sim = obs
	return res
}

func TestAuthLoginBrowserRead(t *testing.T) {
	t.Parallel()
	requireTraqStack(t)
	clientID, sess := prepareOAuth(t)

	ms := auth.NewMockStore(gomock.NewController(t))
	var saved auth.Token
	gomock.InOrder(
		ms.EXPECT().Save(gomock.Any()).DoAndReturn(func(tok auth.Token) (string, error) {
			saved = tok
			return "keychain", nil
		}).Times(1),
		// users me と message send がそれぞれ Load する
		ms.EXPECT().Load().DoAndReturn(func() (auth.Token, string, bool, error) {
			return saved, "keychain", true, nil
		}).Times(2),
	)

	res := driveOAuthLogin(t, clientID, sess, ms, auth.ScopeRead, "approve")
	require.NoError(t, res.sim.err, "the browser simulation failed")
	require.NoError(t, res.err, "browser login failed, output: %s", res.out)
	assert.Contains(t, res.out, "signed in: @traq (user) scopes: read")
	assert.Contains(t, res.out, "storage: keychain")

	assert.Equal(t, auth.MethodOAuth, saved.Method)
	assert.Equal(t, []string{auth.ScopeRead}, saved.Scope)
	assert.Equal(t, "traq", saved.UserName)
	assert.False(t, saved.UserBot)
	assert.Equal(t, testTraqOrigin, saved.Origin)
	assert.NotEmpty(t, saved.UserID)
	assert.False(t, saved.ExpiresAt.IsZero(), "OAuth token should have an expiry")
	assert.NotEmpty(t, saved.RefreshToken, "refresh token should be issued")

	// 保存済みトークンで users me
	e := overrideEnv(t, ms, clientID)
	out, err := runCLI(t, e, "user", "me")
	require.NoError(t, err, "users me with the stored oauth token failed")
	assert.NotContains(t, out, `"id"`, "output should be human-readable text, not JSON")
	assert.Contains(t, out, "@traq")

	// read スコープでは message send はスコープ不足
	out, err = runCLI(t, e, "message", "send", "#general", "should be rejected by the scope check")
	require.Error(t, err, "expected scope error, output: %s", out)
	var ec *ExitCodeError
	require.ErrorAs(t, err, &ec)
	assert.Equal(t, exitScopeInsufficient, ec.Code)
	assert.Contains(t, err.Error(), "does not have the 'write' scope")
}

func TestAuthLoginBrowserWrite(t *testing.T) {
	t.Parallel()
	requireTraqStack(t)
	clientID, sess := prepareOAuth(t)

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
	)

	res := driveOAuthLogin(t, clientID, sess, ms, "read,write", "approve")
	require.NoError(t, res.sim.err, "the browser simulation failed")
	require.NoError(t, res.err, "browser login failed, output: %s", res.out)
	assert.Contains(t, res.out, "signed in: @traq (user) scopes: read,write")
	assert.Equal(t, []string{auth.ScopeRead, auth.ScopeWrite}, saved.Scope)

	const content = "hello traq-cli as oauth (read,write)"
	e := overrideEnv(t, ms, clientID)
	out, err := runCLI(t, e, "message", "send", "#general", content)
	require.NoError(t, err, "message send with the oauth token failed")
	assertPosted(t, out, content)
}

func TestAuthLoginBrowserDenied(t *testing.T) {
	t.Parallel()
	requireTraqStack(t)
	clientID, sess := prepareOAuth(t)

	// Save への期待値を設定しない: Save が呼ばれたら gomock がテストを失敗させる
	ms := auth.NewMockStore(gomock.NewController(t))

	res := driveOAuthLogin(t, clientID, sess, ms, auth.ScopeRead, "deny")
	require.NoError(t, res.sim.err, "the browser simulation failed")
	require.Error(t, res.err, "denied authorization should fail the login, output: %s", res.out)
	assert.Contains(t, res.err.Error(), "authorization was denied (access_denied)")
}

func TestAuthLoginTokenOAuthType(t *testing.T) {
	t.Parallel()
	requireTraqStack(t)
	clientID, sess := prepareOAuth(t)

	ms := auth.NewMockStore(gomock.NewController(t))
	var saves []auth.Token
	ms.EXPECT().Save(gomock.Any()).DoAndReturn(func(tok auth.Token) (string, error) {
		saves = append(saves, tok)
		return "keychain", nil
	}).Times(2)

	res := driveOAuthLogin(t, clientID, sess, ms, auth.ScopeRead, "approve")
	require.NoError(t, res.sim.err, "the browser simulation failed")
	require.NoError(t, res.err, "browser login failed, output: %s", res.out)
	require.Len(t, saves, 1)
	oauthAccessToken := saves[0].AccessToken

	// OAuth トークンを --token でログインすると、/users/me の応答から oauth と判別される
	e := overrideEnv(t, ms, clientID)
	out, err := runCLI(t, e, "auth", "login", "--token", oauthAccessToken)
	require.NoError(t, err, "token login failed")
	assert.Contains(t, out, "signed in: @traq (user)")
	require.Len(t, saves, 2)
	assert.Equal(t, oauthAccessToken, saves[1].AccessToken)
	assert.Equal(t, auth.MethodOAuth, saves[1].Method)
	assert.False(t, saves[1].UserBot)
	assert.Equal(t, "traq", saves[1].UserName)
	assert.Equal(t, testTraqOrigin, saves[1].Origin)
}

func TestAuthStatusOAuth(t *testing.T) {
	t.Parallel()
	requireTraqStack(t)
	clientID, sess := prepareOAuth(t)

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
	)

	res := driveOAuthLogin(t, clientID, sess, ms, auth.ScopeRead, "approve")
	require.NoError(t, res.sim.err, "the browser simulation failed")
	require.NoError(t, res.err, "browser login failed, output: %s", res.out)

	e := overrideEnv(t, ms, clientID)
	out, err := runCLI(t, e, "auth", "status")
	require.NoError(t, err, "auth status failed")
	assert.NotContains(t, out, `"origin"`, "output should be human-readable text, not JSON")
	for _, want := range []string{
		"origin: " + testTraqOrigin,
		"source: keychain",
		"method: oauth",
		"scopes: " + auth.ScopeRead,
		"userId: " + saved.UserID,
		"user: @" + saved.UserName,
		"expiresAt: " + saved.ExpiresAt.Format(time.RFC3339),
	} {
		assert.Contains(t, out, want, "output:\n%s", out)
	}
}

func TestAuthLogoutOAuth(t *testing.T) {
	t.Parallel()
	requireTraqStack(t)
	clientID, sess := prepareOAuth(t)

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
	)

	res := driveOAuthLogin(t, clientID, sess, ms, auth.ScopeRead, "approve")
	require.NoError(t, res.sim.err, "the browser simulation failed")
	require.NoError(t, res.err, "browser login failed, output: %s", res.out)
	require.NotEmpty(t, saved.AccessToken)

	// logout でサーバー側のトークンが失効するはず
	e := overrideEnv(t, ms, clientID)
	out, err := runCLI(t, e, "auth", "logout")
	require.NoError(t, err, "auth logout failed")
	assert.Contains(t, out, "logged out")
	assert.Equal(t, http.StatusUnauthorized, apiMeStatus(t, saved.AccessToken))
}

func TestAuthOAuthRefresh(t *testing.T) {
	requireTraqStack(t)

	t.Run("expired token", func(t *testing.T) {
		t.Parallel()
		clientID, sess := prepareOAuth(t)
		ms := auth.NewMockStore(gomock.NewController(t))
		var loggedIn, refreshed auth.Token
		gomock.InOrder(
			ms.EXPECT().Save(gomock.Any()).DoAndReturn(func(tok auth.Token) (string, error) {
				loggedIn = tok
				return "keychain", nil
			}).Times(1),
			ms.EXPECT().Load().DoAndReturn(func() (auth.Token, string, bool, error) {
				// 有効期限を過ぎたトークンを返す。refresh トークンは温存する
				expired := loggedIn
				expired.ExpiresAt = time.Now().Add(-time.Minute)
				return expired, "keychain", true, nil
			}).Times(1),
			ms.EXPECT().Save(gomock.Any()).DoAndReturn(func(tok auth.Token) (string, error) {
				refreshed = tok
				return "keychain", nil
			}).Times(1),
		)

		res := driveOAuthLogin(t, clientID, sess, ms, auth.ScopeRead, "approve")
		require.NoError(t, res.sim.err, "the browser simulation failed")
		require.NoError(t, res.err, "browser login failed, output: %s", res.out)
		require.NotEmpty(t, loggedIn.RefreshToken)

		e := overrideEnv(t, ms, clientID)
		out, err := runCLI(t, e, "user", "me")
		require.NoError(t, err, "users me with an expired token should refresh it")
		assert.NotContains(t, out, `"id"`, "output should be human-readable text, not JSON")
		assert.Contains(t, out, "@traq")

		// 新しいトークンが保存され、refresh トークンがローテーションされているはず
		assert.NotEqual(t, loggedIn.AccessToken, refreshed.AccessToken)
		assert.NotEqual(t, loggedIn.RefreshToken, refreshed.RefreshToken)
		assert.False(t, refreshed.ExpiresAt.IsZero())
		assert.Equal(t, auth.MethodOAuth, refreshed.Method)
	})

	t.Run("401 then refresh", func(t *testing.T) {
		t.Parallel()
		clientID, sess := prepareOAuth(t)
		ms := auth.NewMockStore(gomock.NewController(t))
		var loggedIn, refreshed auth.Token
		gomock.InOrder(
			ms.EXPECT().Save(gomock.Any()).DoAndReturn(func(tok auth.Token) (string, error) {
				loggedIn = tok
				return "keychain", nil
			}).Times(1),
			ms.EXPECT().Load().DoAndReturn(func() (auth.Token, string, bool, error) {
				// アクセストークンだけが無効になった状態を再現する
				broken := loggedIn
				broken.AccessToken = "corrupted-" + loggedIn.AccessToken
				return broken, "keychain", true, nil
			}).Times(1),
			ms.EXPECT().Save(gomock.Any()).DoAndReturn(func(tok auth.Token) (string, error) {
				refreshed = tok
				return "keychain", nil
			}).Times(1),
		)

		res := driveOAuthLogin(t, clientID, sess, ms, auth.ScopeRead, "approve")
		require.NoError(t, res.sim.err, "the browser simulation failed")
		require.NoError(t, res.err, "browser login failed, output: %s", res.out)
		require.NotEmpty(t, loggedIn.RefreshToken)

		e := overrideEnv(t, ms, clientID)
		out, err := runCLI(t, e, "user", "me")
		require.NoError(t, err, "users me should refresh the token after a 401")
		assert.NotContains(t, out, `"id"`, "output should be human-readable text, not JSON")
		assert.Contains(t, out, "@traq")

		assert.NotEqual(t, loggedIn.AccessToken, refreshed.AccessToken)
		assert.NotEqual(t, loggedIn.RefreshToken, refreshed.RefreshToken)
	})
}

// requestNoRedirect はリダイレクトを追従せずに HTTP リクエストを送り、
// レスポンスと body を返す。
func requestNoRedirect(
	t *testing.T,
	method, reqURL string,
	sess *http.Cookie,
	form url.Values,
) (*http.Response, string) {
	t.Helper()
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(t.Context(), method, reqURL, body)
	require.NoError(t, err)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if sess != nil {
		req.AddCookie(sess)
	}
	resp, err := noRedirectClient.Do(req)
	require.NoError(t, err)
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	return resp, string(raw)
}

// apiMeStatus は指定したトークンで GET /users/me を実行し、ステータスコードを返す。
func apiMeStatus(t *testing.T, token string) int {
	t.Helper()
	req, err := http.NewRequestWithContext(
		t.Context(), http.MethodGet, testTraqOrigin+"/api/v3/users/me", nil,
	)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode
}
