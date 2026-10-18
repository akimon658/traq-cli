package authflow

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/akimon658/traq-cli/internal/auth"
	"github.com/akimon658/traq-cli/internal/client"
	"github.com/akimon658/traq-cli/internal/config"
	"github.com/akimon658/traq-cli/internal/exit"
)

// LoginTimeout はブラウザログイン全体のタイムアウト。テストで短縮できるよう変数にしている。
var LoginTimeout = auth.LoginTimeout

// LoginWithToken は Bot 管理画面等で発行したトークンを検証・保存する。
// bot と OAuth のどちらも受け付け、種別は /users/me の応答から自動判別する。
func LoginWithToken(
	ctx context.Context,
	e *config.Env,
	c *client.Client,
	value string,
) (auth.Token, string, error) {
	me, resp, err := c.GetMe(ctx, value)
	if err != nil {
		apiErr := client.WrapError(err, resp)
		if resp != nil && resp.StatusCode == http.StatusUnauthorized {
			return auth.Token{}, "", exit.GeneralError(
				fmt.Errorf("the token was rejected: %s", apiErr.Error()),
			)
		}
		return auth.Token{}, "", exit.GeneralError(apiErr)
	}
	t := auth.Token{
		AccessToken: value,
		Origin:      e.Origin,
		UserID:      me.Id,
		UserName:    me.Name,
		UserBot:     me.Bot,
		IssuedAt:    time.Now(),
	}
	if me.Bot {
		t.Method = auth.MethodBot
	} else {
		t.Method = auth.MethodOAuth
	}
	venue, err := e.LoadStore().Save(t)
	if err != nil {
		return auth.Token{}, "", err
	}
	return t, venue, nil
}

// BrowserSession はブラウザログインの準備結果を保持する。
type BrowserSession struct {
	PKCE         auth.PKCE
	Callback     *auth.CallbackServer
	RedirectURI  string
	AuthorizeURL string
	Scopes       []string
}

// PrepareBrowserLogin は PKCE 生成・コールバックサーバー起動・認可 URL 組み立てを行う。
// 呼び出し元は AuthorizeURL を表示し、Callback.Wait でコードを待つこと。
func PrepareBrowserLogin(e *config.Env, scopeFlag string) (*BrowserSession, error) {
	scopes, err := auth.NormalizeScopes(auth.ParseScopes(scopeFlag))
	if err != nil {
		return nil, exit.UsageError(err)
	}
	pk, err := auth.NewPKCE()
	if err != nil {
		return nil, err
	}
	cb, err := auth.ListenCallback(pk.State)
	if err != nil {
		return nil, err
	}
	redirectURI := cb.RedirectURI()
	return &BrowserSession{
		PKCE:         pk,
		Callback:     cb,
		RedirectURI:  redirectURI,
		AuthorizeURL: auth.AuthorizeURL(e.Origin, e.ClientID, redirectURI, scopes, pk),
		Scopes:       scopes,
	}, nil
}

// FinishLogin は認可コードをトークンに交換し、/users/me で検証して保存する。
// ブラウザフローと手動フロー (CompleteManualLogin) の共通後半処理。
func FinishLogin(
	ctx context.Context,
	e *config.Env,
	c *client.Client,
	code, redirectURI, verifier string,
	fallbackScopes []string,
) (auth.Token, string, error) {
	resp, err := auth.ExchangeCode(ctx, e.Origin, e.ClientID, redirectURI, code, verifier)
	if err != nil {
		return auth.Token{}, "", err
	}
	scopeList := auth.ParseScopes(resp.Scope)
	if len(scopeList) == 0 {
		scopeList = fallbackScopes
	}

	me, meResp, err := c.GetMe(ctx, resp.AccessToken)
	if err != nil {
		return auth.Token{}, "", exit.GeneralError(client.WrapError(err, meResp))
	}
	t := auth.Token{
		Method:       auth.MethodOAuth,
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		Scope:        scopeList,
		Origin:       e.Origin,
		UserID:       me.Id,
		UserName:     me.Name,
		UserBot:      me.Bot,
		IssuedAt:     time.Now(),
	}
	if resp.ExpiresIn > 0 {
		t.ExpiresAt = time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second)
	}
	venue, err := e.LoadStore().Save(t)
	if err != nil {
		return auth.Token{}, "", err
	}
	return t, venue, nil
}
