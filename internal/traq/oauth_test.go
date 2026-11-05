package traq

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/moznion/go-optional"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gotraq "github.com/traPtitech/go-traq"
	"go.uber.org/mock/gomock"

	"github.com/akimon658/traq-cli/internal/auth"
)

// noRedirectClient はリダイレクトを追従しない HTTP クライアント。
var noRedirectClient = &http.Client{
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

type noRedirectRequest struct {
	method string
	url    string
	sess   *http.Cookie
	form   url.Values
}

func requestNoRedirect(t *testing.T, req noRedirectRequest) (*http.Response, string) {
	t.Helper()
	var body io.Reader
	if req.form != nil {
		body = strings.NewReader(req.form.Encode())
	}
	httpReq, err := http.NewRequestWithContext(t.Context(), req.method, req.url, body)
	require.NoError(t, err)
	if req.form != nil {
		httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if req.sess != nil {
		httpReq.AddCookie(req.sess)
	}
	resp, err := noRedirectClient.Do(httpReq)
	require.NoError(t, err)
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	return resp, string(raw)
}

// prepareOAuth は管理者セッションで OAuth2 公開クライアントを登録して client ID を返す。
func prepareOAuth(t *testing.T) (clientID string, sess *http.Cookie) {
	t.Helper()
	sess, err := adminSession(testTraqOrigin)
	require.NoError(t, err, "failed to login as the admin user")
	created, _, err := traqClient(testTraqOrigin, optional.Some(sess)).
		Oauth2API.CreateClient(t.Context()).
		PostClientRequest(gotraq.PostClientRequest{
			Name:        "cli-" + strings.ReplaceAll(uuid.New().String(), "-", "")[:27],
			Description: "traQ CLI integration test client",
			CallbackUrl: "http://127.0.0.1/callback",
			Scopes:      []gotraq.OAuth2Scope{gotraq.OAUTH2SCOPE_READ, gotraq.OAUTH2SCOPE_WRITE},
		}).
		Execute()
	require.NoError(t, err, "failed to create an OAuth client")
	require.NotEmpty(t, created.Id)
	return created.Id, sess
}

type simObservations struct {
	err             error
	consentLocation string
	code            string
	state           string
}

// browserSimFunc は認可 URL に対するブラウザ役の操作を行い、
// traq-cli のコールバックサーバーに code を届ける。
func browserSimFunc(
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
			if redirectURI != "" {
				_, _ = requestNoRedirect(t, noRedirectRequest{
					method: http.MethodGet,
					url:    redirectURI,
				})
			}
		}()
		if err != nil {
			obs.err = fmt.Errorf("failed to parse the authorize URL: %w", err)
			return
		}
		require.NotEmpty(t, state)
		require.NotEmpty(t, redirectURI)

		authzResp, authzBody := requestNoRedirect(t, noRedirectRequest{
			method: http.MethodGet,
			url:    authorizeURL,
			sess:   sess,
		})
		if authzResp.StatusCode != http.StatusFound {
			obs.err = fmt.Errorf(
				"authorize returned %d, expected 302: %s", authzResp.StatusCode, authzBody,
			)
			return
		}
		obs.consentLocation = authzResp.Header.Get("Location")
		assert.True(t, strings.HasPrefix(obs.consentLocation, "/consent"))

		decideResp, decideBody := requestNoRedirect(t, noRedirectRequest{
			method: http.MethodPost,
			url:    aURL.Scheme + "://" + aURL.Host + "/api/v3/oauth2/authorize/decide",
			sess:   sess,
			form:   url.Values{"submit": {submit}},
		})
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
			assert.Equal(t, state, obs.state)
		}

		callbackResp, callbackBody := requestNoRedirect(t, noRedirectRequest{
			method: http.MethodGet,
			url:    decision,
		})
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

type browserLoginResult struct {
	token Token
	venue string
	err   error
	sim   *simObservations
}

type browserLoginInput struct {
	sess      *http.Cookie
	scopeFlag string
	submit    string
}

// driveBrowserLogin は Prepare→ブラウザ役→Wait→Finish の一連を実行する。
func driveBrowserLogin(t *testing.T, svc *Service, in browserLoginInput) browserLoginResult {
	t.Helper()
	obs := &simObservations{}
	var res browserLoginResult
	res.sim = obs

	bs, err := svc.PrepareBrowserLogin(in.scopeFlag)
	if err != nil {
		res.err = err
		return res
	}
	defer bs.Callback.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		browserSimFunc(t, in.sess, in.submit, obs)(bs.AuthorizeURL)
	}()

	ctx, cancel := context.WithTimeout(t.Context(), LoginTimeout)
	defer cancel()
	code, err := bs.Callback.Wait(ctx)
	<-done
	if err != nil {
		res.err = err
		return res
	}
	tok, venue, err := svc.FinishLogin(t.Context(), FinishRequest{
		Code:           code,
		RedirectURI:    bs.RedirectURI,
		Verifier:       bs.PKCE.Verifier,
		FallbackScopes: bs.Scopes,
	})
	res.token, res.venue, res.err = tok, venue, err
	return res
}

func TestService_BrowserLogin(t *testing.T) {
	t.Run("read scope", func(t *testing.T) {
		t.Parallel()
		requireTraqStack(t)
		clientID, sess := prepareOAuth(t)

		ms := NewMockStore(gomock.NewController(t))
		var saved Token
		ms.EXPECT().Save(gomock.Any()).DoAndReturn(func(tok Token) (string, error) {
			saved = tok
			return "keychain", nil
		}).Times(1)
		svc := NewService(testTraqOrigin, clientID, ms, nil)

		res := driveBrowserLogin(
			t,
			svc,
			browserLoginInput{sess: sess, scopeFlag: ScopeRead, submit: "approve"},
		)
		require.NoError(t, res.sim.err)
		require.NoError(t, res.err)
		assert.Equal(t, MethodOAuth, saved.Method)
		assert.Equal(t, []string{ScopeRead}, saved.Scope)
		assert.Equal(t, "traq", saved.UserName)
		assert.False(t, saved.UserBot)
		assert.NotEmpty(t, saved.RefreshToken)
		assert.False(t, saved.ExpiresAt.IsZero())

		// read スコープでは書き込みが事前検査で弾かれる
		c := svc.WithToken(&ResolvedToken{Token: &saved, Source: "keychain"})
		_, err := c.PostMessage(t.Context(), testGeneralChannelID(t), "should be rejected")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not have the 'write' scope")
	})

	t.Run("write scope", func(t *testing.T) {
		t.Parallel()
		requireTraqStack(t)
		clientID, sess := prepareOAuth(t)

		ms := NewMockStore(gomock.NewController(t))
		var saved Token
		ms.EXPECT().Save(gomock.Any()).DoAndReturn(func(tok Token) (string, error) {
			saved = tok
			return "keychain", nil
		}).Times(1)
		svc := NewService(testTraqOrigin, clientID, ms, nil)

		res := driveBrowserLogin(
			t,
			svc,
			browserLoginInput{sess: sess, scopeFlag: "read,write", submit: "approve"},
		)
		require.NoError(t, res.sim.err)
		require.NoError(t, res.err)
		assert.Equal(t, []string{ScopeRead, ScopeWrite}, saved.Scope)
	})

	t.Run("denied", func(t *testing.T) {
		t.Parallel()
		requireTraqStack(t)
		clientID, sess := prepareOAuth(t)

		ms := NewMockStore(gomock.NewController(t))
		svc := NewService(testTraqOrigin, clientID, ms, nil)

		res := driveBrowserLogin(
			t,
			svc,
			browserLoginInput{sess: sess, scopeFlag: ScopeRead, submit: "deny"},
		)
		require.NoError(t, res.sim.err)
		require.Error(t, res.err)
		assert.Contains(t, res.err.Error(), "authorization was denied (access_denied)")
	})
}

func TestService_Refresh(t *testing.T) {
	t.Run("expired token", func(t *testing.T) {
		t.Parallel()
		requireTraqStack(t)
		clientID, sess := prepareOAuth(t)

		ms := NewMockStore(gomock.NewController(t))
		var loggedIn, refreshed Token
		gomock.InOrder(
			ms.EXPECT().Save(gomock.Any()).DoAndReturn(func(tok Token) (string, error) {
				loggedIn = tok
				return "keychain", nil
			}).Times(1),
			ms.EXPECT().Load().DoAndReturn(func() (Token, string, bool, error) {
				expired := loggedIn
				expired.ExpiresAt = time.Now().Add(-time.Minute)
				return expired, "keychain", true, nil
			}).Times(1),
			ms.EXPECT().Save(gomock.Any()).DoAndReturn(func(tok Token) (string, error) {
				refreshed = tok
				return "keychain", nil
			}).Times(1),
		)
		svc := NewService(testTraqOrigin, clientID, ms, nil)

		res := driveBrowserLogin(
			t,
			svc,
			browserLoginInput{sess: sess, scopeFlag: ScopeRead, submit: "approve"},
		)
		require.NoError(t, res.sim.err)
		require.NoError(t, res.err)
		require.NotEmpty(t, loggedIn.RefreshToken)

		// 期限切れトークンの解決で refresh が走る
		rt, err := svc.ResolveToken(t.Context(), "", "")
		require.NoError(t, err)
		assert.NotEqual(t, loggedIn.AccessToken, rt.Token.AccessToken)
		assert.NotEqual(t, loggedIn.RefreshToken, refreshed.RefreshToken)
	})

	t.Run("401 then refresh", func(t *testing.T) {
		t.Parallel()
		requireTraqStack(t)
		clientID, sess := prepareOAuth(t)

		ms := NewMockStore(gomock.NewController(t))
		var loggedIn Token
		ms.EXPECT().Save(gomock.Any()).DoAndReturn(func(tok Token) (string, error) {
			loggedIn = tok
			return "keychain", nil
		}).Times(1)
		svc := NewService(testTraqOrigin, clientID, ms, nil)

		res := driveBrowserLogin(
			t,
			svc,
			browserLoginInput{sess: sess, scopeFlag: ScopeRead, submit: "approve"},
		)
		require.NoError(t, res.sim.err)
		require.NoError(t, res.err)

		// アクセストークンだけを壊すと GetMe の 401 リトライで refresh される
		broken := loggedIn
		broken.AccessToken = "corrupted-" + loggedIn.AccessToken
		c := svc.WithToken(&ResolvedToken{Token: &broken, Source: "keychain"})
		// Save が2回目として呼ばれる
		ms.EXPECT().Save(gomock.Any()).Return("keychain", nil).Times(1)
		me, err := c.GetMe(t.Context())
		require.NoError(t, err)
		assert.Equal(t, "traq", me.Name)
	})
}

func TestService_ManualLogin(t *testing.T) {
	manualDecision := func(t *testing.T, sess *http.Cookie, submit, authorizeURL string) string {
		t.Helper()
		aURL, err := url.Parse(authorizeURL)
		require.NoError(t, err)
		require.Equal(t, auth.ManualRedirectURI, aURL.Query().Get("redirect_uri"))

		authzResp, _ := requestNoRedirect(t, noRedirectRequest{
			method: http.MethodGet,
			url:    authorizeURL,
			sess:   sess,
		})
		require.Equal(t, http.StatusFound, authzResp.StatusCode)
		decideResp, _ := requestNoRedirect(t, noRedirectRequest{
			method: http.MethodPost,
			url:    aURL.Scheme + "://" + aURL.Host + "/api/v3/oauth2/authorize/decide",
			sess:   sess,
			form:   url.Values{"submit": {submit}},
		})
		require.Equal(t, http.StatusFound, decideResp.StatusCode)
		decision := decideResp.Header.Get("Location")
		require.NotEmpty(t, decision)
		if strings.HasPrefix(decision, "http") {
			return decision
		}
		return aURL.Scheme + "://" + aURL.Host + decision
	}

	t.Run("flow", func(t *testing.T) {
		t.Parallel()
		requireTraqStack(t)
		clientID, sess := prepareOAuth(t)

		ctrl := gomock.NewController(t)
		ms := NewMockStore(ctrl)
		mp := auth.NewMockSessionStore(ctrl)
		var pending auth.ManualSession
		var saved Token
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
			ms.EXPECT().Save(gomock.Any()).DoAndReturn(func(tok Token) (string, error) {
				saved = tok
				return "keychain", nil
			}).Times(1),
			mp.EXPECT().Delete().Return(true, nil).Times(1),
		)

		svc := NewService(testTraqOrigin, clientID, ms, mp)
		authorizeURL, err := svc.StartManualLogin(ScopeRead)
		require.NoError(t, err)

		decision := manualDecision(t, sess, "approve", authorizeURL)
		got, venue, err := svc.CompleteManualLogin(t.Context(), decision)
		require.NoError(t, err)
		assert.Equal(t, "keychain", venue)
		assert.Equal(t, MethodOAuth, got.Method)
		assert.Equal(t, saved, got)
	})

	t.Run("denied", func(t *testing.T) {
		t.Parallel()
		requireTraqStack(t)
		clientID, sess := prepareOAuth(t)

		ctrl := gomock.NewController(t)
		ms := NewMockStore(ctrl)
		mp := auth.NewMockSessionStore(ctrl)
		gomock.InOrder(
			mp.EXPECT().Save(gomock.Any()).Return("/tmp/login-session.json", nil).Times(1),
		)

		svc := NewService(testTraqOrigin, clientID, ms, mp)
		authorizeURL, err := svc.StartManualLogin(ScopeRead)
		require.NoError(t, err)

		decision := manualDecision(t, sess, "deny", authorizeURL)
		// Load 前に deny が parse されるためセッションは読まれない
		_ = mp
		_, _, err = svc.CompleteManualLogin(t.Context(), decision)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "authorization was denied (access_denied)")
	})
}

func TestService_LogoutOAuth(t *testing.T) {
	t.Run("revokes server token", func(t *testing.T) {
		t.Parallel()
		requireTraqStack(t)
		clientID, sess := prepareOAuth(t)

		ms := NewMockStore(gomock.NewController(t))
		var saved Token
		gomock.InOrder(
			ms.EXPECT().Save(gomock.Any()).DoAndReturn(func(tok Token) (string, error) {
				saved = tok
				return "keychain", nil
			}).Times(1),
			ms.EXPECT().Load().DoAndReturn(func() (Token, string, bool, error) {
				return saved, "keychain", true, nil
			}).Times(1),
			ms.EXPECT().Delete().Return(true, nil).Times(1),
		)
		svc := NewService(testTraqOrigin, clientID, ms, nil)

		res := driveBrowserLogin(
			t,
			svc,
			browserLoginInput{sess: sess, scopeFlag: ScopeRead, submit: "approve"},
		)
		require.NoError(t, res.sim.err)
		require.NoError(t, res.err)

		_, err := svc.Logout(t.Context())
		require.NoError(t, err)

		_, resp, err := svc.VerifyToken(t.Context(), saved.AccessToken)
		require.Error(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
}
